# Security Vulnerability Detection

This module implements heuristic-based security vulnerability detection for Soroban smart contracts during transaction replay.

## Overview

The security detector analyzes transaction envelopes, simulation results, events, and logs to identify potential security vulnerabilities and suspicious patterns.

## Finding Types

### Verified Security Risk (`VERIFIED_RISK`)
Confirmed security issues detected through concrete evidence in execution traces:
- Integer overflow/underflow (detected via arithmetic error logs)
- Authorization failures (detected via auth check failures)
- Contract panics/traps (detected via panic events)

### Heuristic Warning (`HEURISTIC_WARNING`)
Potential security concerns based on pattern analysis:
- Large value transfers to unverified contracts
- Potential reentrancy patterns (multiple invocations with state changes)
- Authorization bypass patterns (privileged operations without auth checks)

## Severity Levels

- **HIGH**: Critical security issues requiring immediate attention
- **MEDIUM**: Significant concerns that should be reviewed
- **LOW**: Minor issues or best practice violations
- **INFO**: Informational findings for awareness

## Detected Vulnerabilities

### 1. Integer Overflow/Underflow
**Type**: VERIFIED_RISK  
**Severity**: HIGH

Detects arithmetic operations that fail due to overflow or underflow by analyzing error logs for keywords like `overflow`, `underflow`, `checked_add`, `checked_sub`, `checked_mul`.

### 2. Large Value Transfers
**Type**: HEURISTIC_WARNING  
**Severity**: HIGH/MEDIUM

Flags transfers exceeding thresholds:
- Native XLM: > 1M XLM
- Contract tokens: > 10M tokens (assuming 7 decimals)

### 3. Reentrancy Patterns
**Type**: HEURISTIC_WARNING  
**Severity**: MEDIUM

Detects transactions with multiple contract invocations combined with state changes, indicating potential reentrancy vulnerability.

### 4. Authorization Failures
**Type**: VERIFIED_RISK  
**Severity**: HIGH

Identifies failed authorization checks in contract execution through event analysis.

### 5. Authorization Bypass
**Type**: HEURISTIC_WARNING  
**Severity**: HIGH

Detects privileged operations (admin, owner functions) executed without corresponding authorization checks.

### 6. Contract Panics/Traps
**Type**: VERIFIED_RISK  
**Severity**: HIGH

Identifies contract execution panics or traps that indicate critical errors.

## Usage

```go
import "github.com/dotandev/hintents/internal/security"

// Create detector
detector := security.NewDetector()

// Analyze transaction
findings := detector.Analyze(
    envelopeXdr,
    resultMetaXdr,
    simulationEvents,
    simulationLogs,
)

// Process findings
for _, finding := range findings {
    fmt.Printf("[%s] %s - %s\n", 
        finding.Type, 
        finding.Severity, 
        finding.Title)
    fmt.Printf("  %s\n", finding.Description)
    if finding.Evidence != "" {
        fmt.Printf("  Evidence: %s\n", finding.Evidence)
    }
}
```

## CLI Integration

The security detector is automatically invoked during `erst debug` command:

```bash
./erst debug <transaction-hash>
```

Output includes:
```
=== Security Analysis ===
⚠️  VERIFIED SECURITY RISKS: 2
⚡ HEURISTIC WARNINGS: 1

Findings:

1. ⚠️ [VERIFIED_RISK] HIGH - Integer Overflow/Underflow Detected
   Arithmetic operation failed, indicating potential overflow or underflow
   Evidence: checked_add failed: overflow detected

2. ⚡ [HEURISTIC_WARNING] HIGH - Potential Authorization Bypass
   Privileged operation detected without corresponding authorization check
   Evidence: Review contract authorization logic
```

## Testing

Run tests:
```bash
go test ./internal/security/...
```

Run integration test with flawed contract:
```bash
go test -v ./internal/security -run TestDetector_FlawedContract
```

## Extending Detection Rules

To add new vulnerability checks:

1. Add detection method to `detector.go`:
```go
func (d *Detector) checkNewVulnerability(data interface{}) {
    // Analysis logic
    if vulnerabilityDetected {
        d.addFinding(Finding{
            Type:        FindingVerifiedRisk, // or FindingHeuristicWarn
            Severity:    SeverityHigh,
            Title:       "Vulnerability Name",
            Description: "Detailed description",
            Evidence:    "Supporting evidence",
        })
    }
}
```

2. Call from `Analyze()` method
3. Add test cases in `detector_test.go`

## Limitations

- **Heuristic-based**: Some warnings may be false positives
- **Pattern matching**: Limited to known vulnerability patterns
- **No static analysis**: Does not analyze contract source code
- **Threshold-based**: Large value detection uses fixed thresholds

## Best Practices

1. **Investigate all VERIFIED_RISK findings** - These indicate confirmed issues
2. **Review HEURISTIC_WARNING findings** - May require manual verification
3. **Adjust thresholds** - Customize for your use case if needed
4. **Combine with other tools** - Use alongside static analyzers and audits
5. **Test with known vulnerabilities** - Validate detection on known flawed contracts

## Future Enhancements

- [ ] Configurable thresholds via CLI flags
- [ ] Custom rule definitions via config file
- [ ] Integration with vulnerability databases
- [ ] Machine learning-based pattern detection
- [ ] Source code mapping for findings
- [ ] Severity scoring based on context
