// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import * as vscode from 'vscode';

export class SorobanCodeLensProvider implements vscode.CodeLensProvider {
    provideCodeLenses(document: vscode.TextDocument): vscode.CodeLens[] {
        if (!this.isRustContractDocument(document)) {
            return [];
        }

        const text = document.getText();
        const contractBlocks = this.findContractImplBlocks(document);
        const codeLenses: vscode.CodeLens[] = [];

        for (const block of contractBlocks) {
            const functionRegex = /^\s*(?:pub(?:\s*\([^)]*\))?\s+)?(?:async\s+)?fn\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(/gm;
            let match: RegExpExecArray | null;

            while ((match = functionRegex.exec(block.body)) !== null) {
                const absoluteOffset = block.startOffset + match.index;
                const startPosition = document.positionAt(absoluteOffset);
                const range = new vscode.Range(startPosition.line, 0, startPosition.line, 0);
                const command: vscode.Command = {
                    title: 'Run via ERST',
                    command: 'erst.runSorobanFunction',
                    arguments: [document.uri, match[1]]
                };

                codeLenses.push(new vscode.CodeLens(range, command));
            }
        }

        return codeLenses;
    }

    private isRustContractDocument(document: vscode.TextDocument): boolean {
        return document.languageId === 'rust' || document.fileName.toLowerCase().endsWith('.rs');
    }

    private findContractImplBlocks(document: vscode.TextDocument): Array<{ startOffset: number; body: string }> {
        const text = document.getText();
        const blocks: Array<{ startOffset: number; body: string }> = [];
        const attributePattern = /#\s*\[contractimpl\]/g;
        let attributeMatch: RegExpExecArray | null;

        while ((attributeMatch = attributePattern.exec(text)) !== null) {
            const attributeStart = attributeMatch.index;
            const implStart = this.findNextKeyword(text, attributeStart + attributeMatch[0].length, 'impl');
            if (implStart === -1) {
                continue;
            }

            const braceStart = this.findNextBrace(text, implStart);
            if (braceStart === -1) {
                continue;
            }

            let depth = 0;
            for (let index = braceStart; index < text.length; index += 1) {
                const char = text[index];
                if (char === '{') {
                    depth += 1;
                } else if (char === '}') {
                    depth -= 1;
                    if (depth === 0) {
                        blocks.push({
                            startOffset: braceStart + 1,
                            body: text.slice(braceStart + 1, index)
                        });
                        break;
                    }
                }
            }
        }

        return blocks;
    }

    private findNextKeyword(text: string, fromIndex: number, keyword: string): number {
        const searchText = text.slice(fromIndex);
        const matchIndex = searchText.indexOf(keyword);
        return matchIndex === -1 ? -1 : fromIndex + matchIndex;
    }

    private findNextBrace(text: string, fromIndex: number): number {
        return text.indexOf('{', fromIndex);
    }
}
