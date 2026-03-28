# TypeScript Bindings Architecture

## System Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        Developer Workflow                        │
└─────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                     1. Write Soroban Contract                    │
│                                                                   │
│  #[contract]                                                      │
│  pub struct MyContract;                                           │
│                                                                   │
│  #[contractimpl]                                                  │
│  impl MyContract {                                                │
│      pub fn transfer(from: Address, to: Address, amount: i128)   │
│  }                                                                │
└─────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                     2. Build Contract (Rust)                     │
│                                                                   │
│  $ soroban contract build                                         │
│                                                                   │
│  Output: contract.wasm (with embedded spec)                       │
└─────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│              3. Generate Bindings (erst CLI - Go)                │
│                                                                   │
│  $ erst generate-bindings contract.wasm                           │
│                                                                   │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  Extract Contract Spec (internal/abi/wasm.go)           │    │
│  │  - Read WASM custom section "contractspecv0"            │    │
│  │  - Decode XDR-encoded spec entries                      │    │
│  └─────────────────────────────────────────────────────────┘    │
│                          │                                        │
│                          ▼                                        │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  Parse Spec (internal/abi/spec.go)                      │    │
│  │  - Functions, Structs, Enums, Unions, Events            │    │
│  │  - Type definitions and parameters                       │    │
│  └─────────────────────────────────────────────────────────┘    │
│                          │                                        │
│                          ▼                                        │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  Generate TypeScript (internal/bindings/generator.go)   │    │
│  │  - Map Soroban types → TypeScript types                 │    │
│  │  - Generate interfaces, enums, unions                    │    │
│  │  - Create client class with typed methods               │    │
│  │  - Add erst simulator integration                        │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                    4. Generated TypeScript Files                 │
│                                                                   │
│  types.ts              - Type definitions                         │
│  client.ts             - Client class                             │
│  erst-integration.ts   - Simulator                                │
│  index.ts              - Exports                                  │
│  package.json          - NPM config                               │
│  README.md             - Documentation                            │
└─────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                   5. Frontend Application (TypeScript)           │
│                                                                   │
│  import { MyContractClient } from './bindings';                   │
│                                                                   │
│  const client = new MyContractClient({                            │
│    contractId: 'CDLZFC...',                                       │
│    network: 'testnet',                                            │
│    enableSimulation: true                                         │
│  });                                                              │
│                                                                   │
│  // Type-safe method call                                         │
│  const result = await client.transfer(                            │
│    sourceKeypair,                                                 │
│    'RECIPIENT_ADDRESS',                                           │
│    BigInt(1000000),                                               │
│    { simulate: true }                                             │
│  );                                                               │
└─────────────────────────────────────────────────────────────────┘
```

## Component Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         erst CLI (Go)                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  cmd/generate-bindings.go                                         │
│  ├─ Parse CLI arguments                                           │
│  ├─ Read WASM file                                                │
│  └─ Invoke generator                                              │
│                                                                   │
│  internal/bindings/generator.go                                   │
│  ├─ Extract spec from WASM                                        │
│  ├─ Generate types.ts                                             │
│  ├─ Generate client.ts                                            │
│  ├─ Generate erst-integration.ts                                  │
│  └─ Write files to disk                                           │
│                                                                   │
│  internal/abi/                                                    │
│  ├─ wasm.go: Extract custom sections                              │
│  ├─ spec.go: Decode contract spec                                 │
│  └─ printer.go: Format types                                      │
└─────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Generated TypeScript Package                   │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  types.ts                                                         │
│  ├─ Base types (Address, Bytes, Symbol)                          │
│  ├─ Struct interfaces                                             │
│  ├─ Enum types                                                    │
│  ├─ Union types (discriminated)                                   │
│  ├─ Error enums + classes                                         │
│  └─ Event interfaces                                              │
│                                                                   │
│  client.ts                                                        │
│  ├─ ClientConfig interface                                        │
│  ├─ CallOptions interface                                         │
│  ├─ CallResult<T> interface                                       │
│  ├─ Client class                                                  │
│  │   ├─ Constructor (config)                                      │
│  │   ├─ Contract methods (typed)                                  │
│  │   ├─ Helper methods                                            │
│  │   └─ Network configuration                                     │
│  └─ Stellar SDK integration                                       │
│                                                                   │
│  erst-integration.ts                                              │
│  ├─ ErstConfig interface                                          │
│  ├─ SimulationResult interface                                    │
│  ├─ ErstSimulator class                                           │
│  │   ├─ simulate(tx)                                              │
│  │   └─ debugTransaction(hash)                                    │
│  └─ Process spawning (erst CLI)                                   │
└─────────────────────────────────────────────────────────────────┘
```

