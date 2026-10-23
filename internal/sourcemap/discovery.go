// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package sourcemap

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dotandev/hintents/internal/dwarf"
)

const wasmTargetPath = "target/wasm32-unknown-unknown/release"

// ErrDetachedSymbolNotFound is returned when no matching detached debug symbol file
// (.dwp or .debug.wasm) is found for a given module hash.
var ErrDetachedSymbolNotFound = errors.New("detached debug symbols not found")

// DetachedSymbolNotFoundError provides details on which directories were searched
// when a detached symbol file could not be found for a given module hash.
type DetachedSymbolNotFoundError struct {
	ModuleHash string
	Dirs       []string
}

func (e *DetachedSymbolNotFoundError) Error() string {
	return fmt.Sprintf("detached debug symbol file (.dwp / .debug.wasm) not found for module hash %s in directories: [%s]",
		e.ModuleHash, strings.Join(e.Dirs, ", "))
}

func (e *DetachedSymbolNotFoundError) Unwrap() error {
	return ErrDetachedSymbolNotFound
}

// HashMismatchError is returned when the local WASM hash does not match
// the expected on-chain hash.
type HashMismatchError struct {
	Path    string
	Local   string
	OnChain string
}

func (e *HashMismatchError) Error() string {
	return fmt.Sprintf("opt-level mismatch: local WASM hash %q does not match on-chain hash %q (path: %s)", e.Local, e.OnChain, e.Path)
}

// CheckHashMismatch computes the SHA256 hash of the WASM at path and
// compares it to onChainHash. It returns a HashMismatchError when they
// differ, so callers can surface a warning to the user.
func CheckHashMismatch(path, onChainHash string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read WASM file %q: %w", path, err)
	}
	sum := sha256.Sum256(content)
	local := hex.EncodeToString(sum[:])
	if local != onChainHash {
		return &HashMismatchError{Path: path, Local: local, OnChain: onChainHash}
	}
	return nil
}

// DefaultDetachedSymbolDirs returns a slice of directories where detached debug
// symbol files (.dwp and .debug.wasm) are typically located.
// When projectRoot is provided, target directories relative to projectRoot are included.
// It also checks the ERST_SYMBOL_DIRS environment variable (colon- or semicolon-separated).
func DefaultDetachedSymbolDirs(projectRoot string) []string {
	var dirs []string
	if projectRoot != "" {
		dirs = append(dirs,
			filepath.Join(projectRoot, wasmTargetPath),
			filepath.Join(projectRoot, "target/wasm32-unknown-unknown/debug"),
			filepath.Join(projectRoot, ".erst", "symbols"),
			filepath.Join(projectRoot, "symbols"),
			projectRoot,
		)
	} else {
		dirs = append(dirs,
			wasmTargetPath,
			"target/wasm32-unknown-unknown/debug",
			filepath.Join(".erst", "symbols"),
			"symbols",
			".",
		)
	}

	if envDirs := os.Getenv("ERST_SYMBOL_DIRS"); envDirs != "" {
		for _, d := range filepath.SplitList(envDirs) {
			d = strings.TrimSpace(d)
			if d != "" {
				dirs = append(dirs, d)
			}
		}
	}

	return dirs
}

// isDetachedSymbolFile checks whether a file name has a detached debug symbol extension
// (.dwp or .debug.wasm).
func isDetachedSymbolFile(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".dwp") || strings.HasSuffix(lower, ".debug.wasm")
}

