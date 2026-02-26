// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

#!/usr/bin/env node

const fs = require('fs');
const path = require('path');

const schemaDir = __dirname;
const schemaFiles = [
  'common.schema.json',
  'diagnostic-event.schema.json',
  'categorized-event.schema.json',
  'budget-usage.schema.json',
  'auth-trace.schema.json',
  'wasm-stack-trace.schema.json',
  'simulation-response.schema.json',
  'simulation-request.schema.json'
];

let hasErrors = false;

console.log('Validating schema files...\n');

// Validate each schema file
for (const file of schemaFiles) {
  console.log(`Validating ${file}...`);
  const filePath = path.join(schemaDir, file);
  
  try {
    // Read and parse JSON
    const content = fs.readFileSync(filePath, 'utf8');
    const schema = JSON.parse(content);
    
    // Check for required fields
    const requiredFields = ['$schema', '$id', 'version'];
    const missingFields = requiredFields.filter(field => !schema[field]);
    
    if (missingFields.length > 0) {
      console.error(`  [FAIL] Missing required fields: ${missingFields.join(', ')}`);
      hasErrors = true;
    } else {
      console.log(`  [OK] Has all required fields ($schema, $id, version)`);
    }
    
    // Check version format
    if (schema.version && !/^\d+\.\d+\.\d+$/.test(schema.version)) {
      console.error(`  [FAIL] Invalid version format: ${schema.version} (expected MAJOR.MINOR.PATCH)`);
      hasErrors = true;
    } else if (schema.version) {
      console.log(`  [OK] Version format is valid: ${schema.version}`);
    }
    
    // Extract all $ref values
    const refs = [];
    const extractRefs = (obj) => {
      if (typeof obj !== 'object' || obj === null) return;
      
      for (const key in obj) {
        if (key === '$ref' && typeof obj[key] === 'string') {
          refs.push(obj[key]);
        } else {
          extractRefs(obj[key]);
        }
      }
    };
    extractRefs(schema);
    
    // Check $ref paths
    for (const ref of refs) {
      // Internal references (starting with #) are OK
      if (ref.startsWith('#')) {
        continue;
      }
      
      // Check if it's a relative path (not an absolute URL)
      if (ref.startsWith('http://') || ref.startsWith('https://')) {
        console.error(`  [FAIL] Found absolute URL in $ref: ${ref} (should be relative path)`);
        hasErrors = true;
        continue;
      }
      
      // Extract file path (before #)
      const refFile = ref.split('#')[0];
      
      // Check if referenced file exists
      const refPath = path.join(schemaDir, refFile);
      if (!fs.existsSync(refPath)) {
        console.error(`  [FAIL] Referenced file does not exist: ${refFile}`);
        hasErrors = true;
      } else {
        console.log(`  [OK] Reference exists: ${refFile}`);
      }
    }
    
    console.log(`  [OK] ${file} is valid\n`);
    
  } catch (error) {
    console.error(`  [FAIL] Error parsing ${file}: ${error.message}\n`);
    hasErrors = true;
  }
}

if (hasErrors) {
  console.error('[FAIL] Validation failed with errors');
  process.exit(1);
} else {
  console.log('[OK] All schema files are valid!');
  process.exit(0);
}
