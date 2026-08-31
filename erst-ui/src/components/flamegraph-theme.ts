// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

// ─── Theme types ──────────────────────────────────────────────────────────────

/** The set of color modes the Flamegraph supports. */
export type ThemeMode = 'light' | 'dark' | 'system';

/** Resolved color palette applied to the flamegraph canvas. */
export interface FlamegraphTheme {
  /** Overall SVG background */
  background: string;
  /** Default text fill for labels */
  textColor: string;
  /** Border/outline color for frames */
  frameBorder: string;
  /** Background color of the info/status bar */
  infoBarBackground: string;
  /** Text color in the info/status bar */
  infoBarText: string;
  /** Highlight fill for search matches */
  searchHighlight: string;
  /** Stroke color for highlighted frames */
  searchHighlightStroke: string;
  /** Opacity applied to flame rectangles for visual softening */
  frameOpacity: number;
}

// ─── Built-in palettes ────────────────────────────────────────────────────────

/** Light-mode Flamegraph palette. */
export const LIGHT_THEME: FlamegraphTheme = {
  background: '#ffffff',
  textColor: '#1a1a2e',
  frameBorder: '#cccccc',
  infoBarBackground: '#f5f5f5',
  infoBarText: '#444444',
  searchHighlight: 'rgb(230, 100, 230)',
  searchHighlightStroke: '#a000a0',
  frameOpacity: 1,
};

/** Dark-mode Flamegraph palette (Catppuccin Mocha inspired). */
export const DARK_THEME: FlamegraphTheme = {
  background: '#1e1e2e',
  textColor: '#cdd6f4',
  frameBorder: '#45475a',
  infoBarBackground: '#181825',
  infoBarText: '#a6adc8',
  searchHighlight: 'rgb(245, 194, 231)',
  searchHighlightStroke: '#f5c2e7',
  frameOpacity: 0.92,
};

// ─── Theme utilities ──────────────────────────────────────────────────────────

/**
 * Resolve a ThemeMode to its concrete FlamegraphTheme. When mode is "system"
 * the caller provides the result of the prefers-color-scheme media query so
 * the resolver stays pure and easy to test.
 */
export function resolveTheme(
  mode: ThemeMode,
  systemPrefersDark: boolean,
): FlamegraphTheme {
  switch (mode) {
    case 'dark':
      return DARK_THEME;
    case 'light':
      return LIGHT_THEME;
    case 'system':
    default:
      return systemPrefersDark ? DARK_THEME : LIGHT_THEME;
  }
}

/**
 * Build CSS that applies the resolved theme tokens as CSS custom properties on
 * the SVG root. Returned as a string for injection into a <style> element
 * inside the SVG <defs>.
 */
export function buildThemeCSS(theme: FlamegraphTheme): string {
  return [
    ':root {',
    `  --fg-background: ${theme.background};`,
    `  --fg-text: ${theme.textColor};`,
    `  --fg-frame-border: ${theme.frameBorder};`,
    `  --fg-info-bar-bg: ${theme.infoBarBackground};`,
    `  --fg-info-bar-text: ${theme.infoBarText};`,
    `  --fg-search-highlight: ${theme.searchHighlight};`,
    `  --fg-search-stroke: ${theme.searchHighlightStroke};`,
    `  --fg-frame-opacity: ${theme.frameOpacity};`,
    '}',
    'svg {',
    '  background-color: var(--fg-background);',
    '}',
    'text {',
    '  fill: var(--fg-text);',
    '}',
    'rect[data-highlighted="true"] {',
    '  fill: var(--fg-search-highlight) !important;',
    '  stroke: var(--fg-search-stroke) !important;',
    '  stroke-width: 2px !important;',
    '  paint-order: stroke fill;',
    '}',
    'rect[fill] {',
    '  opacity: var(--fg-frame-opacity);',
    '}',
  ].join('\n');
}