// isHex returns true if the string consists solely of hexadecimal characters.
func isHex(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// extractHashFromDetachedName extracts a candidate module hash from a detached symbol filename.
func extractHashFromDetachedName(name string) string {
	lower := strings.ToLower(name)
	trimmed := lower
	if strings.HasSuffix(trimmed, ".debug.wasm") {
		trimmed = strings.TrimSuffix(trimmed, ".debug.wasm")
	} else if strings.HasSuffix(trimmed, ".dwp") {
		trimmed = strings.TrimSuffix(trimmed, ".dwp")
	} else {
		return ""
	}

	// Exact hash (e.g. 64-char sha256 or any pure hex)
	if isHex(trimmed) {
		return trimmed
	}

	// Try common separators like '.', '-', '_' (e.g. contract.<hash>.dwp)
	for _, delim := range []rune{'.', '-', '_'} {
		parts := strings.Split(trimmed, string(delim))
		for _, part := range parts {
			if len(part) == 64 && isHex(part) {
				return part
			}
		}
	}

	return ""
}

// matchesEmbeddedModuleHash inspects custom sections in a WASM file to see if
// a module_hash / build_id section matches targetHash.
func matchesEmbeddedModuleHash(data []byte, targetHash string) bool {
	if len(data) < 8 || string(data[:4]) != "\x00asm" {
		return false
	}
	sections := dwarf.ParseWASMSections(data)
	for name, val := range sections {
		nameLower := strings.ToLower(name)
		if nameLower == "module_hash" || nameLower == ".module_hash" || nameLower == "build_id" || nameLower == "contract_hash" {
			valStr := strings.ToLower(strings.TrimSpace(string(val)))
			valStr = strings.TrimPrefix(valStr, "0x")
			if valStr == targetHash {
				return true
			}
		}
	}
	return false
}

// matchFileWithHash checks if a file at path matches targetHash via its name,
// parent directory, or embedded WASM custom sections.
func matchFileWithHash(path string, targetHash string) bool {
	base := filepath.Base(path)
	lowerBase := strings.ToLower(base)
	cleanTarget := strings.ToLower(strings.TrimSpace(targetHash))
	if cleanTarget == "" {
		return false
	}

	// Direct match: <hash>.dwp or <hash>.debug.wasm
	if lowerBase == cleanTarget+".dwp" || lowerBase == cleanTarget+".debug.wasm" {
		return true
	}

	// Prefix / suffix / delimiter match: e.g. contract.<hash>.dwp
	if strings.Contains(lowerBase, cleanTarget) {
		return true
	}

	// Parent directory match: <dir>/<hash>/contract.dwp
	parentDir := strings.ToLower(filepath.Base(filepath.Dir(path)))
	if parentDir == cleanTarget {
		return true
	}

	// Check embedded WASM custom sections
	if strings.HasSuffix(lowerBase, ".wasm") {
		if content, err := os.ReadFile(path); err == nil {
			if matchesEmbeddedModuleHash(content, cleanTarget) {
				return true
			}
		}
	}

	return false
}

// FindDetachedSymbolFile searches a configurable list of directories for a matching
// .dwp or .debug.wasm file, keyed by module hash.
// If dirs is empty, DefaultDetachedSymbolDirs("") is used.
func FindDetachedSymbolFile(dirs []string, moduleHash string) (string, error) {
	cleanHash := strings.ToLower(strings.TrimSpace(moduleHash))
	if cleanHash == "" {
		return "", errors.New("module hash cannot be empty")
	}

	searchDirs := dirs
	if len(searchDirs) == 0 {
		searchDirs = DefaultDetachedSymbolDirs("")
	}

	for _, dir := range searchDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		// Fast path: direct candidates in dir
		candidates := []string{
			filepath.Join(dir, cleanHash+".dwp"),
			filepath.Join(dir, cleanHash+".debug.wasm"),
			filepath.Join(dir, strings.ToUpper(cleanHash)+".dwp"),
			filepath.Join(dir, strings.ToUpper(cleanHash)+".debug.wasm"),
			filepath.Join(dir, cleanHash, "debug.dwp"),
			filepath.Join(dir, cleanHash, "debug.wasm"),
			filepath.Join(dir, cleanHash, cleanHash+".dwp"),
			filepath.Join(dir, cleanHash, cleanHash+".debug.wasm"),
		}
		for _, cand := range candidates {
			if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
				abs, err := filepath.Abs(cand)
				if err == nil {
					return abs, nil
				}
				return cand, nil
			}
		}

		// Walk directory for matching files
		var matchedPath string
		_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if isDetachedSymbolFile(d.Name()) && matchFileWithHash(p, cleanHash) {
				matchedPath = p
				return filepath.SkipAll
			}
			return nil
		})

		if matchedPath != "" {
			abs, err := filepath.Abs(matchedPath)
			if err == nil {
				return abs, nil
			}
			return matchedPath, nil
		}
	}

	return "", &DetachedSymbolNotFoundError{
		ModuleHash: moduleHash,
		Dirs:       searchDirs,
	}
}

// FindDetachedSymbols is an alias for FindDetachedSymbolFile.
func FindDetachedSymbols(dirs []string, moduleHash string) (string, error) {
	return FindDetachedSymbolFile(dirs, moduleHash)
}

// DiscoverDetachedSymbols scans a configurable list of directories for detached
// debug symbol files (.dwp and .debug.wasm) and returns a map of module hashes
// to absolute file paths.
func DiscoverDetachedSymbols(dirs []string) (map[string]string, error) {
	searchDirs := dirs
	if len(searchDirs) == 0 {
		searchDirs = DefaultDetachedSymbolDirs("")
	}

	found := make(map[string]string)

	for _, dir := range searchDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}

			if !isDetachedSymbolFile(d.Name()) {
				return nil
			}

			abs, err := filepath.Abs(p)
			if err != nil {
				abs = p
			}

			// Method A: Check parent dir for hash
			parentName := strings.ToLower(filepath.Base(filepath.Dir(p)))
			if len(parentName) == 64 && isHex(parentName) {
				if _, exists := found[parentName]; !exists {
					found[parentName] = abs
				}
			}

			// Method B: Extract hash from filename
			if h := extractHashFromDetachedName(d.Name()); h != "" {
				if _, exists := found[h]; !exists {
					found[h] = abs
				}
			}

			// Method C: Embedded custom section in WASM
			if strings.HasSuffix(strings.ToLower(d.Name()), ".wasm") {
				if content, err := os.ReadFile(p); err == nil {
					sections := dwarf.ParseWASMSections(content)
					for name, val := range sections {
						nameLower := strings.ToLower(name)
						if nameLower == "module_hash" || nameLower == ".module_hash" || nameLower == "build_id" {
							valStr := strings.ToLower(strings.TrimSpace(string(val)))
							valStr = strings.TrimPrefix(valStr, "0x")
							if isHex(valStr) {
								if _, exists := found[valStr]; !exists {
									found[valStr] = abs
								}
							}
						}
					}
				}
			}

			return nil
		})
	}

	return found, nil
}

