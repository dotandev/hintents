// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package sourcemap

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dotandev/hintents/internal/dwarf"
)

// Helper to build a WASM binary with specified custom sections
func buildTestWASM(sections map[string][]byte) []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00})

	for name, content := range sections {
		buf.WriteByte(0x00) // custom section

		nameBytes := []byte(name)
		var nameLenBuf []byte
		nameLenBuf = appendTestULEB(nameLenBuf, uint64(len(nameBytes)))

		payloadLen := len(nameLenBuf) + len(nameBytes) + len(content)
		var payloadLenBuf []byte
		payloadLenBuf = appendTestULEB(payloadLenBuf, uint64(payloadLen))

		buf.Write(payloadLenBuf)
		buf.Write(nameLenBuf)
		buf.Write(nameBytes)
		buf.Write(content)
	}

	return buf.Bytes()
}

func appendTestULEB(dst []byte, val uint64) []byte {
	for {
		b := byte(val & 0x7f)
		val >>= 7
		if val != 0 {
			b |= 0x80
		}
		dst = append(dst, b)
		if val == 0 {
			break
		}
	}
	return dst
}

func testDWARFSections(isDWO bool) map[string][]byte {
	abbrev := []byte{0x01, 0x11, 0x00, 0x00, 0x00, 0x00}
	info := []byte{
		0x08, 0x00, 0x00, 0x00, // length
		0x04, 0x00, // version = 4
		0x00, 0x00, 0x00, 0x00, // abbrev offset
		0x04, // addr size
		0x01, // DIE tag 1
	}

	if isDWO {
		return map[string][]byte{
			".debug_abbrev.dwo": abbrev,
			".debug_info.dwo":   info,
		}
	}
	return map[string][]byte{
		".debug_abbrev": abbrev,
		".debug_info":   info,
	}
}

func TestDefaultDetachedSymbolDirs(t *testing.T) {
	t.Run("with project root", func(t *testing.T) {
		root := "/test/project"
		dirs := DefaultDetachedSymbolDirs(root)
		expected := []string{
			filepath.Join(root, wasmTargetPath),
			filepath.Join(root, "target/wasm32-unknown-unknown/debug"),
			filepath.Join(root, ".erst", "symbols"),
			filepath.Join(root, "symbols"),
			root,
		}
		for _, exp := range expected {
			found := false
			for _, d := range dirs {
				if d == exp {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected dir %q in %v", exp, dirs)
			}
		}
	})

	t.Run("with environment variable", func(t *testing.T) {
		extraDir := "/custom/symbol/dir"
		t.Setenv("ERST_SYMBOL_DIRS", extraDir)
		dirs := DefaultDetachedSymbolDirs("")
		found := false
		for _, d := range dirs {
			if d == extraDir {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q from ERST_SYMBOL_DIRS in %v", extraDir, dirs)
		}
	})
}

func TestFindDetachedSymbolFile_MatchingFormats(t *testing.T) {
	tempDir := t.TempDir()

	hash1 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	hash2 := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	hash3 := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	hash4 := "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	hash5 := "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	hash6 := "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"

	debugWASM := buildTestWASM(testDWARFSections(false))

	// 1. Direct <hash>.dwp
	p1 := filepath.Join(tempDir, hash1+".dwp")
	if err := os.WriteFile(p1, debugWASM, 0600); err != nil {
		t.Fatal(err)
	}

	// 2. Direct <hash>.debug.wasm
	p2 := filepath.Join(tempDir, hash2+".debug.wasm")
	if err := os.WriteFile(p2, debugWASM, 0600); err != nil {
		t.Fatal(err)
	}

	// 3. Prefixed contract.<hash>.dwp
	p3 := filepath.Join(tempDir, "contract."+hash3+".dwp")
	if err := os.WriteFile(p3, debugWASM, 0600); err != nil {
		t.Fatal(err)
	}

	// 4. Prefixed contract-<hash>.debug.wasm
	p4 := filepath.Join(tempDir, "contract-"+hash4+".debug.wasm")
	if err := os.WriteFile(p4, debugWASM, 0600); err != nil {
		t.Fatal(err)
	}

	// 5. In subdirectory <hash>/debug.wasm
	subDir := filepath.Join(tempDir, hash5)
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	p5 := filepath.Join(subDir, "debug.wasm")
	if err := os.WriteFile(p5, debugWASM, 0600); err != nil {
		t.Fatal(err)
	}

	// 6. Embedded custom section module_hash
	secs := testDWARFSections(false)
	secs["module_hash"] = []byte(hash6)
	wasmWithHash := buildTestWASM(secs)
	p6 := filepath.Join(tempDir, "custom_named.debug.wasm")
	if err := os.WriteFile(p6, wasmWithHash, 0600); err != nil {
		t.Fatal(err)
	}

	searchDirs := []string{tempDir}

	tests := []struct {
		name     string
		hash     string
		wantPath string
	}{
		{"direct dwp", hash1, p1},
		{"direct debug wasm uppercase query", strings.ToUpper(hash2), p2},
		{"prefixed dot dwp", hash3, p3},
		{"prefixed dash debug wasm", hash4, p4},
		{"subdirectory hash", hash5, p5},
		{"embedded custom section hash", hash6, p6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FindDetachedSymbolFile(searchDirs, tt.hash)
			if err != nil {
				t.Fatalf("FindDetachedSymbolFile(%q) unexpected error: %v", tt.hash, err)
			}
			if got != tt.wantPath {
				t.Errorf("got %q, want %q", got, tt.wantPath)
			}
		})
	}
}

func TestFindDetachedSymbolFile_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	nonExistentHash := "0000000000000000000000000000000000000000000000000000000000000000"

	_, err := FindDetachedSymbolFile([]string{tempDir}, nonExistentHash)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrDetachedSymbolNotFound) {
		t.Errorf("expected ErrDetachedSymbolNotFound, got %v", err)
	}

	var notFoundErr *DetachedSymbolNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Errorf("expected *DetachedSymbolNotFoundError, got %T", err)
	}
}

