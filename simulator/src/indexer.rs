// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Concurrent event indexing for simulator executions.
//!
//! Event emission can happen from more than one simulated transaction.  The
//! index therefore uses sharded storage for reads and writes while retaining a
//! monotonically increasing sequence number so callers can recover emission
//! order when producing a response or rolling back a snapshot.

use dashmap::DashMap;
use soroban_env_host::events::HostEvent;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Arc;

#[derive(Clone, Default)]
pub struct EventIndexer {
    events: Arc<DashMap<u64, HostEvent>>,
    next_sequence: Arc<AtomicU64>,
}

impl EventIndexer {
    /// Adds an event and returns its stable emission sequence.
    pub fn insert(&self, event: HostEvent) -> u64 {
        let sequence = self.next_sequence.fetch_add(1, Ordering::Relaxed);
        self.events.insert(sequence, event);
        sequence
    }

    /// Returns a cloned event by its emission sequence.
    #[allow(dead_code)]
    pub fn get(&self, sequence: u64) -> Option<HostEvent> {
        self.events.get(&sequence).map(|event| event.clone())
    }

    /// Returns all indexed events in emission order.
    pub fn snapshot(&self) -> Vec<HostEvent> {
        let mut indexed = self
            .events
            .iter()
            .map(|entry| (*entry.key(), entry.value().clone()))
            .collect::<Vec<_>>();
        indexed.sort_unstable_by_key(|(sequence, _)| *sequence);
        indexed.into_iter().map(|(_, event)| event).collect()
    }

    /// Removes events at and after `len`, preserving the earlier history.
    pub fn truncate(&self, len: usize) {
        let cutoff = len as u64;
        let sequences = self
            .events
            .iter()
            .filter_map(|entry| (*entry.key() >= cutoff).then_some(*entry.key()))
            .collect::<Vec<_>>();
        for sequence in sequences {
            self.events.remove(&sequence);
        }
    }

    pub fn len(&self) -> usize {
        self.events.len()
    }

    #[allow(dead_code)]
    pub fn is_empty(&self) -> bool {
        self.events.is_empty()
    }
}

#[cfg(test)]
mod tests {
    use super::EventIndexer;
    use soroban_env_host::events::HostEvent;
    use soroban_env_host::xdr::{
        ContractEvent, ContractEventBody, ContractEventType, ContractEventV0, ExtensionPoint,
        ScVal, VecM,
    };
    use std::sync::Arc;
    use std::thread;

    fn event() -> HostEvent {
        HostEvent {
            event: ContractEvent {
                ext: ExtensionPoint::V0,
                contract_id: None,
                type_: ContractEventType::Diagnostic,
                body: ContractEventBody::V0(ContractEventV0 {
                    topics: VecM::default(),
                    data: ScVal::Void,
                }),
            },
            failed_call: false,
        }
    }

    #[test]
    fn concurrent_inserts_are_retained_and_ordered() {
        let indexer = Arc::new(EventIndexer::default());
        let workers = (0..8)
            .map(|_| {
                let indexer = Arc::clone(&indexer);
                thread::spawn(move || {
                    for _ in 0..32 {
                        indexer.insert(event());
                    }
                })
            })
            .collect::<Vec<_>>();

        for worker in workers {
            worker.join().expect("event worker should complete");
        }

        assert_eq!(indexer.len(), 256);
        assert_eq!(indexer.snapshot().len(), 256);
        assert!(indexer.get(0).is_some());
    }

    #[test]
    fn truncate_removes_only_future_events() {
        let indexer = EventIndexer::default();
        for _ in 0..3 {
            indexer.insert(event());
        }

        indexer.truncate(2);

        assert_eq!(indexer.len(), 2);
        assert!(indexer.get(1).is_some());
        assert!(indexer.get(2).is_none());
    }
}
