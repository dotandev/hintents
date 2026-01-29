# OpenTelemetry Integration

Erst supports exporting distributed traces to external observability platforms via OpenTelemetry.

## Quick Start

1. Start a local Jaeger instance:
```bash
docker-compose -f docker-compose.jaeger.yml up -d
```

2. Run erst with tracing enabled:
```bash
./erst debug --tracing --otlp-url http://localhost:4318 <transaction-hash>
```

3. View traces in Jaeger UI at http://localhost:16686

## Configuration

### CLI Flags

- `--tracing`: Enable OpenTelemetry tracing (default: false)
- `--otlp-url`: OTLP exporter endpoint URL (default: http://localhost:4318)

### Spans Generated

The integration creates the following span hierarchy:

```
debug_transaction
├── fetch_transaction (RPC call to Horizon)
└── simulate_transaction (if simulation is run)
    ├── marshal_request
    ├── execute_simulator
    └── unmarshal_response
```

### Span Attributes

Each span includes relevant attributes:

- **debug_transaction**: `transaction.hash`, `network`
- **fetch_transaction**: `transaction.hash`, `network`, `envelope.size_bytes`
- **simulate_transaction**: `simulator.binary_path`, `request.size_bytes`, `response.stdout_size`

## Supported Platforms

The OTLP HTTP exporter is compatible with:

- Jaeger
- Honeycomb
- Datadog
- New Relic
- Any OTLP-compatible observability platform

## Performance

When tracing is disabled (default), there is zero performance overhead. When enabled, the overhead is minimal due to:

- Efficient span batching
- Asynchronous export
- Minimal attribute collection

## Example Usage

```bash
# Debug with Jaeger
./erst debug --tracing 5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab

# Debug with Honeycomb
./erst debug --tracing --otlp-url https://api.honeycomb.io/v1/traces <tx-hash>

# Debug with custom OTLP endpoint
./erst debug --tracing --otlp-url http://my-collector:4318 <tx-hash>
```
