// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

use serde::Deserialize;
use std::collections::{BTreeMap, HashMap};
use std::fs;
use std::io::{self, Write};
use std::path::{Path, PathBuf};
use wasmparser::{ExternalKind, Operator, Parser, Payload, TypeRef, ValType};

pub fn run_repl(wasm_path: &Path, initial_function: Option<&str>) -> Result<(), String> {
    let bytes = fs::read(wasm_path).map_err(|error| {
        format!(
            "failed to read wasm file '{}': {}",
            wasm_path.display(),
            error
        )
    })?;
    let module = ModuleDebugInfo::from_bytes(&bytes)?;
    let source_mapper = SourceMapper::load(wasm_path)?;
    let mut session = DebuggerSession::new(module, source_mapper)?;

    if let Some(selection) = initial_function {
        session.select_function(selection)?;
    }

    println!("Loaded {}", wasm_path.display());
    println!("Selected {}", session.current_function_label());
    println!("Type 'help' for commands.");

    let stdin = io::stdin();
    loop {
        print!("erst-sim> ");
        io::stdout()
            .flush()
            .map_err(|error| format!("failed to flush stdout: {}", error))?;

        let mut line = String::new();
        let bytes_read = stdin
            .read_line(&mut line)
            .map_err(|error| format!("failed to read command: {}", error))?;
        if bytes_read == 0 {
            println!();
            break;
        }

        match session.handle_command(line.trim())? {
            CommandOutcome::Continue(message) => {
                if !message.is_empty() {
                    println!("{message}");
                }
            }
            CommandOutcome::Exit(message) => {
                if !message.is_empty() {
                    println!("{message}");
                }
                break;
            }
        }
    }

    Ok(())
}

#[derive(Debug)]
enum CommandOutcome {
    Continue(String),
    Exit(String),
}

#[derive(Debug, Clone)]
struct DebuggerSession {
    module: ModuleDebugInfo,
    source_mapper: SourceMapper,
    current_function: usize,
    current_instruction: usize,
    stack: Vec<String>,
}

impl DebuggerSession {
    fn new(module: ModuleDebugInfo, source_mapper: SourceMapper) -> Result<Self, String> {
        let current_function = module
            .default_function_index()
            .ok_or_else(|| "wasm module does not contain any defined functions".to_string())?;
        Ok(Self {
            module,
            source_mapper,
            current_function,
            current_instruction: 0,
            stack: Vec::new(),
        })
    }

    fn handle_command(&mut self, command: &str) -> Result<CommandOutcome, String> {
        if command.is_empty() {
            return Ok(CommandOutcome::Continue(String::new()));
        }

        let mut parts = command.split_whitespace();
        let verb = parts.next().unwrap_or_default();

        match verb {
            "help" => Ok(CommandOutcome::Continue(self.help_text())),
            "list-funcs" => Ok(CommandOutcome::Continue(self.list_functions())),
            "stack" => Ok(CommandOutcome::Continue(format!(
                "stack {}",
                Self::format_stack(&self.stack)
            ))),
            "where" => Ok(CommandOutcome::Continue(self.current_position())),
            "reset" => {
                self.current_instruction = 0;
                self.stack.clear();
                Ok(CommandOutcome::Continue(format!(
                    "Reset {}",
                    self.current_function_label()
                )))
            }
            "use" => {
                let selection = parts.collect::<Vec<_>>().join(" ");
                if selection.is_empty() {
                    return Err("use requires a function name or numeric index".to_string());
                }
                self.select_function(&selection)?;
                Ok(CommandOutcome::Continue(format!(
                    "Selected {}",
                    self.current_function_label()
                )))
            }
            "step-inst" | "si" => Ok(CommandOutcome::Continue(self.step_instruction()?)),
            "quit" | "exit" => Ok(CommandOutcome::Exit("bye".to_string())),
            other => Err(format!("unknown command '{}'", other)),
        }
    }

    fn help_text(&self) -> String {
        [
            "Commands:",
            "  help               Show this help text",
            "  list-funcs         List debuggable functions",
            "  use <name|index>   Select a function by export/name or function index",
            "  where              Show the current function and instruction pointer",
            "  stack              Show the current symbolic stack",
            "  reset              Reset the current function to the first instruction",
            "  step-inst | si     Execute one instruction and print opcode, stack, and source info",
            "  quit | exit        Leave the REPL",
        ]
        .join("\n")
    }

