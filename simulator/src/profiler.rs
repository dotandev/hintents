// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

pub enum FlamegraphKind {
    Cpu,
    MemoryAllocation,
}

pub fn build_flamegraph_input(cpu_insns: u64, mem_bytes: u64, kind: FlamegraphKind) -> String {
    let label = match kind {
        FlamegraphKind::Cpu => "CPU",
        FlamegraphKind::MemoryAllocation => "Memory Allocation",
    };

    format!("Total;{label} {cpu_insns}\nTotal;Memory {mem_bytes}\n")
}

#[cfg(test)]
mod tests {
    use super::{build_flamegraph_input, FlamegraphKind};

    #[test]
    fn memory_flamegraph_input_uses_allocation_label() {
        let input = build_flamegraph_input(10, 42, FlamegraphKind::MemoryAllocation);
        assert!(input.contains("Memory Allocation"));
        assert!(input.contains("42"));
    }

    #[test]
    fn cpu_flamegraph_input_uses_cpu_label() {
        let input = build_flamegraph_input(10, 42, FlamegraphKind::Cpu);
        assert!(input.contains("CPU"));
        assert!(input.contains("10"));
    }
}
