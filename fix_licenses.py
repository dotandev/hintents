#!/usr/bin/env python3
"""Fix license headers in Go and Rust files without breaking string literals."""

import os
import sys

HEADER = """// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

"""

def add_license_header(file_path):
    """Add license header to a file if missing."""
    with open(file_path, 'r', encoding='utf-8') as f:
        content = f.read()
    
    # Check if header already exists
    if "Copyright 2025 Erst Users" in content:
        return False
    
    # Add header at the beginning
    with open(file_path, 'w', encoding='utf-8') as f:
        f.write(HEADER + content)
    
    return True

def main():
    """Main function to fix license headers in all Go and Rust files."""
    fixed_count = 0
    
    for root, dirs, files in os.walk('.'):
        # Skip hidden directories and vendor/target
        dirs[:] = [d for d in dirs if not d.startswith('.') and d not in ['vendor', 'target']]
        
        for file in files:
            if file.endswith('.go') or file.endswith('.rs'):
                file_path = os.path.join(root, file)
                try:
                    if add_license_header(file_path):
                        print(f"🔧 Fixing: {file_path}")
                        fixed_count += 1
                except Exception as e:
                    print(f"⚠️ Error processing {file_path}: {e}")
    
    print(f"✅ Fixed {fixed_count} files")

if __name__ == "__main__":
    main()