    fn list_functions(&self) -> String {
        self.module
            .functions
            .iter()
            .map(|function| {
                format!(
                    "{}: {} ({} instructions)",
                    function.index,
                    function.label,
                    function.instructions.len()
                )
            })
            .collect::<Vec<_>>()
            .join("\n")
    }

    fn current_function(&self) -> &FunctionInfo {
        &self.module.functions[self.current_function]
    }

    fn current_function_label(&self) -> String {
        let function = self.current_function();
        format!("{} [{}]", function.label, function.index)
    }

    fn current_position(&self) -> String {
        let function = self.current_function();
        if let Some(step) = function.instructions.get(self.current_instruction) {
            format!(
                "{} @ instruction {} (offset 0x{:x})",
                function.label, self.current_instruction, step.offset
            )
        } else {
            format!(
                "{} @ end-of-function ({} instructions)",
                function.label,
                function.instructions.len()
            )
        }
    }

    fn select_function(&mut self, selection: &str) -> Result<(), String> {
        let function_idx = if let Ok(index) = selection.parse::<u32>() {
            self.module
                .functions
                .iter()
                .position(|function| function.index == index)
                .ok_or_else(|| format!("function index {} not found", index))?
        } else {
            *self
                .module
                .function_lookup
                .get(selection)
                .ok_or_else(|| format!("function '{}' not found", selection))?
        };

        self.current_function = function_idx;
        self.current_instruction = 0;
        self.stack.clear();
        Ok(())
    }

    fn step_instruction(&mut self) -> Result<String, String> {
        let function = self.current_function();
        let step = function
            .instructions
            .get(self.current_instruction)
            .cloned()
            .ok_or_else(|| format!("{} is already complete", function.label))?;

        let action_summary = step.action.apply(&mut self.stack);
        self.current_instruction += 1;

        let mut lines = vec![format!(
            "{} @ 0x{:x}: {}",
            function.label, step.offset, step.opcode
        )];
        if !action_summary.is_empty() {
            lines.push(format!("effect {}", action_summary));
        }
        lines.push(format!("stack {}", Self::format_stack(&self.stack)));

        if let Some(location) = self.source_mapper.lookup(step.offset) {
            let mut source_line = format!("source {}:{}", location.file, location.line);
            if let Some(column) = location.column {
                source_line.push_str(&format!(":{}", column));
            }
            lines.push(source_line);
            if let Some(snippet) = &location.source {
                lines.push(format!("code   {}", snippet));
            }
        } else {
            lines.push("source <unavailable>".to_string());
        }

        Ok(lines.join("\n"))
    }

    fn format_stack(stack: &[String]) -> String {
        if stack.is_empty() {
            "[]".to_string()
        } else {
            format!("[{}]", stack.join(", "))
        }
    }
}

#[derive(Debug, Clone)]
struct ModuleDebugInfo {
    functions: Vec<FunctionInfo>,
    function_lookup: HashMap<String, usize>,
}

impl ModuleDebugInfo {
    fn from_bytes(bytes: &[u8]) -> Result<Self, String> {
        let mut types = Vec::new();
        let mut function_type_indices = Vec::new();
        let mut import_function_type_indices = Vec::new();
        let mut export_names: HashMap<u32, String> = HashMap::new();
        let mut bodies = Vec::new();

        for payload in Parser::new(0).parse_all(bytes) {
            let payload =
                payload.map_err(|error| format!("failed to parse wasm module: {}", error))?;
            match payload {
                Payload::TypeSection(reader) => {
                    for ty in reader.into_iter_err_on_gc_types() {
                        let ty = ty.map_err(|error| {
                            format!("failed to parse wasm type section: {}", error)
                        })?;
                        types.push(FunctionSignature {
                            params: ty.params().iter().copied().collect(),
                            results: ty.results().iter().copied().collect(),
                        });
                    }
                }
                Payload::ImportSection(reader) => {
                    for import in reader {
                        let import = import.map_err(|error| {
                            format!("failed to parse wasm import section: {}", error)
                        })?;
                        if let TypeRef::Func(type_index) = import.ty {
                            import_function_type_indices.push(type_index);
                        }
                    }
                }
                Payload::FunctionSection(reader) => {
                    for type_index in reader {
                        function_type_indices.push(type_index.map_err(|error| {
                            format!("failed to parse wasm function section: {}", error)
                        })?);
                    }
                }
                Payload::ExportSection(reader) => {
                    for export in reader {
                        let export = export.map_err(|error| {
                            format!("failed to parse wasm export section: {}", error)
                        })?;
                        if export.kind == ExternalKind::Func {
                            export_names.insert(export.index, export.name.to_string());
                        }
                    }
                }
                Payload::CodeSectionEntry(body) => {
                    bodies.push(parse_function_body(
                        &body,
                        &types,
                        &import_function_type_indices,
                        &function_type_indices,
                        &export_names,
                    )?);
                }
                _ => {}
            }
        }

        let imported_count = import_function_type_indices.len() as u32;
        let mut functions = Vec::new();
        let mut function_lookup = HashMap::new();

        for (position, instructions) in bodies.into_iter().enumerate() {
            let function_index = imported_count + position as u32;
            let label = export_names
                .get(&function_index)
                .cloned()
                .unwrap_or_else(|| format!("func[{function_index}]"));

            function_lookup.insert(label.clone(), functions.len());
            function_lookup.insert(function_index.to_string(), functions.len());

            functions.push(FunctionInfo {
                index: function_index,
                label,
                instructions,
            });
        }

        Ok(Self {
            functions,
            function_lookup,
        })
    }

