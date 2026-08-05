// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Hard CPU, memory and wall-clock ceilings for a single simulation run.
//!
//! The Soroban budget meters every Wasm instruction, so a contract that loops
//! forever eventually traps with a budget error. That is not enough for a fuzz
//! harness. A run can also spin inside *native* simulator code — a decoder that
//! never converges on malformed input, a retry loop that never gives up — where
//! the budget is never charged and nothing forces the run to end. libFuzzer will
//! wait for it indefinitely.
//!
//! This module supplies the ceilings that close that gap, cheapest first:
//!
//! 1. [`MeteringLimits::budget`] builds a Soroban [`Budget`] with explicit CPU
//!    instruction and memory byte caps, so metered work traps early.
//! 2. [`Deadline`] is a cooperative wall-clock check for loops the caller drives
//!    itself.
//! 3. [`Watchdog`] runs on its own thread and aborts the process when a run
//!    outlives its wall-clock budget. This is what turns a hang into a reported
//!    failure with a saved reproducer.
//! 4. [`TrackingAllocator`] caps the live heap, and [`install_cpu_rlimit`] gives
//!    the kernel the last word on CPU time.
//!
//! Memory is deliberately *not* capped with `RLIMIT_AS`; see
//! [`install_cpu_rlimit`] for why.

use soroban_env_host::{budget::Budget, HostError};
use std::alloc::{GlobalAlloc, Layout};
use std::sync::atomic::{AtomicU64, AtomicUsize, Ordering};
use std::sync::{Arc, OnceLock};
use std::thread;
use std::time::{Duration, Instant};

/// How often the watchdog thread compares the clock against the deadline of the
/// run in flight.
pub const WATCHDOG_POLL_INTERVAL: Duration = Duration::from_millis(25);

/// Deadline value meaning "no run is in flight".
const DISARMED: u64 = 0;

/// The ceilings applied to a single simulation run.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct MeteringLimits {
    /// Soroban CPU instruction cap. Metered Wasm traps once it is reached, which
    /// is what stops a contract-level infinite loop.
    pub cpu_insns: u64,
    /// Soroban memory cap, in bytes, for host-side allocations.
    pub mem_bytes: u64,
    /// Wall-clock budget for one run, enforced cooperatively by [`Deadline`] and
    /// forcibly by [`Watchdog`].
    pub wall_clock: Duration,
    /// CPU seconds the whole process may consume, handed to `RLIMIT_CPU`.
    pub cpu_seconds: u64,
    /// Largest input worth running. Anything bigger is rejected before work
    /// starts rather than being decoded slowly.
    pub max_input_bytes: usize,
}

impl MeteringLimits {
    /// Live-heap ceiling for the fuzz harness, in bytes.
    ///
    /// An associated constant because `#[global_allocator]` statics have to be
    /// initialised in a const context.
    pub const FUZZ_HEAP_BYTES: usize = 256 * 1024 * 1024;

    /// Ceilings for the fuzz harness: a small fraction of the network's, so an
    /// input that tries to run forever is cut off in milliseconds.
    #[must_use]
    pub const fn fuzzing() -> Self {
        Self {
            cpu_insns: 10_000_000,
            mem_bytes: 8 * 1024 * 1024,
            wall_clock: Duration::from_secs(2),
            cpu_seconds: 10,
            max_input_bytes: 64 * 1024,
        }
    }

    /// Ceilings that mirror the network's per-transaction resource caps.
    #[must_use]
    pub const fn network() -> Self {
        Self {
            cpu_insns: crate::gas_optimizer::CPU_LIMIT,
            mem_bytes: crate::gas_optimizer::MEMORY_LIMIT,
            wall_clock: Duration::from_secs(30),
            cpu_seconds: 120,
            max_input_bytes: 16 * 1024 * 1024,
        }
    }

    /// The CPU/memory pair in the shape `runner::SimHost::new` expects.
    #[must_use]
    pub const fn budget_limits(&self) -> (u64, u64) {
        (self.cpu_insns, self.mem_bytes)
    }

