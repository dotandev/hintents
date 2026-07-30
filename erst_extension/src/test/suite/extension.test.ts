// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import * as assert from 'assert';
import * as vscode from 'vscode';

suite('ERST Extension', () => {
    const EXTENSION_ID = 'hintents.erst-vscode';

    test('Extension is present', () => {
        const ext = vscode.extensions.getExtension(EXTENSION_ID);
        assert.ok(ext, 'Extension should be registered');
    });

    test('Extension activates without error', async () => {
        const ext = vscode.extensions.getExtension(EXTENSION_ID);
        assert.ok(ext, 'Extension must be present to activate');
        await ext!.activate();
        assert.ok(ext!.isActive, 'Extension should be active after activation');
    });

    test('Commands are registered', async () => {
        const commands = await vscode.commands.getCommands(true);
        const expectedCommands = [
            'erst.triggerDebug',
            'erst.setTraceSearchQuery',
            'erst.exportTraceTree',
            'erst.selectTraceStep',
            'erst.showXdr',
            'erst.showStateDiff',
            'erst.nextTraceStep',
            'erst.prevTraceStep',
        ];
        for (const cmd of expectedCommands) {
            assert.ok(commands.includes(cmd), `${cmd} should be registered`);
        }
    });

    test('ERST trace view is registered', () => {
        const ext = vscode.extensions.getExtension(EXTENSION_ID);
        assert.ok(ext, 'Extension must be present');
        const contributes = ext!.packageJSON.contributes;
        assert.ok(contributes, 'Extension should have contributes section');
        const views = contributes.views;
        assert.ok(views, 'Extension should contribute views');
        assert.ok(views['erst-explorer'], 'Extension should contribute views for erst-explorer container');
        const erstViews = views['erst-explorer'];
        assert.ok(
            erstViews.some((v: { id: string }) => v.id === 'erst-traces'),
            'erst-traces view should be registered in erst-explorer'
        );
    });
});