func TestFindDetachedSymbolFile_EmptyHash(t *testing.T) {
	_, err := FindDetachedSymbolFile([]string{"/some/dir"}, "")
	if err == nil {
		t.Fatal("expected error for empty hash")
	}
}

func TestDiscoverDetachedSymbols(t *testing.T) {
	tempDir := t.TempDir()

	hash1 := "1111111111111111111111111111111111111111111111111111111111111111"
	hash2 := "2222222222222222222222222222222222222222222222222222222222222222"

	debugWASM := buildTestWASM(testDWARFSections(false))

	p1 := filepath.Join(tempDir, hash1+".dwp")
	p2 := filepath.Join(tempDir, "token."+hash2+".debug.wasm")

	_ = os.WriteFile(p1, debugWASM, 0600)
	_ = os.WriteFile(p2, debugWASM, 0600)

	found, err := DiscoverDetachedSymbols([]string{tempDir})
	if err != nil {
		t.Fatalf("DiscoverDetachedSymbols failed: %v", err)
	}

	if got := found[hash1]; got != p1 {
		t.Errorf("found[%s] = %q, want %q", hash1, got, p1)
	}
	if got := found[hash2]; got != p2 {
		t.Errorf("found[%s] = %q, want %q", hash2, got, p2)
	}
}

func TestLoadDWARFForModule_Embedded(t *testing.T) {
	embeddedWASM := buildTestWASM(testDWARFSections(false))

	p, err := LoadDWARFForModule(embeddedWASM, nil)
	if err != nil {
		t.Fatalf("LoadDWARFForModule with embedded DWARF failed: %v", err)
	}
	if !p.HasDebugInfo() {
		t.Fatal("expected debug info to be present")
	}
}

func TestLoadDWARFForModule_DetachedWASM(t *testing.T) {
	tempDir := t.TempDir()

	// Stripped module (no debug info)
	strippedModule := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	sum := sha256.Sum256(strippedModule)
	moduleHash := hex.EncodeToString(sum[:])

	// Create detached .debug.wasm in search directory
	detachedWASM := buildTestWASM(testDWARFSections(false))
	detachedPath := filepath.Join(tempDir, moduleHash+".debug.wasm")
	if err := os.WriteFile(detachedPath, detachedWASM, 0600); err != nil {
		t.Fatal(err)
	}

	// Load DWARF for stripped module
	p, err := LoadDWARFForModule(strippedModule, []string{tempDir})
	if err != nil {
		t.Fatalf("LoadDWARFForModule failed: %v", err)
	}
	if !p.HasDebugInfo() {
		t.Fatal("expected debug info loaded from detached symbol file")
	}
	if p.BinaryType() != "wasm" {
		t.Errorf("binaryType = %q, want wasm", p.BinaryType())
	}
}