// LoadDWARFForModule loads DWARF debug information for a WASM module.
// If the module contains embedded DWARF symbols, it parses and returns them directly.
// If the module has none embedded (e.g. a stripped release build), it computes the
// module's hash, searches searchDirs for a matching .dwp or .debug.wasm file,
// and loads the DWARF from that detached file.
func LoadDWARFForModule(moduleData []byte, searchDirs []string) (*dwarf.Parser, error) {
	// First, check if the module contains embedded DWARF
	p, err := dwarf.NewParser(moduleData)
	if err == nil && p.HasDebugInfo() {
		return p, nil
	}

	// If module parsing failed for reasons other than missing debug info, return that error
	if err != nil && !errors.Is(err, dwarf.ErrNoDebugInfo) {
		return nil, err
	}

	// Module has no embedded DWARF. Compute module hash.
	sum := sha256.Sum256(moduleData)
	moduleHash := hex.EncodeToString(sum[:])

	// Search configurable directories for matching .dwp / .debug.wasm
	filePath, err := FindDetachedSymbolFile(searchDirs, moduleHash)
	if err != nil {
		return nil, fmt.Errorf("module has no embedded DWARF: %w", err)
	}

	debugData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read detached symbol file %q: %w", filePath, err)
	}

	// Load DWARF from detached file into parser
	return dwarf.NewParserWithDetached(moduleData, debugData)
}

// LoadDetachedDWARF searches searchDirs for a matching .dwp or .debug.wasm file
// keyed by moduleHash, and loads a DWARF parser from it.
func LoadDetachedDWARF(searchDirs []string, moduleHash string) (*dwarf.Parser, error) {
	filePath, err := FindDetachedSymbolFile(searchDirs, moduleHash)
	if err != nil {
		return nil, err
	}

	debugData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read detached symbol file %q: %w", filePath, err)
	}

	return dwarf.NewParser(debugData)
}

// LoadDetachedDWARFInto loads detached DWARF from a matching .dwp or .debug.wasm file
// into an existing parser.
func LoadDetachedDWARFInto(p *dwarf.Parser, searchDirs []string, moduleHash string) error {
	filePath, err := FindDetachedSymbolFile(searchDirs, moduleHash)
	if err != nil {
		return err
	}

	debugData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read detached symbol file %q: %w", filePath, err)
	}

	return p.LoadDetached(debugData)
}

// LoadDWARFForModuleFile loads DWARF symbols for a WASM file on disk,
// using embedded symbols if present, or searching searchDirs for detached
// symbols (.dwp / .debug.wasm) if stripped.
func LoadDWARFForModuleFile(wasmPath string, searchDirs []string) (*dwarf.Parser, error) {
	moduleData, err := os.ReadFile(wasmPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read module file %q: %w", wasmPath, err)
	}
	return LoadDWARFForModule(moduleData, searchDirs)
}

// DiscoverLocalSymbols scans for WASM and detached symbol files in the local target directory.
// It returns a map of WASM / module hashes to their absolute file paths.
func DiscoverLocalSymbols(projectRoot string) (map[string]string, error) {
	searchDir := filepath.Join(projectRoot, wasmTargetPath)
	found := make(map[string]string)

	if _, err := os.Stat(searchDir); os.IsNotExist(err) {
		return found, nil
	}

	files, err := os.ReadDir(searchDir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		fullPath := filepath.Join(searchDir, file.Name())

		// If it is a detached symbol file (.dwp or .debug.wasm), also index by extracted module hash
		if isDetachedSymbolFile(file.Name()) {
			if h := extractHashFromDetachedName(file.Name()); h != "" {
				if _, exists := found[h]; !exists {
					found[h] = fullPath
				}
			}
		}

		if strings.HasSuffix(file.Name(), ".wasm") {
			content, err := os.ReadFile(fullPath)
			if err != nil {
				continue
			}

			// Calculate SHA256 hash to match against contract bytecode
			hash := sha256.Sum256(content)
			hashStr := hex.EncodeToString(hash[:])
			found[hashStr] = fullPath
		}
	}

	return found, nil
}