    fn default_function_index(&self) -> Option<usize> {
        self.functions
            .iter()
            .position(|function| !function.instructions.is_empty())
            .or_else(|| (!self.functions.is_empty()).then_some(0))
    }
}

#[derive(Debug, Clone)]
struct FunctionInfo {
    index: u32,
    label: String,
    instructions: Vec<InstructionStep>,
}

#[derive(Debug, Clone)]
struct InstructionStep {
    offset: u64,
    opcode: String,
    action: StepAction,
}

#[derive(Debug, Clone)]
enum StepAction {
    None,
    PushLiteral(String),
    LocalGet(u32),
    LocalSet(u32),
    LocalTee(u32),
    GlobalGet(u32),
    GlobalSet(u32),
    Unary(String),
    Binary(String),
    Ternary(String),
    Pop(usize),
    Call {
        label: String,
        params: usize,
        results: usize,
        consume_callee: bool,
    },
    Return,
}

impl StepAction {
    fn apply(&self, stack: &mut Vec<String>) -> String {
        match self {
            StepAction::None => String::new(),
            StepAction::PushLiteral(value) => {
                stack.push(value.clone());
                format!("push {}", value)
            }
            StepAction::LocalGet(index) => {
                let value = format!("local[{index}]");
                stack.push(value.clone());
                format!("push {}", value)
            }
            StepAction::LocalSet(index) => {
                let value = pop_or_placeholder(stack);
                format!("local[{index}] = {}", value)
            }
            StepAction::LocalTee(index) => {
                let value = pop_or_placeholder(stack);
                stack.push(value.clone());
                format!("local[{index}] = {}", value)
            }
            StepAction::GlobalGet(index) => {
                let value = format!("global[{index}]");
                stack.push(value.clone());
                format!("push {}", value)
            }
            StepAction::GlobalSet(index) => {
                let value = pop_or_placeholder(stack);
                format!("global[{index}] = {}", value)
            }
            StepAction::Unary(label) => {
                let value = pop_or_placeholder(stack);
                let rendered = format!("{label}({value})");
                stack.push(rendered.clone());
                rendered
            }
            StepAction::Binary(label) => {
                let rhs = pop_or_placeholder(stack);
                let lhs = pop_or_placeholder(stack);
                let rendered = format!("{label}({lhs}, {rhs})");
                stack.push(rendered.clone());
                rendered
            }
            StepAction::Ternary(label) => {
                let third = pop_or_placeholder(stack);
                let second = pop_or_placeholder(stack);
                let first = pop_or_placeholder(stack);
                let rendered = format!("{label}({first}, {second}, {third})");
                stack.push(rendered.clone());
                rendered
            }
            StepAction::Pop(count) => {
                let mut popped = Vec::new();
                for _ in 0..*count {
                    popped.push(pop_or_placeholder(stack));
                }
                format!("pop {}", popped.join(", "))
            }
            StepAction::Call {
                label,
                params,
                results,
                consume_callee,
            } => {
                let total_pops = params + usize::from(*consume_callee);
                let mut args = Vec::new();
                for _ in 0..total_pops {
                    args.push(pop_or_placeholder(stack));
                }
                args.reverse();
                for result_index in 0..*results {
                    stack.push(format!("{label}#result[{result_index}]"));
                }
                format!("call {}({})", label, args.join(", "))
            }
            StepAction::Return => "return".to_string(),
        }
    }
}

