// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Lightweight String Interner for Path Deduplication
//!
//! Deep call stacks and extensive source map entries often duplicate the same
//! source file paths hundreds or thousands of times. By interning path strings
//! into shared `Arc<str>` references, memory usage is drastically reduced,
//! and clones become cheap atomic reference bumps instead of deep heap allocations.

#![allow(dead_code)]

use once_cell::sync::Lazy;
use std::collections::HashSet;
use std::sync::{Arc, RwLock};

/// Thread-safe string interner for deduplicating file paths.
#[derive(Debug, Default)]
pub struct PathInterner {
    pool: RwLock<HashSet<Arc<str>>>,
}

impl PathInterner {
    /// Creates a new, empty `PathInterner`.
    #[must_use]
    pub fn new() -> Self {
        Self {
            pool: RwLock::new(HashSet::new()),
        }
    }

    /// Interns a string slice, returning an `Arc<str>`.
    ///
    /// If the path is already known, the existing `Arc<str>` reference is cloned
    /// without any new string allocation.
    pub fn intern(&self, path: &str) -> Arc<str> {
        // Fast path: attempt shared read lock
        if let Ok(pool) = self.pool.read() {
            if let Some(existing) = pool.get(path) {
                return Arc::clone(existing);
            }
        }

        // Slow path: acquire exclusive write lock
        let mut pool = self.pool.write().unwrap_or_else(|poisoned| poisoned.into_inner());
        if let Some(existing) = pool.get(path) {
            return Arc::clone(existing);
        }

        let interned: Arc<str> = Arc::from(path);
        pool.insert(Arc::clone(&interned));
        interned
    }

    /// Returns the number of distinct path strings stored in the interner.
    #[must_use]
    pub fn len(&self) -> usize {
        self.pool.read().map_or(0, |pool| pool.len())
    }

    /// Returns `true` if the interner contains no paths.
    #[must_use]
    pub fn is_empty(&self) -> bool {
        self.len() == 0
    }

    /// Returns `true` if the given path has already been interned.
    pub fn contains(&self, path: &str) -> bool {
        self.pool
            .read()
            .is_ok_and(|pool| pool.contains(path))
    }

    /// Clears all interned paths from the pool.
    pub fn clear(&self) {
        if let Ok(mut pool) = self.pool.write() {
            pool.clear();
        }
    }
}

/// Global shared path interner instance.
static GLOBAL_PATH_INTERNER: Lazy<PathInterner> = Lazy::new(PathInterner::new);

/// Helper function to intern a path using the global `PathInterner`.
#[must_use]
pub fn intern_path(path: &str) -> Arc<str> {
    GLOBAL_PATH_INTERNER.intern(path)
}

/// Returns the number of distinct paths in the global interner.
#[allow(dead_code)]
#[must_use]
pub fn global_interner_len() -> usize {
    GLOBAL_PATH_INTERNER.len()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_interner_deduplicates_same_strings() {
        let interner = PathInterner::new();
        let p1 = interner.intern("src/contracts/token.rs");
        let p2 = interner.intern("src/contracts/token.rs");

        assert_eq!(p1, p2);
        // Verify pointer equality (they point to the exact same buffer on heap)
        assert!(Arc::ptr_eq(&p1, &p2));
        assert_eq!(interner.len(), 1);
        assert!(!interner.is_empty());
    }

    #[test]
    fn test_interner_multiple_distinct_strings() {
        let interner = PathInterner::new();
        let p1 = interner.intern("src/lib.rs");
        let p2 = interner.intern("src/main.rs");
        let p3 = interner.intern("src/types.rs");

        assert_ne!(p1, p2);
        assert_ne!(p2, p3);
        assert_eq!(interner.len(), 3);
        assert!(interner.contains("src/lib.rs"));
        assert!(!interner.contains("src/unknown.rs"));
    }

    #[test]
    fn test_global_intern_path() {
        let p1 = intern_path("soroban/contracts/vault.rs");
        let p2 = intern_path("soroban/contracts/vault.rs");

        assert_eq!(p1, p2);
        assert!(Arc::ptr_eq(&p1, &p2));
    }

    #[test]
    fn test_interner_clear() {
        let interner = PathInterner::new();
        interner.intern("test1.rs");
        interner.intern("test2.rs");
        assert_eq!(interner.len(), 2);

        interner.clear();
        assert_eq!(interner.len(), 0);
        assert!(interner.is_empty());
    }

    #[test]
    fn test_multithreaded_interning() {
        use std::thread;

        let interner = Arc::new(PathInterner::new());
        let mut handles = Vec::new();

        for _ in 0..8 {
            let interner = Arc::clone(&interner);
            handles.push(thread::spawn(move || {
                for i in 0..100 {
                    let path = format!("file_{}.rs", i % 5);
                    let _ = interner.intern(&path);
                }
            }));
        }

        for h in handles {
            h.join().unwrap();
        }

        // Only 5 unique files should exist in pool
        assert_eq!(interner.len(), 5);
    }
}