func TestLoadDWARFForModule_DetachedDWPSplit(t *testing.T) {
	tempDir := t.TempDir()

	strippedModule := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	sum := sha256.Sum256(strippedModule)
	moduleHash := hex.EncodeToString(sum[:])

	// Create detached .dwp using split DWO sections
	detachedDWP := buildTestWASM(testDWARFSections(true))
	detachedPath := filepath.Join(tempDir, moduleHash+".dwp")
	if err := os.WriteFile(detachedPath, detachedDWP, 0600); err != nil {
		t.Fatal(err)
	}

	p, err := LoadDWARFForModule(strippedModule, []string{tempDir})
	if err != nil {
		t.Fatalf("LoadDWARFForModule with split DWO dwp failed: %v", err)
	}
	if !p.HasDebugInfo() {
		t.Fatal("expected debug info from split DWO dwp")
	}
}

func TestLoadDWARFForModule_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	strippedModule := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}

	_, err := LoadDWARFForModule(strippedModule, []string{tempDir})
	if err == nil {
		t.Fatal("expected error when detached symbols are missing")
	}
	if !errors.Is(err, ErrDetachedSymbolNotFound) {
		t.Errorf("expected error wrapping ErrDetachedSymbolNotFound, got %v", err)
	}
}

func TestLoadDetachedDWARF(t *testing.T) {
	tempDir := t.TempDir()
	hash := "3333333333333333333333333333333333333333333333333333333333333333"

	debugWASM := buildTestWASM(testDWARFSections(false))
	detachedPath := filepath.Join(tempDir, hash+".dwp")
	_ = os.WriteFile(detachedPath, debugWASM, 0600)

	p, err := LoadDetachedDWARF([]string{tempDir}, hash)
	if err != nil {
		t.Fatalf("LoadDetachedDWARF failed: %v", err)
	}
	if !p.HasDebugInfo() {
		t.Fatal("expected debug info")
	}
}

func TestLoadDetachedDWARFInto(t *testing.T) {
	tempDir := t.TempDir()
	hash := "4444444444444444444444444444444444444444444444444444444444444444"

	debugWASM := buildTestWASM(testDWARFSections(false))
	detachedPath := filepath.Join(tempDir, hash+".debug.wasm")
	_ = os.WriteFile(detachedPath, debugWASM, 0600)

	p := &dwarf.Parser{}
	if p.HasDebugInfo() {
		t.Fatal("expected no debug info initially")
	}

	if err := LoadDetachedDWARFInto(p, []string{tempDir}, hash); err != nil {
		t.Fatalf("LoadDetachedDWARFInto failed: %v", err)
	}
	if !p.HasDebugInfo() {
		t.Fatal("expected debug info after loading")
	}
}

func TestLoadDWARFForModuleFile(t *testing.T) {
	tempDir := t.TempDir()

	strippedModule := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	sum := sha256.Sum256(strippedModule)
	moduleHash := hex.EncodeToString(sum[:])

	wasmPath := filepath.Join(tempDir, "contract.wasm")
	_ = os.WriteFile(wasmPath, strippedModule, 0600)

	debugWASM := buildTestWASM(testDWARFSections(false))
	debugPath := filepath.Join(tempDir, moduleHash+".dwp")
	_ = os.WriteFile(debugPath, debugWASM, 0600)

	p, err := LoadDWARFForModuleFile(wasmPath, []string{tempDir})
	if err != nil {
		t.Fatalf("LoadDWARFForModuleFile failed: %v", err)
	}
	if !p.HasDebugInfo() {
		t.Fatal("expected debug info")
	}
}

func TestDiscoverLocalSymbols_WithDetached(t *testing.T) {
	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, wasmTargetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}

	hash := "5555555555555555555555555555555555555555555555555555555555555555"
	debugWASM := buildTestWASM(testDWARFSections(false))
	dwpPath := filepath.Join(targetDir, hash+".dwp")
	_ = os.WriteFile(dwpPath, debugWASM, 0600)

	found, err := DiscoverLocalSymbols(tempDir)
	if err != nil {
		t.Fatalf("DiscoverLocalSymbols failed: %v", err)
	}

	if got := found[hash]; got != dwpPath {
		t.Errorf("found[%s] = %q, want %q", hash, got, dwpPath)
	}
}