fn pop_or_placeholder(stack: &mut Vec<String>) -> String {
    stack.pop().unwrap_or_else(|| "<empty>".to_string())
}

#[derive(Debug, Clone)]
struct FunctionSignature {
    params: Vec<ValType>,
    results: Vec<ValType>,
}

fn parse_function_body(
    body: &wasmparser::FunctionBody<'_>,
    types: &[FunctionSignature],
    imported_function_type_indices: &[u32],
    function_type_indices: &[u32],
    export_names: &HashMap<u32, String>,
) -> Result<Vec<InstructionStep>, String> {
    let all_function_type_indices = imported_function_type_indices
        .iter()
        .chain(function_type_indices.iter())
        .copied()
        .collect::<Vec<_>>();

    let mut reader = body
        .get_operators_reader()
        .map_err(|error| format!("failed to read wasm function body: {}", error))?;
    let mut instructions = Vec::new();

    while !reader.eof() {
        let offset = reader.original_position() as u64;
        let operator = reader
            .read()
            .map_err(|error| format!("failed to parse wasm instruction: {}", error))?;
        instructions.push(InstructionStep {
            offset,
            opcode: format!("{operator:?}"),
            action: step_action_for_operator(
                &operator,
                types,
                &all_function_type_indices,
                export_names,
            ),
        });
    }

    Ok(instructions)
}