    /// Builds a Soroban budget that traps once either cap is reached.
    pub fn budget(&self) -> Result<Budget, HostError> {
        let budget = Budget::default();
        budget.reset_limits(self.cpu_insns, self.mem_bytes)?;
        Ok(budget)
    }

    /// Starts a cooperative wall-clock deadline for one run.
    #[must_use]
    pub fn deadline(&self) -> Deadline {
        Deadline::new(self.wall_clock)
    }
}

impl Default for MeteringLimits {
    fn default() -> Self {
        Self::network()
    }
}

/// Why a run was cut short.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum MeteringError {
    /// The run outlived its wall-clock budget.
    #[error("run exceeded its {limit_ms} ms wall-clock budget after {elapsed_ms} ms")]
    Timeout { elapsed_ms: u64, limit_ms: u64 },

    /// The CPU instruction cap was reached.
    #[error("run exhausted its CPU budget ({consumed} of {limit} instructions)")]
    CpuExhausted { consumed: u64, limit: u64 },

    /// The memory cap was reached.
    #[error("run exhausted its memory budget ({consumed} of {limit} bytes)")]
    MemoryExhausted { consumed: u64, limit: u64 },

    /// A process resource limit could not be applied.
    #[error("could not apply {resource}: os error {errno}")]
    Rlimit { resource: &'static str, errno: i32 },
}

#[cfg(unix)]
impl MeteringError {
    fn last_os_rlimit(resource: &'static str) -> Self {
        let failure = std::io::Error::last_os_error();
        let errno = failure.raw_os_error().unwrap_or(0);

        Self::Rlimit { resource, errno }
    }
}

/// A cooperative wall-clock budget for one run.
///
/// Checking it costs a single monotonic clock read, so callers can afford to call
/// [`Deadline::check`] between units of work — per operation, per decoded entry —
/// and bail out with a real error instead of being killed.
#[derive(Debug, Clone, Copy)]
pub struct Deadline {
    started: Instant,
    limit: Duration,
}

impl Deadline {
    /// Starts a deadline `limit` from now.
    #[must_use]
    pub fn new(limit: Duration) -> Self {
        Self {
            started: Instant::now(),
            limit,
        }
    }

    /// Wall-clock time consumed so far.
    #[must_use]
    pub fn elapsed(&self) -> Duration {
        self.started.elapsed()
    }

    /// The budget this deadline was created with.
    #[must_use]
    pub const fn limit(&self) -> Duration {
        self.limit
    }

    /// Whether the budget is used up.
    #[must_use]
    pub fn expired(&self) -> bool {
        self.elapsed() >= self.limit
    }

    /// Returns [`MeteringError::Timeout`] once the budget is used up.
    pub fn check(&self) -> Result<(), MeteringError> {
        let elapsed = self.elapsed();
        if elapsed < self.limit {
            return Ok(());
        }

        Err(MeteringError::Timeout {
            elapsed_ms: whole_millis(elapsed),
            limit_ms: whole_millis(self.limit),
        })
    }
}

/// Whole milliseconds in `duration`, saturating rather than wrapping.
fn whole_millis(duration: Duration) -> u64 {
    let millis = duration.as_millis();

    u64::try_from(millis).unwrap_or(u64::MAX)
}

/// Checks a live Soroban budget against `limits`.
///
/// The budget traps on its own once a cap is reached; this is for callers that
/// want to stop *before* the trap, or to report which cap was hit. A budget read
/// that fails is treated as "nothing consumed", because a broken budget handle is
/// not evidence that a limit was exceeded.
pub fn check_budget(budget: &Budget, limits: &MeteringLimits) -> Result<(), MeteringError> {
    let cpu = budget.get_cpu_insns_consumed().unwrap_or(0);
    if cpu >= limits.cpu_insns {
        return Err(MeteringError::CpuExhausted {
            consumed: cpu,
            limit: limits.cpu_insns,
        });
    }

    let mem = budget.get_mem_bytes_consumed().unwrap_or(0);
    if mem >= limits.mem_bytes {
        return Err(MeteringError::MemoryExhausted {
            consumed: mem,
            limit: limits.mem_bytes,
        });
    }

    Ok(())
}

