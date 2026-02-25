#!/usr/bin/env pwsh

# Copyright 2025 Erst Users
# SPDX-License-Identifier: Apache-2.0

# Check for license headers in Go and Rust files
# Exit with status 1 if any files are missing headers

$ErrorActionPreference = "Stop"
$MISSING_HEADERS = 0
$EXPECTED_HEADER = "Copyright 2025 Erst Users"

Write-Host "Checking for license headers in Go and Rust files..."

# Check Go files
Write-Host ""
Write-Host "Checking Go files (.go)..."
$goFiles = Get-ChildItem -Path . -Filter "*.go" -Recurse | Where-Object { $_.FullName -notlike "*target*" -and $_.FullName -notlike "*vendor*" }
foreach ($file in $goFiles) {
    $lines = Get-Content $file.FullName
    $firstNonBuildLine = $null
    
    foreach ($line in $lines) {
        if ($line -notmatch "^//go:build" -and $line -notmatch "^// \+build" -and $line.Trim() -ne "") {
            $firstNonBuildLine = $line
            break
        }
    }
    
    if ($firstNonBuildLine -and $firstNonBuildLine -notmatch $EXPECTED_HEADER) {
        Write-Host "  [FAIL] Missing license header: $($file.FullName)"
        $MISSING_HEADERS++
    } else {
        Write-Host "  [OK] $($file.FullName)"
    }
}

# Check Rust files
Write-Host ""
Write-Host "Checking Rust files (.rs)..."
$rsFiles = Get-ChildItem -Path . -Filter "*.rs" -Recurse | Where-Object { $_.FullName -notlike "*target*" -and $_.FullName -notlike "*vendor*" }
foreach ($file in $rsFiles) {
    $firstLine = Get-Content $file.FullName -TotalCount 1
    if ($firstLine -notmatch $EXPECTED_HEADER) {
        Write-Host "  [FAIL] Missing license header: $($file.FullName)"
        $MISSING_HEADERS++
    } else {
        Write-Host "  [OK] $($file.FullName)"
    }
}

Write-Host ""
if ($MISSING_HEADERS -eq 0) {
    Write-Host "[OK] All files have proper license headers"
    exit 0
} else {
    Write-Host "[FAIL] Found $MISSING_HEADERS file(s) missing license headers"
    exit 1
}