fn step_action_for_operator(
    operator: &Operator<'_>,
    types: &[FunctionSignature],
    all_function_type_indices: &[u32],
    export_names: &HashMap<u32, String>,
) -> StepAction {
    let binary = |label: &str| StepAction::Binary(label.to_string());
    let unary = |label: &str| StepAction::Unary(label.to_string());
    let ternary = |label: &str| StepAction::Ternary(label.to_string());

    match operator {
        Operator::LocalGet { local_index } => StepAction::LocalGet(*local_index),
        Operator::LocalSet { local_index } => StepAction::LocalSet(*local_index),
        Operator::LocalTee { local_index } => StepAction::LocalTee(*local_index),
        Operator::GlobalGet { global_index } => StepAction::GlobalGet(*global_index),
        Operator::GlobalSet { global_index } => StepAction::GlobalSet(*global_index),
        Operator::I32Const { value } => StepAction::PushLiteral(format!("i32.const {}", value)),
        Operator::I64Const { value } => StepAction::PushLiteral(format!("i64.const {}", value)),
        Operator::F32Const { value } => StepAction::PushLiteral(format!("f32.const {:?}", value)),
        Operator::F64Const { value } => StepAction::PushLiteral(format!("f64.const {:?}", value)),
        Operator::RefNull { .. } => StepAction::PushLiteral("ref.null".to_string()),
        Operator::RefFunc { function_index } => {
            StepAction::PushLiteral(format!("ref.func {}", function_index))
        }
        Operator::Drop => StepAction::Pop(1),
        Operator::Select | Operator::TypedSelect { .. } => ternary("select"),
        Operator::Call { function_index } => {
            let label = export_names
                .get(function_index)
                .cloned()
                .unwrap_or_else(|| format!("func[{function_index}]"));
            let signature =
                signature_for_function(*function_index, types, all_function_type_indices);
            StepAction::Call {
                label,
                params: signature.map(|sig| sig.params.len()).unwrap_or_default(),
                results: signature.map(|sig| sig.results.len()).unwrap_or_default(),
                consume_callee: false,
            }
        }
        Operator::CallIndirect { type_index, .. } => {
            let signature = types.get(*type_index as usize);
            StepAction::Call {
                label: format!("call_indirect type[{type_index}]"),
                params: signature.map(|sig| sig.params.len()).unwrap_or_default(),
                results: signature.map(|sig| sig.results.len()).unwrap_or_default(),
                consume_callee: true,
            }
        }
        Operator::Return => StepAction::Return,
        Operator::BrIf { .. } => StepAction::Pop(1),
        Operator::If { .. } => StepAction::Pop(1),
        Operator::MemoryGrow { .. } | Operator::TableGrow { .. } => unary("grow"),
        Operator::MemorySize { .. } => StepAction::PushLiteral("memory.size".to_string()),
        Operator::TableSize { .. } => StepAction::PushLiteral("table.size".to_string()),
        Operator::MemoryCopy { .. } => StepAction::Pop(3),
        Operator::MemoryFill { .. } => StepAction::Pop(3),
        Operator::MemoryInit { .. } => StepAction::Pop(3),
        Operator::DataDrop { .. } => StepAction::None,
        Operator::TableCopy { .. } => StepAction::Pop(3),
        Operator::TableInit { .. } => StepAction::Pop(3),
        Operator::ElemDrop { .. } => StepAction::None,
        Operator::TableGet { .. } => unary("table.get"),
        Operator::TableSet { .. } => StepAction::Pop(2),
        Operator::I32Load { .. }
        | Operator::I64Load { .. }
        | Operator::F32Load { .. }
        | Operator::F64Load { .. }
        | Operator::I32Load8S { .. }
        | Operator::I32Load8U { .. }
        | Operator::I32Load16S { .. }
        | Operator::I32Load16U { .. }
        | Operator::I64Load8S { .. }
        | Operator::I64Load8U { .. }
        | Operator::I64Load16S { .. }
        | Operator::I64Load16U { .. }
        | Operator::I64Load32S { .. }
        | Operator::I64Load32U { .. } => unary("load"),
        Operator::I32Store { .. }
        | Operator::I64Store { .. }
        | Operator::F32Store { .. }
        | Operator::F64Store { .. }
        | Operator::I32Store8 { .. }
        | Operator::I32Store16 { .. }
        | Operator::I64Store8 { .. }
        | Operator::I64Store16 { .. }
        | Operator::I64Store32 { .. } => StepAction::Pop(2),
        Operator::I32Eqz | Operator::I64Eqz => unary("eqz"),
        Operator::I32Clz
        | Operator::I32Ctz
        | Operator::I32Popcnt
        | Operator::I64Clz
        | Operator::I64Ctz
        | Operator::I64Popcnt
        | Operator::F32Abs
        | Operator::F32Neg
        | Operator::F32Ceil
        | Operator::F32Floor
        | Operator::F32Trunc
        | Operator::F32Nearest
        | Operator::F32Sqrt
        | Operator::F64Abs
        | Operator::F64Neg
        | Operator::F64Ceil
        | Operator::F64Floor
        | Operator::F64Trunc
        | Operator::F64Nearest
        | Operator::F64Sqrt
        | Operator::I32WrapI64
        | Operator::I32TruncF32S
        | Operator::I32TruncF32U
        | Operator::I32TruncF64S
        | Operator::I32TruncF64U
        | Operator::I64ExtendI32S
        | Operator::I64ExtendI32U
        | Operator::I64TruncF32S
        | Operator::I64TruncF32U
        | Operator::I64TruncF64S
        | Operator::I64TruncF64U
        | Operator::F32ConvertI32S
        | Operator::F32ConvertI32U
        | Operator::F32ConvertI64S
        | Operator::F32ConvertI64U
        | Operator::F32DemoteF64
        | Operator::F64ConvertI32S
        | Operator::F64ConvertI32U
        | Operator::F64ConvertI64S
        | Operator::F64ConvertI64U
        | Operator::F64PromoteF32
        | Operator::I32Extend8S
        | Operator::I32Extend16S
        | Operator::I64Extend8S
        | Operator::I64Extend16S
        | Operator::I64Extend32S => unary("unary"),
        Operator::I32Eq
        | Operator::I32Ne
        | Operator::I32LtS
        | Operator::I32LtU
        | Operator::I32GtS
        | Operator::I32GtU
        | Operator::I32LeS
        | Operator::I32LeU
        | Operator::I32GeS
        | Operator::I32GeU
        | Operator::I64Eq
        | Operator::I64Ne
        | Operator::I64LtS
        | Operator::I64LtU
        | Operator::I64GtS
        | Operator::I64GtU
        | Operator::I64LeS
        | Operator::I64LeU
        | Operator::I64GeS
        | Operator::I64GeU
        | Operator::F32Eq
        | Operator::F32Ne
        | Operator::F32Lt
        | Operator::F32Gt
        | Operator::F32Le
        | Operator::F32Ge
        | Operator::F64Eq
        | Operator::F64Ne
        | Operator::F64Lt
        | Operator::F64Gt
        | Operator::F64Le
        | Operator::F64Ge
        | Operator::I32Add
        | Operator::I32Sub
        | Operator::I32Mul
        | Operator::I32DivS
        | Operator::I32DivU
        | Operator::I32RemS
        | Operator::I32RemU
        | Operator::I32And
        | Operator::I32Or
        | Operator::I32Xor
        | Operator::I32Shl
        | Operator::I32ShrS
        | Operator::I32ShrU
        | Operator::I32Rotl
        | Operator::I32Rotr
        | Operator::I64Add
        | Operator::I64Sub
        | Operator::I64Mul
        | Operator::I64DivS
        | Operator::I64DivU
        | Operator::I64RemS
        | Operator::I64RemU
        | Operator::I64And
        | Operator::I64Or
        | Operator::I64Xor
        | Operator::I64Shl
        | Operator::I64ShrS
        | Operator::I64ShrU
        | Operator::I64Rotl
        | Operator::I64Rotr
        | Operator::F32Add
        | Operator::F32Sub
        | Operator::F32Mul
        | Operator::F32Div
        | Operator::F32Min
        | Operator::F32Max
        | Operator::F32Copysign
        | Operator::F64Add
        | Operator::F64Sub
        | Operator::F64Mul
        | Operator::F64Div
        | Operator::F64Min
        | Operator::F64Max
        | Operator::F64Copysign => binary("binary"),
        _ => StepAction::None,
    }
}

