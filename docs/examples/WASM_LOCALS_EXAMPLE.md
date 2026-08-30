# WASM Locals Display - Usage Examples

## Example 1: Simple Integer Locals

### Soroban Smart Contract
```rust
#[contract]
pub struct Counter;

#[contractimpl]
impl Counter {
    pub fn increment(env: Env) -> i32 {
        let mut count: i32 = 0;
        count += 1;
        let max_value: i32 = 1000;
        if count > max_value {
            return -1;
        }
        count
    }
}
```

### Trace Step with WASM Locals
```json
{
    "step": 42,
    "timestamp": "2024-01-15T10:30:00Z",
    "operation": "LocalSet",
    "contract_id": "CA7QMUSQMSQWV4OSURMQNQ7HKOJBKJ5HCMRPCX4KNLMQT5CQVNQWGWHV4",
    "function": "increment",
    "wasm_locals": [
        {
            "name": "count",
            "type": "i32",
            "location": "local[0]",
            "value": 1,
            "startLine": 10,
            "endLine": 18
        },
        {
            "name": "max_value",
            "type": "i32",
            "location": "local[1]",
            "value": 1000,
            "startLine": 11,
            "endLine": 18
        }
    ],
    "wasm_offset": 12345
}
```

### VS Code Display

**Variables Panel:**
```
Locals
  step: 42
  operation: "LocalSet"
  contract_id: "CA7QM..."
  function: "increment"

WASM Locals
  count: i32 = 1
  max_value: i32 = 1000

Arguments
  env: Env
```

**Hover Tooltip:**
When hovering over `count` in the source:
```
count: i32
Value: 1
Location: local[0]
Scope: lines 10-18
```

---

## Example 2: Complex Struct Locals

### Soroban Smart Contract
```rust
#[derive(Clone)]
pub struct Account {
    pub balance: i128,
    pub is_active: bool,
    pub level: u32,
}

#[contract]
pub struct Bank;

#[contractimpl]
impl Bank {
    pub fn update_account(env: Env, account: Account) -> bool {
        let mut acc = account;
        acc.balance += 100;
        acc.level = 2;
        true
    }
}
```

### Trace Step with WASM Locals
```json
{
    "step": 156,
    "timestamp": "2024-01-15T10:30:05Z",
    "operation": "StructSet",
    "contract_id": "CA7QMUSQMSQWV4OSURMQNQ7HKOJBKJ5HCMRPCX4KNLMQT5CQVNQWGWHV4",
    "function": "update_account",
    "wasm_locals": [
        {
            "name": "account",
            "type": "Account",
            "location": "memory[256:320]",
            "value": {
                "balance": 1000,
                "is_active": true,
                "level": 1
            },
            "startLine": 20,
            "endLine": 25
        },
        {
            "name": "acc",
            "type": "Account",
            "location": "memory[320:384]",
            "value": {
                "balance": 1100,
                "is_active": true,
                "level": 2
            },
            "startLine": 21,
            "endLine": 25
        }
    ],
    "wasm_offset": 45678
}
```

### VS Code Display

**Variables Panel - Expanded:**
```
WASM Locals
  account: Account
    ▼ balance: i128 = 1000
    ▼ is_active: bool = true
    ▼ level: u32 = 1
  acc: Account
    ▼ balance: i128 = 1100
    ▼ is_active: bool = true
    ▼ level: u32 = 2
```

**Hover Tooltip:**
When hovering over `acc` in the source:
```
acc: Account
Value: {
  balance: 1100,
  is_active: true,
  level: 2
}
Location: memory[320:384]
Scope: lines 21-25
```

---

## Example 3: Array/Vector Locals

### Soroban Smart Contract
```rust
#[contract]
pub struct DataManager;

#[contractimpl]
impl DataManager {
    pub fn process_values(env: Env) -> i32 {
        let mut values: Vec<i32> = vec![1, 2, 3, 4, 5];
        let sum: i32 = values.iter().sum();
        let count: usize = values.len();
        sum / count as i32
    }
}
```

### Trace Step with WASM Locals
```json
{
    "step": 89,
    "timestamp": "2024-01-15T10:30:10Z",
    "operation": "ArrayAccess",
    "contract_id": "CA7QMUSQMSQWV4OSURMQNQ7HKOJBKJ5HCMRPCX4KNLMQT5CQVNQWGWHV4",
    "function": "process_values",
    "wasm_locals": [
        {
            "name": "values",
            "type": "Vec<i32>",
            "location": "memory[512:552]",
            "value": [1, 2, 3, 4, 5],
            "startLine": 32,
            "endLine": 37
        },
        {
            "name": "sum",
            "type": "i32",
            "location": "local[0]",
            "value": 15,
            "startLine": 33,
            "endLine": 37
        },
        {
            "name": "count",
            "type": "usize",
            "location": "local[1]",
            "value": 5,
            "startLine": 34,
            "endLine": 37
        }
    ],
    "wasm_offset": 67890
}
```

