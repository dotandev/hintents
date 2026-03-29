# Suggestion Engine Flow Diagram

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    erst debug <tx-hash>                      │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│              Fetch Transaction from Network                  │
│  - Transaction Envelope (XDR)                               │
│  - Result Metadata (XDR)                                    │
│  - Diagnostic Events                                        │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                  Run Simulation                              │
│  - Execute transaction locally                              │
│  - Capture events and logs                                  │
│  - Generate execution trace                                 │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│              Decode Events (decoder.DecodeEvents)            │
│  - Parse XDR diagnostic events                              │
│  - Build call tree structure                                │
│  - Extract topics and data                                  │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│         Suggestion Engine (AnalyzeCallTree)                  │
│                                                              │
│  ┌────────────────────────────────────────────────┐        │
│  │  1. Collect all events from call tree          │        │
│  └────────────────┬───────────────────────────────┘        │
│                   │                                          │
│                   ▼                                          │
│  ┌────────────────────────────────────────────────┐        │
│  │  2. For each event, check against rules:       │        │
│  │     - Match keywords in topics/data             │        │
│  │     - Run event-specific checks                 │        │
│  │     - Collect matching suggestions              │        │
│  └────────────────┬───────────────────────────────┘        │
│                   │                                          │
│                   ▼                                          │
│  ┌────────────────────────────────────────────────┐        │
│  │  3. Deduplicate suggestions                     │        │
│  │     - Each rule triggers only once              │        │
│  └────────────────┬───────────────────────────────┘        │
│                   │                                          │
│                   ▼                                          │
│  ┌────────────────────────────────────────────────┐        │
│  │  4. Return suggestions with confidence          │        │
│  └────────────────────────────────────────────────┘        │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│           Format Suggestions (FormatSuggestions)             │
│  - Add header and warning                                   │
│  - Format with confidence icons                             │
│  - Number suggestions                                       │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                    Display to User                           │
│                                                              │
│  === Potential Fixes (Heuristic Analysis) ===               │
│  ⚠️  These are suggestions based on common patterns          │
│                                                              │
│  1. 🔴 [Confidence: high]                                   │
│     Potential Fix: Ensure you have called initialize()      │
│                                                              │
│  2. 🟡 [Confidence: medium]                                 │
│     Potential Fix: Check parameter types                    │
└─────────────────────────────────────────────────────────────┘
```

## Rule Matching Flow

```
┌─────────────────────────────────────────────────────────────┐
│                    Decoded Event                             │
│  {                                                           │
│    ContractID: "abc123",                                    │
│    Topics: ["storage_empty", "error"],                      │
│    Data: "ScvVoid"                                          │
│  }                                                           │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│              Check Against All Rules                         │
└────────────────────────┬────────────────────────────────────┘
                         │
        ┌────────────────┼────────────────┐
        │                │                │
        ▼                ▼                ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│   Rule 1:    │  │   Rule 2:    │  │   Rule 3:    │
│ Uninitialized│  │    Missing   │  │ Insufficient │
│   Contract   │  │     Auth     │  │   Balance    │
└──────┬───────┘  └──────┬───────┘  └──────┬───────┘
       │                 │                 │
       ▼                 ▼                 ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│  Keywords:   │  │  Keywords:   │  │  Keywords:   │
│  "empty"     │  │  "auth"      │  │  "balance"   │
│  "missing"   │  │  "unauthorized"│ │  "insufficient"│
│  "null"      │  │  "permission"│  │  "underfunded"│
└──────┬───────┘  └──────┬───────┘  └──────┬───────┘
       │                 │                 │
       ▼                 ▼                 ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│  Match?      │  │  Match?      │  │  Match?      │
│  [OK] YES       │  │  [FAIL] NO        │  │  [FAIL] NO        │
└──────┬───────┘  └──────────────┘  └──────────────┘
       │
       ▼
