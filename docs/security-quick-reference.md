# Security Vulnerability Detection - Quick Reference

## Detected Vulnerabilities

| Vulnerability | Type | Severity | Detection Method |
|--------------|------|----------|------------------|
| Integer Overflow/Underflow | VERIFIED_RISK | HIGH | Log keywords: overflow, underflow, checked_* |
| Authorization Failure | VERIFIED_RISK | HIGH | Event: auth + (fail\|invalid) |
| Contract Panic/Trap | VERIFIED_RISK | HIGH | Event: panic\|trap |
| Large Value Transfer | HEURISTIC_WARNING | HIGH/MEDIUM | XLM > 1M or tokens > 10M |
| Reentrancy Pattern | HEURISTIC_WARNING | MEDIUM | Multiple invocations + state changes |
| Authorization Bypass | HEURISTIC_WARNING | HIGH | Privileged op without auth check |

## CLI Usage

```bash
# Debug a transaction with security analysis
./erst debug <transaction-hash>

# Example output
=== Security Analysis ===
⚠️  VERIFIED SECURITY RISKS: 1
⚡ HEURISTIC WARNINGS: 1

Findings:

1. ⚠️ [VERIFIED_RISK] HIGH - Integer Overflow/Underflow Detected
   Arithmetic operation failed, indicating potential overflow or underflow
   Evidence: checked_add failed: overflow detected

2. ⚡ [HEURISTIC_WARNING] HIGH - Large Value Transfer Detected
   Transfer of 20000000.00 XLM detected. Verify recipient address.
   Evidence: Destination: GBRP...OX2H
```

## Programmatic Usage

```go
import "github.com/dotandev/hintents/internal/security"

// Create detector
detector := security.NewDetector()

// Analyze transaction
findings := detector.Analyze(
    envelopeXdr,    // Base64 encoded transaction envelope
    resultMetaXdr,  // Base64 encoded result meta (optional)
    events,         // Diagnostic events from simulation
    logs,           // Debug logs from simulation
)

// Process findings
for _, finding := range findings {
    switch finding.Type {
    case security.FindingVerifiedRisk:
        // Handle confirmed security issue
        log.Error(finding.Title, "evidence", finding.Evidence)
    case security.FindingHeuristicWarn:
        // Handle potential security concern
        log.Warn(finding.Title, "description", finding.Description)
    }
}
```

## Finding Structure

```go
type Finding struct {
    Type        FindingType  // VERIFIED_RISK or HEURISTIC_WARNING
    Severity    Severity     // HIGH, MEDIUM, LOW, INFO
    Title       string       // Short description
    Description string       // Detailed explanation
    Evidence    string       // Supporting evidence (optional)
}
```

## Interpretation Guide

### VERIFIED_RISK (⚠️)
- **Action Required**: Investigate immediately
- **Confidence**: High - based on concrete evidence
- **Examples**: Overflow errors, auth failures, panics

### HEURISTIC_WARNING (⚡)
- **Action Required**: Review and verify
- **Confidence**: Medium - based on patterns
- **Examples**: Large transfers, missing auth checks

## Testing

```bash
# Run security tests
go test ./internal/security/... -v

# Run with coverage
go test ./internal/security/... -cover

# Run specific test
go test ./internal/security -run TestDetector_FlawedContract -v
```

## Extension

Add custom vulnerability check:

```go
func (d *Detector) checkCustomVulnerability(data interface{}) {
    if vulnerabilityDetected {
        d.addFinding(Finding{
            Type:        FindingVerifiedRisk,
            Severity:    SeverityHigh,
            Title:       "Custom Vulnerability",
            Description: "Detailed description",
            Evidence:    "Supporting evidence",
        })
    }
}
```

Then call from `Analyze()` method.

## Limitations

- Pattern-based detection (may have false positives/negatives)
- Fixed thresholds for large value detection
- No static code analysis
- Requires simulation logs and events

## Best Practices

1. Always investigate VERIFIED_RISK findings
2. Review HEURISTIC_WARNING findings in context
3. Combine with other security tools
4. Test with known vulnerable contracts
5. Adjust thresholds for your use case
