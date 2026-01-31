# Hintents Debugging Cookbook

This cookbook provides practical recipes for debugging common Soroban smart contract errors using Hintents. It is designed to help you move from an opaque error code to a clear understanding of the root cause by teaching you how to read execution traces and diagnostic events.

## Table of Contents

1. [Auth Failure: Missing Signature](#1-auth-failure-missing-signature)
2. [Storage Footprint Mismatch](#2-storage-footprint-mismatch)
3. [State Not Initialized](#3-state-not-initialized)
4. [WASM Trap: Arithmetic Overflow](#4-wasm-trap-arithmetic-overflow)
5. [WASM Trap: Unwrap on None](#5-wasm-trap-unwrap-on-none)

---

## 1. Auth Failure: Missing Signature

### Error Title
**Auth Failure: Missing `require_auth` or Invalid Signature**

### What the Error Looks Like
In a standard Soroban client, you might see a generic error:
```
Error: HostError: Error(Context, InvalidAction)
```

In **Hintents**, the error is decoded, and the trace will show a failure during the authorization check:
```
Event: Diagnostic
Topics: ["fn_call", "contract:123...", "transfer"]
Status: Failed
Error: Host Trap: HostError: Error(Context, InvalidAction)
```

### How to Reproduce

**Contract Code (Rust):**
```rust
pub fn transfer(env: Env, from: Address, to: Address, amount: i128) {
    // BUG: Missing from.require_auth();
    // Anyone can call this and move funds!
    
    // If the token contract requires auth for 'from', this cross-contract call 
    // will fail if 'from' hasn't authorized the top-level invocation.
    token::Client::new(&env, &token_id).transfer(&from, &to, &amount);
}
```

**Invocation:**
```bash
# Invoking without signing as 'from'
erst debug --wasm ./contract.wasm --args "transfer" --args "UserA" --args "UserB" --args 100
```

### Why It Happens
Soroban's auth framework requires that any address whose state is being modified (or who is approving an action) must explicitly authorize the invocation. If your contract calls another contract (like a token transfer) that calls `require_auth`, the top-level invoker must have signed the transaction, and the authorization tree must match.

### How to Read the Trace
1.  **Look for `Diagnostic` events**: Hintents captures host events. Look for events preceding the failure.
2.  **Check the Auth Stack**: In the trace explorer, check if there is an `Authorize` entry.
3.  **Identify the Blocker**: If the error is `InvalidAction` in the context of a cross-contract call (like `transfer`), it usually means the sub-invocation failed verification.

### How to Fix It
Add the required authorization check in your contract:

```rust
pub fn transfer(env: Env, from: Address, to: Address, amount: i128) {
    // FIX: Require authorization from the sender
    from.require_auth();
    
    token::Client::new(&env, &token_id).transfer(&from, &to, &amount);
}
```
Or ensure the transaction is signed by `from` when invoking.

---

## 2. Storage Footprint Mismatch

### Error Title
**Storage Footprint Mismatch / Missing Entry**

### What the Error Looks Like
```
Error: HostError: Error(Storage, MissingEntry)
```
Or if writing to a key not in the footprint:
```
Error: HostError: Error(Storage, ExceededLimit)
```

### How to Reproduce

**Contract Code:**
```rust
#[contracttype]
pub enum DataKey {
    Counter,
}

pub fn increment(env: Env) -> u32 {
    let mut count: u32 = env.storage().instance().get(&DataKey::Counter).unwrap_or(0);
    count += 1;
    env.storage().instance().set(&DataKey::Counter, &count); // Write
    count
}
```

**Invocation:**
If you construct a transaction manually (or use `erst` with a manual footprint) and omit `DataKey::Counter` from the read/write set.

### Why It Happens
Soroban requires a **Storage Footprint** for every transaction. This footprint declares every storage key the contract will read or write. If the contract attempts to access a key that wasn't declared in the transaction's footprint, the host halts execution immediately.

### How to Read the Trace
1.  **Hintents Log**: Look for "Loaded X Ledger Entries". If the count is 0 or low, your footprint might be empty.
2.  **Error Location**: The trace will point to the exact line where `env.storage().get()` or `.set()` was called.
3.  **Diagnostic Events**: Hintents often captures the specific key that failed. Look for a `Diagnostic` event with the key hash right before the crash.

### How to Fix It
Ensure your transaction simulation includes the necessary keys.
*   **In Development**: Use `soroban-cli` or `erst`'s simulation mode which automatically builds the footprint.
*   **In Production**: Always simulate the transaction (`simulateTransaction` RPC endpoint) before submitting it to the network to generate the correct footprint.

---

## 3. State Not Initialized

### Error Title
**State Not Initialized (Reading Missing Data)**

### What the Error Looks Like
```
Error: VM Trap: Unreachable Instruction (Panic or invalid code path)
```
*Note: This often manifests as a panic (unwrap on None) if you blindly unwrap the result of a `get`.*

If you handle the error, you might see:
```
Error: HostError: Error(Storage, MissingEntry)
```

### How to Reproduce

**Contract Code:**
```rust
pub fn get_admin(env: Env) -> Address {
    // BUG: Assuming 'Admin' key exists. If contract wasn't initialized, this panics.
    env.storage().instance().get(&DataKey::Admin).unwrap()
}
```

**Invocation:**
Calling `get_admin` immediately after deploying the WASM, without calling an `init` function first.

### Why It Happens
Smart contracts do not have a "constructor" that runs automatically on upload. You must explicitly initialize state. Accessing a key that hasn't been written yet returns `None`. Calling `.unwrap()` on that `None` causes a panic (Trap).

### How to Read the Trace
1.  **Identify the Panic**: Hintents decodes the error as **"VM Trap: Unreachable Instruction"**.
2.  **Source Mapping**: If you compiled with debug symbols, Hintents will point to the exact line number:
    ```
    Failed at line 15 in src/lib.rs
    env.storage().instance().get(&DataKey::Admin).unwrap()
    ```
3.  **Trace Flow**: You will see the `get` host function call return, followed immediately by the trap.

### How to Fix It
1.  **Add an Init Function**:
    ```rust
    pub fn init(env: Env, admin: Address) {
        // Check if already initialized to prevent takeover
        if env.storage().instance().has(&DataKey::Admin) {
            panic!("Already initialized");
        }
        env.storage().instance().set(&DataKey::Admin, &admin);
    }
    ```
2.  **Safely Handle Missing Data**:
    ```rust
    pub fn get_admin(env: Env) -> Option<Address> {
        env.storage().instance().get(&DataKey::Admin)
    }
    ```

---

## 4. WASM Trap: Arithmetic Overflow

### Error Title
**WASM Trap: Integer Overflow**

### What the Error Looks Like
```
Error: VM Trap: Integer Overflow
```

### How to Reproduce

**Contract Code:**
```rust
pub fn add_unsafe(env: Env, a: u64, b: u64) -> u64 {
    // BUG: Rust panics on overflow in debug mode (and Soroban enables this check)
    a + b
}
```

**Invocation:**
```bash
erst debug --wasm ./contract.wasm --args "add_unsafe" --args 18446744073709551615 --args 1
```

### Why It Happens
In Rust (and WASM), standard arithmetic operators (`+`, `-`, `*`) can panic if the result exceeds the maximum value for the type (e.g., `u64::MAX`). Soroban contracts run with overflow checks enabled for safety.

### How to Read the Trace
1.  **Decoded Error**: Hintents specifically parses the trap code and displays **"VM Trap: Integer Overflow"**.
2.  **Instruction Pointer**: The trace will stop at the arithmetic instruction.
3.  **Source Map**: Hintents links this directly to the `a + b` line in your Rust code.

### How to Fix It
Use explicit checked or saturating arithmetic if overflow is possible:

```rust
pub fn add_safe(env: Env, a: u64, b: u64) -> u64 {
    a.checked_add(b).unwrap_or(u64::MAX) 
    // OR panic with a custom error
    // a.checked_add(b).expect("Overflow detected")
}
```

---

## 5. WASM Trap: Unwrap on None

### Error Title
**WASM Trap: Unwrap on None (Unreachable)**

### What the Error Looks Like
```
Error: VM Trap: Unreachable Instruction (Panic or invalid code path)
```

### How to Reproduce

**Contract Code:**
```rust
pub fn get_balance(env: Env, user: Address) -> i128 {
    // BUG: If 'user' has no balance, get() returns None. unwrap() panics.
    let balance: i128 = env.storage().persistent().get(&user).unwrap(); 
    balance
}
```

### Why It Happens
The `.unwrap()` method on an `Option` or `Result` tells the program to panic if the value is `None` or `Err`. In WASM, a panic compiles down to an `unreachable` instruction, which causes the VM to abort immediately.

### How to Read the Trace
1.  **Ambiguous Error**: "Unreachable" can mean many things (panic, assertion failed, index out of bounds).
2.  **Context Clues**: Look at the function call immediately preceding the trap in the Hintents trace.
    *   Did you just call `map.get()`?
    *   Did you just call `vec.get()`?
    *   Did you just call a sub-contract?
3.  **Hintents Insight**: If the previous host function call (e.g., `map_get`) succeeded but returned a "void" or "absent" value, and *then* the trap occurred, it is almost certainly a failed `unwrap()`.

### How to Fix It
Always handle potential absence of data:

```rust
pub fn get_balance(env: Env, user: Address) -> i128 {
    // FIX: Provide a default or handle the error gracefully
    env.storage().persistent().get(&user).unwrap_or(0)
}
```