/// State shared between the watchdog thread and the run it guards.
struct WatchdogState {
    /// Fixed reference point. Deadlines are nanosecond offsets from here so they
    /// fit in an atomic.
    origin: Instant,
    /// Offset by which the run in flight must finish, or `DISARMED`.
    deadline_nanos: AtomicU64,
    /// Offset at which the run in flight started, used only for reporting.
    armed_at_nanos: AtomicU64,
}

impl WatchdogState {
    fn now_nanos(&self) -> u64 {
        let nanos = self.origin.elapsed().as_nanos();

        u64::try_from(nanos).unwrap_or(u64::MAX)
    }

    fn arm(&self, budget: Duration) {
        let started = self.now_nanos();
        let budget_nanos = u64::try_from(budget.as_nanos()).unwrap_or(u64::MAX);
        // A zero-length budget still has to land on a deadline the thread acts
        // on, and DISARMED is zero.
        let deadline = started.saturating_add(budget_nanos).max(DISARMED + 1);

        self.armed_at_nanos.store(started, Ordering::Relaxed);
        self.deadline_nanos.store(deadline, Ordering::Release);
    }

    fn disarm(&self) {
        self.deadline_nanos.store(DISARMED, Ordering::Release);
    }
}

/// A thread that ends the process when a run outlives its wall-clock budget.
///
/// This is the ceiling that actually breaks a hang. A run that never returns to
/// the harness cannot check its own [`Deadline`], so something outside it has to
/// act; here that is an abort, which libFuzzer records as a crash and saves the
/// offending input for. One watchdog guards one run at a time, matching the shape
/// libFuzzer already has: it drives the target on a single thread.
///
/// The thread lives for the rest of the process. There is nothing to shut down,
/// which is why a single watchdog is shared by every run.
pub struct Watchdog {
    state: Arc<WatchdogState>,
}

impl Watchdog {
    /// Spawns a watchdog that aborts the process when a run overstays.
    #[must_use]
    pub fn spawn(poll_interval: Duration) -> Self {
        Self::spawn_with(poll_interval, |elapsed, limit| {
            eprintln!("[metering] wall-clock hang: {elapsed:?} > {limit:?}");
            eprintln!("[metering] aborting to force a timeout");
            std::process::abort();
        })
    }

    /// Spawns a watchdog that calls `on_timeout(elapsed, limit)` instead of
    /// aborting, so tests can observe a trip and survive it.
    #[must_use]
    pub fn spawn_with<F>(poll_interval: Duration, on_timeout: F) -> Self
    where
        F: Fn(Duration, Duration) + Send + 'static,
    {
        let state = Arc::new(WatchdogState {
            origin: Instant::now(),
            deadline_nanos: AtomicU64::new(DISARMED),
            armed_at_nanos: AtomicU64::new(0),
        });

        let watched = Arc::clone(&state);
        thread::Builder::new()
            .name("erst-metering-watchdog".to_owned())
            .spawn(move || watch(&watched, poll_interval, &on_timeout))
            .expect("watchdog thread must start");

        Self { state }
    }

    /// Puts the caller's run under the watchdog for `budget`.
    pub fn arm(&self, budget: Duration) -> RunGuard<'_> {
        self.state.arm(budget);

        RunGuard {
            state: self.state.as_ref(),
            deadline: Deadline::new(budget),
        }
    }
}

/// Polls the armed deadline forever, reporting every run that overstays it.
fn watch<F>(state: &WatchdogState, poll_interval: Duration, on_timeout: &F)
where
    F: Fn(Duration, Duration),
{
    // Deadlines are monotonic offsets, so remembering the last one reported is
    // enough to report each overstaying run exactly once. The alternative —
    // clearing the deadline from here — would unwatch any run that armed itself
    // between the load and the clear.
    let mut reported = DISARMED;

    loop {
        thread::sleep(poll_interval);

        let deadline = state.deadline_nanos.load(Ordering::Acquire);
        if deadline == DISARMED || deadline == reported {
            continue;
        }

        let now = state.now_nanos();
        if now < deadline {
            continue;
        }

        reported = deadline;
        let armed_at = state.armed_at_nanos.load(Ordering::Relaxed);
        on_timeout(
            Duration::from_nanos(now.saturating_sub(armed_at)),
            Duration::from_nanos(deadline.saturating_sub(armed_at)),
        );
    }
}