┌──────────────────────────────────────────────────┐
│  Event Checks:                                    │
│  - Check if topics contain "storage"              │
│  - Check if topics contain "empty"                │
│  [OK] PASS                                           │
└──────┬───────────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────────────┐
│  Add Suggestion:                                  │
│  {                                                │
│    Rule: "uninitialized_contract",               │
│    Description: "Ensure you have called          │
│                  initialize()...",               │
│    Confidence: "high"                            │
│  }                                                │
└───────────────────────────────────────────────────┘
```

## Call Tree Analysis

```
┌─────────────────────────────────────────────────────────────┐
│                      Call Tree                               │
│                                                              │
│  ROOT (TOP_LEVEL)                                           │
│  ├─ Event: fn_call "execute"                               │
│  ├─ Event: storage_empty          ← Triggers Rule 1        │
│  │                                                           │
│  └─ SubCall: contract_a.transfer                           │
│     ├─ Event: fn_call "transfer"                           │
│     ├─ Event: insufficient_balance ← Triggers Rule 3        │
│     │                                                        │
│     └─ SubCall: contract_b.check_auth                      │
│        ├─ Event: fn_call "check_auth"                      │
│        └─ Event: unauthorized      ← Triggers Rule 2        │
└─────────────────────────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│              Collect All Events (Recursive)                  │
│  [                                                           │
│    {Topics: ["fn_call", "execute"]},                        │
│    {Topics: ["storage_empty"]},           ← Rule 1          │
│    {Topics: ["fn_call", "transfer"]},                       │
│    {Topics: ["insufficient_balance"]},    ← Rule 3          │
│    {Topics: ["fn_call", "check_auth"]},                     │
│    {Topics: ["unauthorized"]}             ← Rule 2          │
│  ]                                                           │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                  Analyze All Events                          │
│  - Rule 1 matches: storage_empty                            │
│  - Rule 2 matches: unauthorized                             │
│  - Rule 3 matches: insufficient_balance                     │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│              Return 3 Suggestions                            │
│  1. Uninitialized contract (high)                           │
│  2. Missing authorization (high)                            │
│  3. Insufficient balance (high)                             │
└─────────────────────────────────────────────────────────────┘
```

## Confidence Level Decision Tree

```
                    ┌─────────────────┐
                    │  Pattern Match  │
                    └────────┬────────┘
                             │
                ┌────────────┴────────────┐
                │                         │
                ▼                         ▼
        ┌───────────────┐         ┌───────────────┐
        │  Multiple     │         │  Single       │
        │  Indicators   │         │  Indicator    │
        └───────┬───────┘         └───────┬───────┘
                │                         │
                ▼                         ▼
        ┌───────────────┐         ┌───────────────┐
        │  Well-known   │         │  Ambiguous    │
        │  Error        │         │  Pattern      │
        └───────┬───────┘         └───────┬───────┘
                │                         │
                ▼                         ▼
        ┌───────────────┐         ┌───────────────┐
        │  HIGH         │         │  MEDIUM       │
        │  Confidence   │         │  Confidence   │
        │  🔴           │         │  🟡           │
        └───────────────┘         └───────────────┘
                                          │
                                          │
                                  ┌───────┴───────┐
                                  │               │
                                  ▼               ▼
                          ┌───────────┐   ┌───────────┐
                          │ Speculative│   │  LOW      │
                          │ Match     │   │ Confidence│
                          └─────┬─────┘   │  🟢       │
                                │         └───────────┘
                                └─────────────┘
```

## Integration Points

```
┌─────────────────────────────────────────────────────────────┐
│                    erst CLI (cmd/debug.go)                   │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│              Simulator (internal/simulator)                  │
│  - Runs transaction                                         │
│  - Returns events and logs                                  │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│              Decoder (internal/decoder)                      │
│  ┌────────────────────────────────────────────────┐        │
│  │  decoder.DecodeEvents(eventsXdr)               │        │
│  │    → CallNode tree                             │        │
│  └────────────────────────────────────────────────┘        │
│                         │                                    │
│                         ▼                                    │
│  ┌────────────────────────────────────────────────┐        │
│  │  suggestionEngine.AnalyzeCallTree(callTree)    │        │
│  │    → []Suggestion                              │        │
│  └────────────────────────────────────────────────┘        │
│                         │                                    │
│                         ▼                                    │
│  ┌────────────────────────────────────────────────┐        │
│  │  FormatSuggestions(suggestions)                │        │
│  │    → formatted string                          │        │
│  └────────────────────────────────────────────────┘        │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                    Display to User                           │
└─────────────────────────────────────────────────────────────┘
```

## License

Copyright 2026 Erst Users  
SPDX-License-Identifier: Apache-2.0
