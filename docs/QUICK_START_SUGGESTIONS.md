# Quick Start: Error Suggestions

## For Users

### Basic Usage

Debug any failed transaction to get suggestions:

```bash
erst debug <transaction-hash> --network testnet
```

The output will include a "Potential Fixes" section:

```
=== Potential Fixes (Heuristic Analysis) ===
⚠️  These are suggestions based on common error patterns. Always verify before applying.

1. 🔴 [Confidence: high]
   Potential Fix: Ensure you have called initialize() on this contract before invoking other functions.
```

### Understanding Confidence Levels

- 🔴 **High**: Strong evidence, likely the correct fix
- 🟡 **Medium**: Possible cause, investigate further
- 🟢 **Low**: Speculative, consider as last resort

### Common Scenarios

#### "Contract not initialized"
```bash
# Your contract needs initialization
# Fix: Call initialize() first
stellar contract invoke \
  --id <contract-id> \
  --source <account> \
  -- initialize
```

#### "Insufficient balance"
```bash
# Add more XLM to your account
stellar account fund <your-address>
```

#### "Missing authorization"
```bash
# Ensure you're signing with the correct account
stellar contract invoke \
  --id <contract-id> \
  --source <authorized-account> \
  -- your_function
```

## For Developers

### Using in Code

```go
import "github.com/dotandev/hintents/internal/decoder"

// Create engine
engine := decoder.NewSuggestionEngine()

// Analyze events
suggestions := engine.AnalyzeEvents(events)

// Display
fmt.Print(decoder.FormatSuggestions(suggestions))
```

### Adding Custom Rules

```go
// Define your rule
rule := decoder.ErrorPattern{
    Name:     "my_custom_error",
    Keywords: []string{"custom", "error"},
    Suggestion: decoder.Suggestion{
        Rule:        "my_custom_error",
        Description: "Potential Fix: Your custom fix here",
        Confidence:  "high",
    },
}

// Add to engine
engine.AddCustomRule(rule)
```

### Testing Your Rules

```go
func TestMyCustomRule(t *testing.T) {
    engine := decoder.NewSuggestionEngine()
    engine.AddCustomRule(myRule)
    
    events := []decoder.DecodedEvent{
        {Topics: []string{"custom_error"}},
    }
    
    suggestions := engine.AnalyzeEvents(events)
    // Assert suggestions contain your rule
}
```

## Tips

1. **Always verify suggestions** - They're heuristic-based, not guaranteed
2. **Check confidence levels** - Start with high-confidence suggestions
3. **Combine with traces** - Use suggestions alongside execution traces
4. **Report false positives** - Help improve the engine

## Examples

### Example 1: Uninitialized Token Contract

```bash
$ erst debug abc123... --network testnet

=== Potential Fixes ===
1. 🔴 [Confidence: high]
   Potential Fix: Ensure you have called initialize() on this contract

# Solution:
$ stellar contract invoke --id <token> -- initialize \
    --admin <your-address> \
    --decimal 7 \
    --name "My Token" \
    --symbol "MTK"
```

### Example 2: Authorization Error

```bash
$ erst debug def456... --network testnet

=== Potential Fixes ===
1. 🔴 [Confidence: high]
   Potential Fix: Verify that all required signatures are present

# Solution: Check you're using the right account
$ stellar keys show
$ stellar contract invoke --source <correct-account> ...
```

### Example 3: Resource Limits

```bash
$ erst debug ghi789... --network testnet

=== Potential Fixes ===
1. 🟡 [Confidence: medium]
   Potential Fix: Optimize your contract code to reduce CPU/memory usage

# Solution: Increase resource limits
$ stellar contract invoke \
    --fee 1000000 \
    --instructions 10000000 \
    ...
```

## Need Help?

- [DOC] Full docs: [ERROR_SUGGESTIONS.md](ERROR_SUGGESTIONS.md)
- [BUG] Report issues: GitHub Issues
- 💬 Ask questions: GitHub Discussions
- [LOG] Examples: [suggestions_example.go](../internal/decoder/suggestions_example.go)

## License

Copyright 2025 Erst Users  
SPDX-License-Identifier: Apache-2.0