/// Proof that a run is being watched, plus the cooperative deadline for it.
///
/// Dropping the guard disarms the watchdog, so a run that finishes in time is
/// never reported.
#[must_use = "dropping the guard immediately disarms the watchdog"]
pub struct RunGuard<'a> {
    state: &'a WatchdogState,
    deadline: Deadline,
}

impl RunGuard<'_> {
    /// The cooperative deadline for this run.
    pub const fn deadline(&self) -> &Deadline {
        &self.deadline
    }

    /// Returns [`MeteringError::Timeout`] once this run is out of time.
    pub fn check(&self) -> Result<(), MeteringError> {
        self.deadline.check()
    }
}

impl Drop for RunGuard<'_> {
    fn drop(&mut self) {
        self.state.disarm();
    }
}

/// Installs the process-wide ceilings the fuzz harness relies on and returns the
/// shared watchdog.
///
/// Idempotent: the rlimit is applied and the watchdog thread spawned on the first
/// call, and every later call hands back the same watchdog. Fuzz targets are
/// re-entered thousands of times a second, so this has to stay cheap afterwards.
pub fn install_fuzz_limits() -> &'static Watchdog {
    static WATCHDOG: OnceLock<Watchdog> = OnceLock::new();

    WATCHDOG.get_or_init(|| {
        let limits = MeteringLimits::fuzzing();
        if let Err(err) = install_cpu_rlimit(&limits) {
            // Not fatal: the watchdog still bounds wall-clock time without it.
            eprintln!("[metering] CPU rlimit not applied: {err}");
        }

        Watchdog::spawn(WATCHDOG_POLL_INTERVAL)
    })
}

/// Caps the CPU time of the whole process with `RLIMIT_CPU`.
///
/// The kernel signals the process once the soft limit is passed, covering the
/// case the watchdog cannot: a run that starves every other thread. Existing
/// limits are only ever lowered, so this is safe to call in an environment that
/// already applied a tighter cap.
///
/// `RLIMIT_AS` is deliberately left alone. Sanitizer-instrumented fuzz builds
/// reserve terabytes of virtual address space for shadow memory, so capping
/// address space kills the process before it reaches `main`. The live heap is
/// bounded by [`TrackingAllocator`] instead, which counts real allocations.
#[cfg(unix)]
pub fn install_cpu_rlimit(limits: &MeteringLimits) -> Result<(), MeteringError> {
    let mut current = libc::rlimit {
        rlim_cur: 0,
        rlim_max: 0,
    };

    // SAFETY: `current` is a live, correctly typed `rlimit` owned by this frame.
    if unsafe { libc::getrlimit(libc::RLIMIT_CPU, &mut current) } != 0 {
        return Err(MeteringError::last_os_rlimit("RLIMIT_CPU"));
    }

    // `rlim_t` is u64 on every platform this project builds for, so the ceiling
    // needs no conversion. The min() calls keep an existing tighter limit, since
    // an unprivileged process may only ever lower its own.
    let soft = limits.cpu_seconds.min(current.rlim_cur);
    let next = libc::rlimit {
        rlim_cur: soft.min(current.rlim_max),
        rlim_max: current.rlim_max,
    };

    // SAFETY: `next` is a live, correctly typed `rlimit` owned by this frame.
    if unsafe { libc::setrlimit(libc::RLIMIT_CPU, &next) } != 0 {
        return Err(MeteringError::last_os_rlimit("RLIMIT_CPU"));
    }

    Ok(())
}

/// No-op where POSIX resource limits are unavailable. [`Watchdog`] still bounds
/// wall-clock time on those platforms.
#[cfg(not(unix))]
pub fn install_cpu_rlimit(_limits: &MeteringLimits) -> Result<(), MeteringError> {
    Ok(())
}