fn signature_for_function<'a>(
    function_index: u32,
    types: &'a [FunctionSignature],
    all_function_type_indices: &[u32],
) -> Option<&'a FunctionSignature> {
    let type_index = *all_function_type_indices.get(function_index as usize)?;
    types.get(type_index as usize)
}

#[derive(Debug, Default, Clone)]
struct SourceMapper {
    entries: BTreeMap<u64, SourceLocation>,
}

impl SourceMapper {
    fn load(wasm_path: &Path) -> Result<Self, String> {
        let map_path = source_map_path(wasm_path);
        if !map_path.exists() {
            return Ok(Self::default());
        }

        let raw = fs::read_to_string(&map_path).map_err(|error| {
            format!(
                "failed to read source map '{}': {}",
                map_path.display(),
                error
            )
        })?;

        let entries: Vec<SourceLocation> = serde_json::from_str(&raw).map_err(|error| {
            format!(
                "failed to parse source map '{}': {}",
                map_path.display(),
                error
            )
        })?;

        let mut by_offset = BTreeMap::new();
        for entry in entries {
            by_offset.insert(entry.offset, entry);
        }

        Ok(Self { entries: by_offset })
    }

    fn lookup(&self, offset: u64) -> Option<&SourceLocation> {
        self.entries
            .range(..=offset)
            .next_back()
            .map(|(_, entry)| entry)
    }
}

fn source_map_path(wasm_path: &Path) -> PathBuf {
    wasm_path.with_extension("map.json")
}

#[derive(Debug, Deserialize, Clone)]
struct SourceLocation {
    offset: u64,
    file: String,
    line: u32,
    column: Option<u32>,
    source: Option<String>,
}

#[cfg(test)]
mod tests {
    use super::*;

    fn build_session(wat: &str) -> DebuggerSession {
        let wasm = wat::parse_str(wat).expect("valid wat");
        let module = ModuleDebugInfo::from_bytes(&wasm).expect("module should parse");
        DebuggerSession::new(module, SourceMapper::default()).expect("session should initialize")
    }

    #[test]
    fn list_exported_functions() {
        let session = build_session(
            r#"
            (module
              (func (export "add_one") (param i32) (result i32)
                local.get 0
                i32.const 1
                i32.add))
            "#,
        );

        let listing = session.list_functions();
        assert!(listing.contains("add_one"));
    }

    #[test]
    fn step_instruction_updates_symbolic_stack() {
        let mut session = build_session(
            r#"
            (module
              (func (export "add_one") (param i32) (result i32)
                local.get 0
                i32.const 1
                i32.add))
            "#,
        );

        let first = session.step_instruction().expect("first step");
        assert!(first.contains("LocalGet"));
        assert!(first.contains("stack [local[0]]"));

        let second = session.step_instruction().expect("second step");
        assert!(second.contains("I32Const"));
        assert!(second.contains("i32.const 1"));

        let third = session.step_instruction().expect("third step");
        assert!(third.contains("I32Add"));
        assert!(third.contains("binary(local[0], i32.const 1)"));
    }

    #[test]
    fn source_mapper_returns_nearest_entry() {
        let mut mapper = SourceMapper::default();
        mapper.entries.insert(
            8,
            SourceLocation {
                offset: 8,
                file: "src/lib.rs".to_string(),
                line: 12,
                column: Some(4),
                source: Some("x + 1".to_string()),
            },
        );

        let location = mapper.lookup(10).expect("location should resolve");
        assert_eq!(location.file, "src/lib.rs");
        assert_eq!(location.line, 12);
    }
}
