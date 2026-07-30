// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import * as path from 'node:path';
import * as vscode from 'vscode';
import { FlamegraphWebview } from '../views/flamegraphWebview';

export interface ProfileFlamegraphPaths {
    cpuPath: string;
    memoryPath: string;
    title?: string;
}

const supportedExtensions = new Set(['.html', '.htm', '.svg']);

/**
 * Opens generated CPU and memory flamegraphs in a tabbed VS Code webview.
 *
 * The extension's command registration can call this after the profiler writes
 * both files. Keeping the renderer behind this function avoids exposing VS Code
 * webview lifecycle details to the command.
 */
export async function showProfileFlamegraphs(
    paths: ProfileFlamegraphPaths,
    viewColumn: vscode.ViewColumn = vscode.ViewColumn.Beside
): Promise<void> {
    try {
        const [cpu, memory] = await Promise.all([
            validateFlamegraphPath('CPU', paths.cpuPath),
            validateFlamegraphPath('Memory', paths.memoryPath)
        ]);

        FlamegraphWebview.show(
            {
                cpu,
                memory,
                title: paths.title
            },
            viewColumn
        );
    } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        void vscode.window.showErrorMessage(`Unable to display flamegraphs: ${message}`);
    }
}

async function validateFlamegraphPath(
    label: string,
    filePath: string
): Promise<vscode.Uri> {
    const normalizedPath = filePath.trim();
    if (normalizedPath.length === 0) {
        throw new Error(`${label} flamegraph path is empty.`);
    }

    const extension = path.extname(normalizedPath).toLowerCase();
    if (!supportedExtensions.has(extension)) {
        throw new Error(
            `${label} flamegraph must be an HTML or SVG file: ${normalizedPath}`
        );
    }

    const uri = vscode.Uri.file(normalizedPath);
    let stat: vscode.FileStat;

    try {
        stat = await vscode.workspace.fs.stat(uri);
    } catch {
        throw new Error(`${label} flamegraph does not exist: ${normalizedPath}`);
    }

    if ((stat.type & vscode.FileType.File) === 0) {
        throw new Error(`${label} flamegraph is not a file: ${normalizedPath}`);
    }

    try {
        await vscode.workspace.fs.readFile(uri);
    } catch {
        throw new Error(`${label} flamegraph is not readable: ${normalizedPath}`);
    }

    return uri;
}
