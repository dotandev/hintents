# Compiling Soroban Contracts with Debug Symbols

This guide shows how to compile Soroban contracts with debug symbols for use with `erst`'s source mapping feature.

## Prerequisites

- Rust toolchain
- Soroban CLI
- A Soroban contract project

## Compilation Steps

### 1. Enable Debug Symbols in Cargo.toml

Add or modify the profile settings in your contract's `Cargo.toml`:

```toml
[profile.release]
debug = true
debug-assertions = false
overflow-checks = false
lto = false
panic = "abort"
codegen-units = 1
opt-level = "z"
```

### 2. Build with Debug Information

```bash
# Build the contract with debug symbols
cargo build --target wasm32-unknown-unknown --release

# Or use soroban CLI
soroban contract build
```

### 3. Verify Debug Symbols

Check if your WASM contains debug symbols:

```bash
# Install wasm-objdump if not already installed
cargo install wasmprinter

# Check for debug sections
wasm-objdump -h target/wasm32-unknown-unknown/release/your_contract.wasm | grep debug
```

You should see sections like:
- `.debug_info`
- `.debug_line`
- `.debug_str`
- `.debug_abbrev`

### 4. Extract WASM for erst

```bash
# Get the WASM file
cp target/wasm32-unknown-unknown/release/your_contract.wasm contract.wasm

# Convert to base64 for erst
base64 -w 0 contract.wasm > contract.wasm.b64
```

## Example Contract

Here's a simple contract that demonstrates source mapping:

```rust
#![no_std]
use soroban_sdk::{contract, contractimpl, Env, Symbol, symbol_short};

#[contract]
pub struct TokenContract;

#[contractimpl]
impl TokenContract {
    pub fn transfer(env: Env, from: Symbol, to: Symbol, amount: i128) -> Result<(), Symbol> {
        // Line 12: Check balance
        let balance = Self::get_balance(&env, &from);
        
        // Line 15: This line might fail - insufficient balance
        if balance < amount {
            return Err(symbol_short!("insufficient"));
        }
        
        // Line 20: Update balances
        Self::set_balance(&env, &from, balance - amount);
        let to_balance = Self::get_balance(&env, &to);
        Self::set_balance(&env, &to, to_balance + amount);
        
        Ok(())
    }
    
    fn get_balance(env: &Env, account: &Symbol) -> i128 {
        env.storage().instance().get(account).unwrap_or(0)
    }
    
    fn set_balance(env: &Env, account: &Symbol, balance: i128) {
        env.storage().instance().set(account, &balance);
    }
}
```

## Testing with erst

Once you have a contract with debug symbols:

1. Deploy and execute the contract on Stellar
2. If it fails, get the transaction hash
3. Use `erst debug <transaction-hash>` with the WASM file

The output will show:
```
Failed at line 15 in src/lib.rs
```

## Troubleshooting

### No Debug Symbols Found

If `erst` reports no debug symbols:

1. Verify debug symbols are present: `wasm-objdump -h contract.wasm | grep debug`
2. Check Cargo.toml profile settings
3. Ensure you're using the release build with debug=true

### Large WASM Files

Debug symbols significantly increase WASM size:
- Without debug: ~50KB
- With debug: ~500KB+

This is normal and expected for development builds.

### Optimization Issues

Some optimizations can make source mapping less accurate:
- Set `lto = false` for better mapping
- Use `opt-level = "z"` instead of "s" for size optimization
- Consider `codegen-units = 1` for better debug info
