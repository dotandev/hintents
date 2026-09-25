import express, { Request, Response } from 'express';
import cors from 'cors';
import { exec } from 'child_process';
import fs from 'fs';
import path from 'path';
import os from 'os';

const app = express();
app.use(cors());
app.use(express.json());

interface CompileRequest {
  code: string;
}

app.post('/api/compile', (req: Request<{}, {}, CompileRequest>, res: Response) => {
  const { code } = req.body;
  
  if (!code) {
    return res.status(400).json({ error: 'No code provided.' });
  }

  // 1. Create a temporary directory
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'erst-playground-'));
  const srcDir = path.join(tempDir, 'src');
  fs.mkdirSync(srcDir);

  // 2. Write the Rust code to lib.rs
  const libRsPath = path.join(srcDir, 'lib.rs');
  fs.writeFileSync(libRsPath, code);

  // 3. Write a standard Soroban Cargo.toml
  const cargoToml = `[package]
name = "playground-contract"
version = "0.1.0"
edition = "2021"

[lib]
crate-type = ["cdylib"]

[dependencies]
soroban-sdk = "21.0.0"

[profile.release]
opt-level = "z"
opt-level = "z"
overflow-checks = true
debug = 0
strip = "symbols"
debug-assertions = false
panic = "abort"
codegen-units = 1
lto = true
`;
  fs.writeFileSync(path.join(tempDir, 'Cargo.toml'), cargoToml);

  console.log(`Starting compilation in ${tempDir}`);

  // 4. Run cargo build (target wasm32)
  exec('cargo build --target wasm32-unknown-unknown --release', { cwd: tempDir }, (error, stdout, stderr) => {
    if (error) {
      console.error('Compilation failed:', stderr);
      return res.status(400).json({ error: 'Compilation Failed', details: stderr });
    }

    const wasmPath = path.join(tempDir, 'target', 'wasm32-unknown-unknown', 'release', 'playground_contract.wasm');
    
    // 5. Check if WASM generated
    if (!fs.existsSync(wasmPath)) {
      return res.status(500).json({ error: 'WASM binary not found after compilation.' });
    }

    // Return success (in real-world, we'd now pipe this to `erst` simulator)
    res.json({
      status: 'success',
      message: 'Compiled successfully to WASM!',
      wasmSize: fs.statSync(wasmPath).size,
      logs: stdout
    });
  });
});

const PORT = process.env.PORT || 8080;
app.listen(PORT, () => {
  console.log(`Erst Playground Engine running on port ${PORT}`);
});
