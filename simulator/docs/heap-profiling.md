# Heap Profiling

The simulator supports heap profiling to track memory allocation before and after contract execution.

## Usage

Add the `--heap-profile` flag when running the simulator:

```bash
./simulator --heap-profile < input.json
```

## Output

When enabled, the simulator will create two files:

- `heap_profile_before.txt` - Memory statistics before contract execution
- `heap_profile_after.txt` - Memory statistics after contract execution

Each file contains:
- Allocated bytes: Total bytes allocated by the application
- Resident bytes: Total bytes in physical memory

## Example

```bash
echo '{"envelope_xdr":"...","result_meta_xdr":"...","ledger_entries":null,"contract_wasm":null,"enable_optimization_advisor":false,"profile":false,"timestamp":"2026-02-24T15:30:00Z"}' | ./simulator --heap-profile
```

This will generate heap profile snapshots showing memory usage patterns during contract execution.

## Platform Support

Heap profiling is supported on all platforms except MSVC (Windows with Microsoft Visual C++). On MSVC, the flag is accepted but profiling is disabled with a warning message.

## Implementation

The feature uses jemalloc's statistics interface to capture memory metrics at key execution points.