## Runtime Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                      Frontend Application                        │
└─────────────────────────────────────────────────────────────────┘
                                 │
                                 │ client.transfer(...)
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Generated Client Class                      │
├─────────────────────────────────────────────────────────────────┤
│  1. Validate parameters (TypeScript types)                        │
│  2. Build Stellar transaction                                     │
│  3. Check if simulation requested                                 │
└─────────────────────────────────────────────────────────────────┘
                    │                              │
         simulate=true                  simulate=false
                    │                              │
                    ▼                              ▼
┌──────────────────────────────┐  ┌──────────────────────────────┐
│    ErstSimulator.simulate    │  │   Stellar Network (RPC)      │
├──────────────────────────────┤  ├──────────────────────────────┤
│  1. Convert tx to XDR        │  │  1. Sign transaction         │
│  2. Spawn erst CLI process   │  │  2. Submit to network        │
│  3. Pass XDR via stdin       │  │  3. Wait for confirmation    │
│  4. Parse JSON response      │  │  4. Parse result             │
│  5. Return simulation result │  │  5. Return transaction hash  │
└──────────────────────────────┘  └──────────────────────────────┘
                    │                              │
                    ▼                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                         erst CLI (Go)                            │
├─────────────────────────────────────────────────────────────────┤
│  1. Read XDR from stdin                                           │
│  2. Fetch ledger state from network                               │
│  3. Spawn erst-sim (Rust) subprocess                              │
│  4. Pass simulation request                                       │
└─────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                      erst-sim (Rust)                             │
├─────────────────────────────────────────────────────────────────┤
│  1. Decode XDR                                                    │
│  2. Initialize Soroban Host VM                                    │
│  3. Load ledger state                                             │
│  4. Execute transaction                                           │
│  5. Capture events, logs, budget                                  │
│  6. Return simulation result                                      │
└─────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Simulation Result                           │
├─────────────────────────────────────────────────────────────────┤
│  {                                                                │
│    status: 'success',                                             │
│    result: <typed-value>,                                         │
│    events: [...],                                                 │
│    logs: [...],                                                   │
│    budgetUsage: {                                                 │
│      cpuInstructions: 12345,                                      │
│      memoryBytes: 1024,                                           │
│      cpuUsagePercent: 12.34,                                      │
│      memoryUsagePercent: 5.67                                     │
│    }                                                              │
│  }                                                                │
└─────────────────────────────────────────────────────────────────┘
```

## Type Mapping System

```
┌─────────────────────────────────────────────────────────────────┐
│                    Soroban Type System                           │
└─────────────────────────────────────────────────────────────────┘
                                 │
                                 │ mapTypeDefToTS()
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                   TypeScript Type System                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  Primitives:                                                      │
│  Bool          → boolean                                          │
│  U32, I32      → number                                           │
│  U64, I64      → bigint                                           │
│  U128, I128    → bigint                                           │
│  String        → string                                           │
│  Symbol        → string                                           │
│  Address       → string                                           │
│  Bytes         → Uint8Array                                       │
│                                                                   │
│  Containers:                                                      │
│  Option<T>     → T | null                                         │
│  Vec<T>        → T[]                                              │
│  Map<K, V>     → Map<K, V>                                        │
│  Tuple<T1, T2> → [T1, T2]                                         │
│                                                                   │
│  Custom Types:                                                    │
│  Struct        → interface                                        │
│  Enum          → enum                                             │
│  Union         → Discriminated union                              │
│  Error Enum    → enum + Error class                               │
└─────────────────────────────────────────────────────────────────┘
```

## Data Flow Example

```
Contract Method: transfer(from: Address, to: Address, amount: i128)

