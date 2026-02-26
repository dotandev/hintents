# Error Suggestion Engine

## Overview

The Error Suggestion Engine is a heuristic-based system that analyzes Soroban transaction failures and provides actionable suggestions to help developers fix common errors. This feature is particularly valuable for junior developers transitioning to Stellar development.

## Features

- **Heuristic Analysis**: Pattern matching against common error scenarios
- **Confidence Levels**: Each suggestion includes a confidence rating (high, medium, low)
- **Extensible Rules**: Support for custom error patterns
- **Call Tree Analysis**: Analyzes entire execution traces including nested contract calls
- **Clear Marking**: All suggestions are clearly marked as "Potential Fixes"

## Built-in Rules

### 1. Uninitialized Contract
**Confidence**: High

**Triggers when**:
- Events contain keywords: "empty", "not found", "missing", "null"
- Storage-related events indicate empty state

**Suggestion**:
```
Potential Fix: Ensure you have called initialize() on this contract before invoking other functions.
```

### 2. Missing Authorization
**Confidence**: High

**Triggers when**:
- Events contain keywords: "auth", "unauthorized", "permission", "signature"
- Authorization-related failures detected

**Suggestion**:
```
Potential Fix: Verify that all required signatures are present and the invoker has proper authorization.
```

### 3. Insufficient Balance
**Confidence**: High

**Triggers when**:
- Events contain keywords: "balance", "insufficient", "underfunded", "funds"
- Balance-related errors detected

**Suggestion**:
```
Potential Fix: Ensure the account has sufficient balance to cover the transaction and maintain minimum reserves.
```

### 4. Invalid Parameters
**Confidence**: Medium

**Triggers when**:
- Events contain keywords: "invalid", "malformed", "bad", "parameter"
- Parameter validation failures detected

**Suggestion**:
```
Potential Fix: Check that all function parameters match the expected types and constraints.
```

### 5. Contract Not Found
**Confidence**: High

**Triggers when**:
- Contract ID is empty or all zeros
- Events indicate missing contract

**Suggestion**:
```
Potential Fix: Verify the contract ID is correct and the contract has been deployed to the network.
```

### 6. Resource Limit Exceeded
**Confidence**: Medium

**Triggers when**:
- Events contain keywords: "limit", "exceeded", "quota", "budget"
- Resource exhaustion detected

**Suggestion**:
```
Potential Fix: Optimize your contract code to reduce CPU/memory usage, or increase resource limits in the transaction.
```

### 7. Reentrancy Detected
**Confidence**: Medium

**Triggers when**:
- Events contain keywords: "reentrant", "recursive", "loop"
- Reentrancy patterns detected

**Suggestion**:
```
Potential Fix: Implement reentrancy guards or use the checks-effects-interactions pattern to prevent recursive calls.
```

## Usage

### CLI Integration

The suggestion engine is automatically integrated into the `erst debug` command:

```bash
erst debug <transaction-hash> --network testnet
```

Output example:
```
=== Potential Fixes (Heuristic Analysis) ===
⚠️  These are suggestions based on common error patterns. Always verify before applying.

1. 🔴 [Confidence: high]
   Potential Fix: Ensure you have called initialize() on this contract before invoking other functions.

2. 🟡 [Confidence: medium]
   Potential Fix: Optimize your contract code to reduce CPU/memory usage, or increase resource limits in the transaction.
```

### Programmatic Usage

```go
import "github.com/dotandev/hintents/internal/decoder"

// Create engine
engine := decoder.NewSuggestionEngine()

// Analyze events
events := []decoder.DecodedEvent{
    {
        ContractID: "abc123",
        Topics:     []string{"storage_empty", "error"},
        Data:       "ScvVoid",
    },
}

suggestions := engine.AnalyzeEvents(events)

// Format and display
output := decoder.FormatSuggestions(suggestions)
fmt.Println(output)
```

### Analyzing Call Trees

```go
// Decode events into a call tree
callTree, err := decoder.DecodeEvents(eventsXdr)
if err != nil {
    log.Fatal(err)
}

// Analyze the entire call tree
suggestions := engine.AnalyzeCallTree(callTree)
```

## Adding Custom Rules

You can extend the engine with custom rules for project-specific errors:

```go
customRule := decoder.ErrorPattern{
    Name:     "custom_timeout",
    Keywords: []string{"timeout", "deadline", "expired"},
    EventChecks: []func(decoder.DecodedEvent) bool{
        func(e decoder.DecodedEvent) bool {
            // Custom logic to detect timeout
            return strings.Contains(e.Data, "timeout")
        },
    },
    Suggestion: decoder.Suggestion{
        Rule:        "custom_timeout",
        Description: "Potential Fix: Increase the transaction timeout or optimize contract execution time.",
        Confidence:  "medium",
    },
}

engine.AddCustomRule(customRule)
```

## Confidence Levels

### High Confidence (🔴)
- Strong pattern match with well-known error scenarios
- Multiple indicators point to the same issue
- Solution is straightforward and commonly applicable

### Medium Confidence (🟡)
- Partial pattern match or ambiguous indicators
- Multiple possible causes
- Solution may require additional investigation

### Low Confidence (🟢)
- Weak pattern match or speculative
- Limited evidence
- Suggestion is exploratory

## Best Practices

### For Users

1. **Always Verify**: Suggestions are heuristic-based and may not always be accurate
2. **Check Confidence**: Prioritize high-confidence suggestions
3. **Context Matters**: Consider your specific contract logic and use case
4. **Combine with Other Tools**: Use suggestions alongside trace analysis and security findings

### For Developers

1. **Keep Rules Focused**: Each rule should target a specific error pattern
2. **Avoid False Positives**: Test rules against various scenarios
3. **Document Patterns**: Clearly document what triggers each rule
4. **Update Regularly**: Add new rules as common patterns emerge

## Architecture

### Components

```
SuggestionEngine
├── rules []ErrorPattern
│   ├── Name
│   ├── Keywords
│   ├── EventChecks
│   └── Suggestion
└── Methods
    ├── AnalyzeEvents()
    ├── AnalyzeCallTree()
    └── AddCustomRule()
```

### Flow

1. **Event Collection**: Gather all diagnostic events from transaction
2. **Pattern Matching**: Check each event against rule keywords and conditions
3. **Deduplication**: Ensure each rule triggers only once
4. **Formatting**: Present suggestions with confidence indicators
5. **Display**: Show suggestions before security analysis

## Testing

Run the test suite:

```bash
go test ./internal/decoder/suggestions_test.go -v
```

Test coverage includes:
- All built-in rules
- Custom rule addition
- Call tree analysis
- Deduplication logic
- Formatting output
- Edge cases (empty events, no matches, etc.)

## Future Enhancements

- [ ] Machine learning-based pattern detection
- [ ] Integration with contract source code for context-aware suggestions
- [ ] Suggestion ranking based on historical accuracy
- [ ] Community-contributed rule database
- [ ] Multi-language support for suggestions
- [ ] Interactive suggestion refinement
- [ ] Link suggestions to documentation

## Contributing

To add new rules:

1. Identify a common error pattern
2. Define keywords and event checks
3. Write a clear, actionable suggestion
4. Add test cases
5. Document the rule in this file
6. Submit a PR with examples

## References

- [Soroban Documentation](https://soroban.stellar.org/docs)
- [Common Soroban Errors](https://soroban.stellar.org/docs/learn/errors)
- [Stellar Error Codes](https://developers.stellar.org/docs/encyclopedia/error-codes)

## License

Copyright 2025 Erst Users  
SPDX-License-Identifier: Apache-2.0
