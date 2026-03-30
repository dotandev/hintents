// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

#[derive(Debug, Clone, PartialEq, Eq)]
struct SnapshotFrame {
    snapshot_id: String,
    label: &'static str,
    step: usize,
}

#[derive(Debug, Default)]
struct SnapshotRegistry {
    frames: Vec<SnapshotFrame>,
}

impl SnapshotRegistry {
    fn record(&mut self, step: usize) -> SnapshotFrame {
        let frame = SnapshotFrame {
            snapshot_id: format!("snap-{step:04}"),
            label: "Final",
            step,
        };
        self.frames.push(frame.clone());
        frame
    }
}

fn execute_with_forced_trap(
    total_steps: usize,
    trap_at: usize,
) -> (SnapshotRegistry, SnapshotFrame, String) {
    assert!(trap_at > 0, "trap_at must be greater than 0");
    assert!(trap_at < total_steps, "trap_at must be inside total steps");

    let mut registry = SnapshotRegistry::default();
    let mut last_good = registry.record(0);

    for step in 1..total_steps {
        if step == trap_at {
            let err = format!(
                "contract trap at step {step}; use snapshot_id={} for last-known-good state",
                last_good.snapshot_id
            );
            return (registry, last_good, err);
        }
        last_good = registry.record(step);
    }

    unreachable!("test expects a trap before completion")
}

#[test]
fn final_snapshot_is_preserved_before_trap() {
    let (registry, final_snapshot, err) = execute_with_forced_trap(8, 4);

    assert_eq!(final_snapshot.step, 3);
    assert_eq!(final_snapshot.label, "Final");
    assert!(err.contains(&final_snapshot.snapshot_id));

    let persisted = registry.frames.iter().any(|frame| {
        frame.snapshot_id == final_snapshot.snapshot_id && frame.step == final_snapshot.step
    });
    assert!(persisted, "registry dropped the final snapshot");
}

#[test]
fn error_message_links_to_snapshot_id() {
    let (_registry, final_snapshot, err) = execute_with_forced_trap(6, 2);
    let expected = format!("snapshot_id={}", final_snapshot.snapshot_id);
    assert!(
        err.contains(&expected),
        "error message must include linked snapshot id; got: {err}"
    );
}

#[test]
fn panic_fixture_contract_contains_trap_marker() {
    let src = include_str!("../../tests/fixtures/contracts/panic.rust");
    assert!(src.contains("panic!"), "fixture must intentionally panic");
    assert!(
        src.contains("panic_with_snapshot"),
        "fixture should expose panic entrypoint"
    );
}