/// A [`GlobalAlloc`] wrapper that refuses to let the live heap pass a ceiling.
///
/// Fuzz inputs are very good at finding a length field that turns into a
/// multi-gigabyte allocation. Left alone the process starts swapping and the run
/// is indistinguishable from a hang. Capped here, the allocation fails, Rust's
/// out-of-memory handler aborts, and libFuzzer saves the input.
///
/// The ceiling is on *live* bytes rather than total bytes allocated, so a run
/// that churns through many short-lived buffers is not punished for it.
pub struct TrackingAllocator<A> {
    inner: A,
    limit: usize,
    live: AtomicUsize,
    peak: AtomicUsize,
}

impl<A> TrackingAllocator<A> {
    /// Wraps `inner`, refusing allocations that would push the live heap past
    /// `limit` bytes.
    ///
    /// `const` so it can initialise a `#[global_allocator]` static.
    pub const fn new(inner: A, limit: usize) -> Self {
        Self {
            inner,
            limit,
            live: AtomicUsize::new(0),
            peak: AtomicUsize::new(0),
        }
    }

    /// Bytes currently allocated through this allocator.
    pub fn live_bytes(&self) -> usize {
        self.live.load(Ordering::Relaxed)
    }

    /// High-water mark of the live byte count.
    pub fn peak_bytes(&self) -> usize {
        self.peak.load(Ordering::Relaxed)
    }

    /// The ceiling this allocator enforces.
    pub const fn limit_bytes(&self) -> usize {
        self.limit
    }

    /// Accounts for `bytes` about to be allocated, or refuses them.
    fn reserve(&self, bytes: usize) -> bool {
        let order = Ordering::Relaxed;
        let claimed = self.live.fetch_update(order, order, |live| {
            let next = live.saturating_add(bytes);
            if next > self.limit {
                None
            } else {
                Some(next)
            }
        });

        match claimed {
            Ok(previous) => {
                let live = previous.saturating_add(bytes);
                self.peak.fetch_max(live, order);
                true
            }
            Err(_) => false,
        }
    }

    /// Returns `bytes` to the ceiling.
    fn release(&self, bytes: usize) {
        self.live.fetch_sub(bytes, Ordering::Relaxed);
    }
}

// SAFETY: every method forwards to `inner`, which is itself a correct
// `GlobalAlloc`. The accounting around those calls never touches the memory and
// never hands back a pointer `inner` did not produce.
unsafe impl<A: GlobalAlloc> GlobalAlloc for TrackingAllocator<A> {
    unsafe fn alloc(&self, layout: Layout) -> *mut u8 {
        if !self.reserve(layout.size()) {
            return std::ptr::null_mut();
        }

        // SAFETY: `layout` is forwarded unchanged from our caller.
        let ptr = unsafe { self.inner.alloc(layout) };
        if ptr.is_null() {
            self.release(layout.size());
        }

        ptr
    }

    unsafe fn alloc_zeroed(&self, layout: Layout) -> *mut u8 {
        if !self.reserve(layout.size()) {
            return std::ptr::null_mut();
        }

        // SAFETY: `layout` is forwarded unchanged from our caller.
        let ptr = unsafe { self.inner.alloc_zeroed(layout) };
        if ptr.is_null() {
            self.release(layout.size());
        }

        ptr
    }

    unsafe fn dealloc(&self, ptr: *mut u8, layout: Layout) {
        // SAFETY: `ptr` and `layout` are forwarded unchanged from our caller.
        unsafe { self.inner.dealloc(ptr, layout) };
        self.release(layout.size());
    }

