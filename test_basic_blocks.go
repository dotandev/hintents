package main

import (
	"fmt"
	"log"

	"github.com/dotandev/hintents/internal/wat"
	"github.com/Whiznificent/hintents/internal/wat"
)

func main() {
	// Test basic block analysis with a simple function
	// Function body: i32.const 1, i32.const 2, i32.add, drop
	body := []byte{
		0x41, 0x01, // i32.const 1
		0x41, 0x02, // i32.const 2
		0x6a, // i32.add
		0x1a, // drop
	}

	wasm := buildMinimalWasm(body)
	
	d := wat.NewDisassembler(wasm)
	
	// Test basic block analysis
	instructions, err := d.DecodeAll()
	if err != nil {
		log.Fatalf("DecodeAll failed: %v", err)
	}

	fmt.Printf("Decoded %d instructions:\n", len(instructions))
	for _, inst := range instructions {
		fmt.Printf("  0x%04x: %s\n", inst.Offset, inst.String())
	}

	// Test basic block analysis
	analysis := d.AnalyzeBasicBlocks(instructions)
	fmt.Printf("\nBasic Block Analysis:\n")
	fmt.Printf("Found %d basic blocks:\n", len(analysis.Blocks))
	
	for i, block := range analysis.Blocks {
		fmt.Printf("  Block %d: 0x%04x-0x%04x [%s]\n", i, block.StartOffset, block.EndOffset, block.BlockType)
		if block.IsJumpTarget {
			fmt.Printf("    [JUMP TARGET] - sources: %v\n", block.JumpSources)
		}
		fmt.Printf("    Instructions: %d\n", len(block.Instructions))
	}

	fmt.Printf("\nJump Targets:\n")
	for jumpOffset, targetOffset := range analysis.JumpTargets {
		fmt.Printf("  0x%04x -> 0x%04x\n", jumpOffset, targetOffset)
	}

	// Test with basic blocks formatting
	fmt.Printf("\n--- Testing with Basic Blocks Formatting ---\n")
	
	// Find the i32.add instruction
	var addOffset uint64
	for _, inst := range instructions {
		if inst.Mnemonic == "i32.add" {
			addOffset = inst.Offset
			break
		}
	}

	snippet, err := d.DisassembleAtWithBasicBlocks(addOffset, 2)
	if err != nil {
		log.Fatalf("DisassembleAtWithBasicBlocks failed: %v", err)
	}

	fmt.Printf("Formatted output with basic blocks:\n")
	fmt.Printf("%s\n", snippet.Format())
}

// Copy of the test helper functions
func buildMinimalWasm(functionBody []byte) []byte {
	// WASM header: magic + version
	module := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}

	// Type section: one function type () -> ()
	typeSection := []byte{
		0x01, // section id (Type)
		0x04, // section size
		0x01, // one type
		0x60, // func type
		0x00, // no params
		0x00, // no results
	}
	module = append(module, typeSection...)

	// Function section: one function referencing type 0
	funcSection := []byte{
		0x03, // section id (Function)
		0x02, // section size
		0x01, // one function
		0x00, // type index 0
	}
	module = append(module, funcSection...)

	// Code section
	// Function body = local decl count (0) + body bytes
	funcBody := append([]byte{0x00}, functionBody...) // 0 locals
	funcBody = append(funcBody, 0x0b)                 // end opcode

	funcBodyLen := encodeULEB128(uint64(len(funcBody)))
	codeSectionPayload := append([]byte{0x01}, funcBodyLen...) // 1 function
	codeSectionPayload = append(codeSectionPayload, funcBody...)

	codeSectionLen := encodeULEB128(uint64(len(codeSectionPayload)))
	codeSection := append([]byte{0x0a}, codeSectionLen...) // SectionCode = 10
	codeSection = append(codeSection, codeSectionPayload...)

	module = append(module, codeSection...)

	return module
}

func encodeULEB128(v uint64) []byte {
	if v == 0 {
		return []byte{0x00}
	}
	var result []byte
	for v > 0 {
		b := byte(v & 0x7f)
		v >>= 7
		if v > 0 {
			b |= 0x80
		}
		result = append(result, b)
	}
	return result
}
