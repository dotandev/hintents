# TypeScript Bindings Generator

This package provides automatic TypeScript client generation for Soroban smart contracts with built-in erst simulation support.

## Overview

The bindings generator extracts contract specifications from compiled WASM files and generates strongly-typed TypeScript clients that provide:

- **Type Safety**: Full TypeScript types for all contract methods and data structures
- **Erst Integration**: Built-in simulation and debugging capabilities
- **Developer Experience**: IntelliSense, autocomplete, and compile-time validation
- **Production Ready**: Complete NPM packages with documentation

## Architecture

### Components

1. **Generator** (`generator.go`)
   - Main code generation logic
   - Type mapping from Soroban to TypeScript
   - File generation and writing

2. **ABI Integration** (`internal/abi/`)
   - WASM custom section extraction
   - Contract spec decoding
   - Type definition parsing

3. **CLI Command** (`cmd/generate-bindings.go`)
   - Command-line interface
   - Argument parsing
   - File I/O coordination

### Generated Files

```
output-directory/
├── types.ts              # Type definitions
├── client.ts             # Client class
├── erst-integration.ts   # Simulator
├── index.ts              # Exports
├── package.json          # NPM config
└── README.md             # Documentation
```

## Type Mapping

| Soroban Type | TypeScript Type | Notes |
|--------------|-----------------|-------|
| `Bool` | `boolean` | |
| `U32`, `I32` | `number` | 32-bit integers |
| `U64`, `I64` | `bigint` | 64-bit integers |
| `U128`, `I128` | `bigint` | 128-bit integers |
| `U256`, `I256` | `bigint` | 256-bit integers |
| `String` | `string` | |
| `Symbol` | `string` | Contract symbols |
| `Address` | `string` | Stellar addresses |
| `Bytes` | `Uint8Array` | Byte arrays |
| `BytesN(N)` | `Uint8Array` | Fixed-size byte arrays |
| `Timepoint` | `Date` | Unix timestamps |
| `Duration` | `number` | Time durations |
| `Option<T>` | `T \| null` | Optional values |
| `Vec<T>` | `T[]` | Arrays |
| `Map<K, V>` | `Map<K, V>` | Maps |
| `Tuple<T1, T2>` | `[T1, T2]` | Tuples |
| `Result<T, E>` | `Result<T, E>` | Result type |
| Custom structs | `interface` | Struct definitions |
| Custom enums | `enum` | Enum definitions |
| Custom unions | Discriminated union | Union types |
| Error enums | `enum` + Error class | Error handling |

## Code Generation Process

### 1. Extract Contract Spec

```go
// Extract custom section from WASM
specBytes, err := abi.ExtractCustomSection(wasmBytes, "contractspecv0")

// Decode XDR-encoded spec
spec, err := abi.DecodeContractSpec(specBytes)
```

### 2. Generate Types

```go
func (g *Generator) generateTypes() string {
    // Generate base types
    // Generate struct interfaces
    // Generate enum types
    // Generate union types
    // Generate error types
    // Generate event types
}
```

### 3. Generate Client

```go
func (g *Generator) generateClient() string {
    // Generate client configuration
    // Generate client class
    // Generate contract methods
    // Generate helper methods
}
```

### 4. Generate Erst Integration

```go
func (g *Generator) generateErstIntegration() string {
    // Generate simulator class
    // Generate simulation methods
    // Generate debug methods
}
```

## Usage Example

### Input: Soroban Contract

```rust
#[contract]
pub struct TokenContract;

#[contractimpl]
impl TokenContract {
    pub fn transfer(env: Env, from: Address, to: Address, amount: i128) {
        // Implementation
    }
}
```

### Output: TypeScript Client

```typescript
export class TokenContractClient {
  async transfer(
    source: StellarSdk.Keypair,
    from: Address,
    to: Address,
    amount: bigint,
    options?: CallOptions
  ): Promise<CallResult<void>> {
    // Generated implementation
  }
}
```

### Frontend Usage

```typescript
const client = new TokenContractClient({
  contractId: 'CDLZFC...',
  network: 'testnet',
  enableSimulation: true,
});

const result = await client.transfer(
  keypair,
  'FROM_ADDRESS',
  'TO_ADDRESS',
  BigInt(1000000),
  { simulate: true }
);
```