    unsafe fn realloc(&self, ptr: *mut u8, layout: Layout, new_size: usize) -> *mut u8 {
        let growth = new_size.saturating_sub(layout.size());
        if growth > 0 && !self.reserve(growth) {
            return std::ptr::null_mut();
        }

        // SAFETY: all three arguments are forwarded unchanged from our caller.
        let resized = unsafe { self.inner.realloc(ptr, layout, new_size) };
        if resized.is_null() {
            self.release(growth);
        } else {
            self.release(layout.size().saturating_sub(new_size));
        }

        resized
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::alloc::System;
    use std::sync::atomic::AtomicBool;

    #[test]
    fn fuzzing_ceilings_are_tighter_than_network_ceilings() {
        let fuzzing = MeteringLimits::fuzzing();
        let network = MeteringLimits::network();

        assert!(fuzzing.cpu_insns < network.cpu_insns);
        assert!(fuzzing.mem_bytes < network.mem_bytes);
        assert!(fuzzing.wall_clock < network.wall_clock);
        assert!(fuzzing.cpu_seconds < network.cpu_seconds);
        assert!(fuzzing.max_input_bytes < network.max_input_bytes);
    }

    #[test]
    fn a_fresh_budget_has_not_exhausted_its_ceilings() {
        let limits = MeteringLimits::fuzzing();
        let budget = limits.budget().expect("limits apply");

        assert_eq!(check_budget(&budget, &limits), Ok(()));
    }

    #[test]
    fn a_zero_cpu_ceiling_reports_an_exhausted_budget() {
        let limits = MeteringLimits {
            cpu_insns: 0,
            ..MeteringLimits::fuzzing()
        };
        let budget = limits.budget().expect("limits apply");
        let outcome = check_budget(&budget, &limits);

        assert!(matches!(outcome, Err(MeteringError::CpuExhausted { .. })));
    }

    #[test]
    fn an_expired_deadline_reports_a_timeout() {
        let deadline = Deadline::new(Duration::ZERO);
        let outcome = deadline.check();

        assert!(deadline.expired());
        assert!(matches!(outcome, Err(MeteringError::Timeout { .. })));
    }

    #[test]
    fn a_fresh_deadline_has_time_left() {
        let deadline = Deadline::new(Duration::from_secs(60));

        assert!(!deadline.expired());
        assert_eq!(deadline.check(), Ok(()));
        assert_eq!(deadline.limit(), Duration::from_secs(60));
    }

    #[test]
    fn the_watchdog_reports_a_run_that_overstays() {
        let tripped = Arc::new(AtomicBool::new(false));
        let observed = Arc::clone(&tripped);
        let watchdog = Watchdog::spawn_with(Duration::from_millis(5), move |_, _| {
            observed.store(true, Ordering::Relaxed);
        });

        let _guard = watchdog.arm(Duration::from_millis(10));
        thread::sleep(Duration::from_millis(400));

        assert!(tripped.load(Ordering::Relaxed), "hang not reported");
    }

    #[test]
    fn dropping_the_guard_disarms_the_watchdog() {
        let tripped = Arc::new(AtomicBool::new(false));
        let observed = Arc::clone(&tripped);
        let watchdog = Watchdog::spawn_with(Duration::from_millis(5), move |_, _| {
            observed.store(true, Ordering::Relaxed);
        });

        drop(watchdog.arm(Duration::from_millis(10)));
        thread::sleep(Duration::from_millis(400));

        assert!(!tripped.load(Ordering::Relaxed), "finished run reported");
    }

    #[test]
    fn the_allocator_refuses_to_grow_past_its_ceiling() {
        let allocator = TrackingAllocator::new(System, 4096);
        let layout = Layout::from_size_align(1024, 8).expect("valid layout");
        assert_eq!(allocator.limit_bytes(), 4096);

        // SAFETY: the layout has a non-zero size, and the block is freed below.
        let ptr = unsafe { allocator.alloc(layout) };
        assert!(!ptr.is_null());
        assert_eq!(allocator.live_bytes(), 1024);

        let oversized = Layout::from_size_align(8192, 8).expect("valid layout");
        // SAFETY: a refused allocation returns null and owns nothing.
        let refused = unsafe { allocator.alloc(oversized) };
        assert!(refused.is_null());
        assert_eq!(allocator.live_bytes(), 1024, "refusal leaked bytes");

        // SAFETY: `ptr` came from this allocator with this layout.
        unsafe { allocator.dealloc(ptr, layout) };
        assert_eq!(allocator.live_bytes(), 0);
        assert_eq!(allocator.peak_bytes(), 1024);
    }
}