### VS Code Display

**Variables Panel - Expanded:**
```
WASM Locals
  values: Vec<i32> = [5 items]
    ▼ [0]: i32 = 1
    ▼ [1]: i32 = 2
    ▼ [2]: i32 = 3
    ▼ [3]: i32 = 4
    ▼ [4]: i32 = 5
  sum: i32 = 15
  count: usize = 5
```

---

## Example 4: Option and Result Locals

### Soroban Smart Contract
```rust
#[contract]
pub struct SafeOps;

#[contractimpl]
impl SafeOps {
    pub fn safe_divide(a: i32, b: i32) -> Result<i32, String> {
        if b == 0 {
            return Err("Division by zero".to_string());
        }
        let result: Option<i32> = Some(a / b);
        Ok(result.unwrap())
    }
}
```

### Trace Step with WASM Locals
```json
{
    "step": 201,
    "timestamp": "2024-01-15T10:30:15Z",
    "operation": "MatchOption",
    "contract_id": "CA7QMUSQMSQWV4OSURMQNQ7HKOJBKJ5HCMRPCX4KNLMQT5CQVNQWGWHV4",
    "function": "safe_divide",
    "wasm_locals": [
        {
            "name": "a",
            "type": "i32",
            "location": "local[0]",
            "value": 100,
            "startLine": 40,
            "endLine": 48
        },
        {
            "name": "b",
            "type": "i32",
            "location": "local[1]",
            "value": 4,
            "startLine": 40,
            "endLine": 48
        },
        {
            "name": "result",
            "type": "Option<i32>",
            "location": "local[2]",
            "value": {
                "variant": "Some",
                "data": 25
            },
            "startLine": 45,
            "endLine": 48
        }
    ],
    "wasm_offset": 89012
}
```

### VS Code Display

**Variables Panel - Expanded:**
```
WASM Locals
  a: i32 = 100
  b: i32 = 4
  result: Option<i32>
    ▼ variant: "Some"
    ▼ data: i32 = 25
```

---

## Example 5: Error/Trap Scenario

### Trace Step at Error Point
```json
{
    "step": 312,
    "timestamp": "2024-01-15T10:30:20Z",
    "operation": "MemoryTrap",
    "contract_id": "CA7QMUSQMSQWV4OSURMQNQ7HKOJBKJ5HCMRPCX4KNLMQT5CQVNQWGWHV4",
    "function": "risky_operation",
    "error": "Out of bounds memory access at offset 4096",
    "wasm_locals": [
        {
            "name": "index",
            "type": "u32",
            "location": "local[0]",
            "value": 4096,
            "startLine": 50,
            "endLine": 55
        },
        {
            "name": "buffer_size",
            "type": "u32",
            "location": "local[1]",
            "value": 1024,
            "startLine": 51,
            "endLine": 55
        }
    ],
    "wasm_offset": 123456
}
```

### VS Code Display

**Variables Panel:**
```
Locals
  step: 312
  operation: "MemoryTrap"
  error: "Out of bounds memory access at offset 4096"

WASM Locals
  index: u32 = 4096
  buffer_size: u32 = 1024
```

**Error Display:**
The debugger highlights the error and shows locals that caused it, making it easy to identify:
- `index` (4096) exceeds `buffer_size` (1024)

---

## Debugging Workflow

### Step-by-Step Process

1. **Launch Debug Session**
   - Set transaction hash in launch configuration
   - VS Code connects to ERST simulator

2. **View Trace**
   - Use the Step controls to navigate trace steps
   - Each step shows execution state

3. **Inspect WASM Locals**
   - Look at "WASM Locals" scope in Variables panel
   - See local variables and their types

4. **Expand Complex Types**
   - Click expand arrow on structs, arrays, options
   - Inspect nested values

5. **Hover for Quick Info**
   - Hover over variable names in source
   - See inline tooltips with type and value

6. **Identify Issues**
   - Watch values change across steps
   - Spot logic errors or unexpected states
   - Find memory/logic issues at error points

---

## Tips & Tricks

### Filtering Locals
- Large functions may have many locals
- Focus on locals relevant to current step
- Watch panel shows only currently visible scopes

### Memory Inspection
- Locals with `memory[offset:size]` location are in heap
- Array/Vector types show all elements when expanded
- Struct members show individual field values

### Type Information
- Type names match Rust source code
- Generic types show full qualification (e.g., `Vec<u64>`)
- Custom types show struct/enum names

### Performance
- Locals are cached per step
- No performance impact when scope is collapsed
- Expanding large arrays may take a moment

---

## Integration with Other Features

### Combined with Source Mapping
- Hover tooltips include source line ranges
- Links to exact source code location
- Shows variable lifetime in source

### Combined with Performance Profiling
- Locals visible alongside budget information
- Correlate variable values with CPU/memory usage
- Identify expensive operations

### Combined with Breakpoints
- Set breakpoints at specific line numbers
- WASM locals available at each breakpoint
- Useful for condition-based debugging