1. Soroban Contract Spec (XDR):
   ┌────────────────────────────────────────┐
   │ ScSpecFunctionV0 {                     │
   │   name: "transfer",                    │
   │   inputs: [                            │
   │     { name: "from", type: Address },   │
   │     { name: "to", type: Address },     │
   │     { name: "amount", type: I128 }     │
   │   ],                                   │
   │   outputs: [Void]                      │
   │ }                                      │
   └────────────────────────────────────────┘
                    │
                    │ Generator
                    ▼
2. Generated TypeScript:
   ┌────────────────────────────────────────┐
   │ async transfer(                        │
   │   source: StellarSdk.Keypair,          │
   │   from: Address,                       │
   │   to: Address,                         │
   │   amount: bigint,                      │
   │   options?: CallOptions                │
   │ ): Promise<CallResult<void>>           │
   └────────────────────────────────────────┘
                    │
                    │ Runtime
                    ▼
3. Frontend Call:
   ┌────────────────────────────────────────┐
   │ await client.transfer(                 │
   │   keypair,                             │
   │   'GDQP2K...',                          │
   │   'GC3C4A...',                          │
   │   BigInt(1000000),                     │
   │   { simulate: true }                   │
   │ )                                      │
   └────────────────────────────────────────┘
```

## Security & Validation

```
┌─────────────────────────────────────────────────────────────────┐
│                        Type Safety Layers                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  Layer 1: TypeScript Compiler                                     │
│  ├─ Compile-time type checking                                    │
│  ├─ Parameter validation                                          │
│  └─ Return type verification                                      │
│                                                                   │
│  Layer 2: Generated Client                                        │
│  ├─ Runtime parameter validation                                  │
│  ├─ Network configuration checks                                  │
│  └─ Transaction building validation                               │
│                                                                   │
│  Layer 3: Stellar SDK                                             │
│  ├─ XDR encoding validation                                       │
│  ├─ Signature verification                                        │
│  └─ Network protocol compliance                                   │
│                                                                   │
│  Layer 4: erst Simulation                                         │
│  ├─ Contract execution validation                                 │
│  ├─ Budget limit checks                                           │
│  └─ State consistency verification                                │
│                                                                   │
│  Layer 5: Soroban Network                                         │
│  ├─ Final transaction validation                                  │
│  ├─ Authorization checks                                          │
│  └─ State commitment                                              │
└─────────────────────────────────────────────────────────────────┘
```

## Extension Points

```
┌─────────────────────────────────────────────────────────────────┐
│                      Future Enhancements                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  1. Framework Integrations                                        │
│     ├─ React hooks generator                                      │
│     ├─ Vue composables generator                                  │
│     └─ Svelte stores generator                                    │
│                                                                   │
│  2. Additional Outputs                                            │
│     ├─ GraphQL schema                                             │
│     ├─ OpenAPI specification                                      │
│     └─ Protocol Buffers                                           │
│                                                                   │
│  3. Testing Utilities                                             │
│     ├─ Mock data generators                                       │
│     ├─ Fixture builders                                           │
│     └─ Snapshot testing                                           │
│                                                                   │
│  4. Development Tools                                             │
│     ├─ Contract upgrade helpers                                   │
│     ├─ Multi-contract orchestration                               │
│     └─ State migration tools                                      │
└─────────────────────────────────────────────────────────────────┘
```

## Performance Considerations

```
┌─────────────────────────────────────────────────────────────────┐
│                      Optimization Strategy                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  Generation Time:                                                 │
│  ├─ Single-pass spec parsing                                      │
│  ├─ Efficient string building                                     │
│  └─ Minimal file I/O                                              │
│                                                                   │
│  Runtime Performance:                                             │
│  ├─ Lazy client initialization                                    │
│  ├─ Connection pooling                                            │
│  ├─ Response caching                                              │
│  └─ Batch transaction support                                     │
│                                                                   │
│  Simulation Performance:                                          │
│  ├─ Process reuse                                                 │
│  ├─ State caching                                                 │
│  └─ Parallel execution                                            │
└─────────────────────────────────────────────────────────────────┘
```