## Testing

### Unit Tests

```go
func TestMapTypeDefToTS(t *testing.T) {
    g := &Generator{}
    
    tests := []struct {
        name     string
        typeDef  xdr.ScSpecTypeDef
        expected string
    }{
        {"Bool", xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeBool}, "boolean"},
        {"U64", xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeU64}, "bigint"},
    }
    
    for _, tt := range tests {
        result := g.mapTypeDefToTS(tt.typeDef)
        if result != tt.expected {
            t.Errorf("got %v, want %v", result, tt.expected)
        }
    }
}
```

### Integration Tests

```go
func TestGenerateBindingsEndToEnd(t *testing.T) {
    wasmBytes, _ := os.ReadFile("test.wasm")
    
    config := GeneratorConfig{
        WasmBytes:   wasmBytes,
        OutputDir:   t.TempDir(),
        PackageName: "test-contract",
    }
    
    generator := NewGenerator(config)
    files, err := generator.Generate()
    
    // Verify all files generated
    // Verify file contents
}
```

## Extension Points

### Custom Type Mappers

```go
// Add custom type mapping
func (g *Generator) mapCustomType(td xdr.ScSpecTypeDef) string {
    // Custom mapping logic
}
```

### Custom Templates

```go
// Override template generation
func (g *Generator) generateCustomTemplate() string {
    // Custom template logic
}
```

### Post-Processing

```go
// Add post-processing hook
func (g *Generator) postProcess(files []GeneratedFile) []GeneratedFile {
    // Post-processing logic
}
```

## Performance

### Generation Time

- Small contracts (<10 functions): <100ms
- Medium contracts (10-50 functions): <500ms
- Large contracts (>50 functions): <2s

### Memory Usage

- Typical: <50MB
- Large contracts: <200MB

### Optimization Strategies

1. **Single-pass parsing**: Parse spec once
2. **String builder**: Efficient string concatenation
3. **Lazy evaluation**: Generate only requested files
4. **Caching**: Cache parsed specs

## Error Handling

### Generation Errors

```go
// WASM file errors
if err := validateWasm(wasmBytes); err != nil {
    return fmt.Errorf("invalid WASM: %w", err)
}

// Spec extraction errors
if specBytes == nil {
    return fmt.Errorf("contract spec not found")
}

// Type mapping errors
if !isValidType(typeDef) {
    return fmt.Errorf("unsupported type: %v", typeDef)
}
```

### Runtime Errors

```typescript
// Client errors
try {
  await client.transfer(...);
} catch (error) {
  if (error instanceof ContractErrorError) {
    // Handle contract error
  } else {
    // Handle network error
  }
}
```

## Best Practices

### 1. Version Control

Commit generated bindings to ensure consistency:

```bash
erst generate-bindings contract.wasm --output ./src/bindings
git add src/bindings
git commit -m "Update contract bindings"
```

### 2. Regeneration

Always regenerate when contract changes:

```bash
# After contract update
soroban contract build
erst generate-bindings target/wasm32-unknown-unknown/release/contract.wasm
```

### 3. Testing

Test generated bindings thoroughly:

```typescript
describe('Generated Client', () => {
  it('should have correct types', () => {
    const client = new MyContractClient({...});
    // Type assertions
  });
});
```

### 4. Documentation

Document custom types and methods:

```typescript
/**
 * Transfer tokens between addresses
 * @param from Source address
 * @param to Destination address
 * @param amount Amount to transfer
 */
async transfer(from: Address, to: Address, amount: bigint)
```

## Troubleshooting

### Contract Spec Not Found

**Problem**: `contract spec not found in WASM file`

**Solution**: Ensure contract is built with spec:
```bash
soroban contract build
```

### Type Mapping Errors

**Problem**: `unsupported type: ...`

**Solution**: Check if type is supported or add custom mapper

### Generation Failures

**Problem**: Generation fails with error

**Solution**: 
1. Verify WASM file is valid
2. Check erst version
3. Review error message
4. File issue with details

## Contributing

### Adding New Type Mappings

1. Add mapping in `mapTypeDefToTS()`
2. Add test case
3. Update documentation
4. Submit PR

### Adding New Features

1. Discuss in issue first
2. Implement with tests
3. Update documentation
4. Submit PR

## License

Apache-2.0
