// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

import React, { useEffect, useState, useCallback } from 'react';
import {
  ThemeMode,
  FlamegraphTheme,
  LIGHT_THEME,
  DARK_THEME,
  resolveTheme,
  buildThemeCSS,
} from './flamegraph-theme';

// Re-export types and utilities so consumers can import them from this module.
export type { ThemeMode, FlamegraphTheme };
export { LIGHT_THEME, DARK_THEME, resolveTheme, buildThemeCSS };

// ─── useColorScheme hook ──────────────────────────────────────────────────────

/**
 * React hook that tracks the OS/browser color-scheme preference.
 * Returns `true` when the system prefers dark mode; `false` otherwise.
 * Falls back to `false` in environments where matchMedia is unavailable.
 */
function useColorScheme(): boolean {
  const getQuery = () =>
    typeof window !== 'undefined' && typeof window.matchMedia === 'function'
      ? window.matchMedia('(prefers-color-scheme: dark)')
      : null;

  const [prefersDark, setPrefersDark] = useState<boolean>(() => {
    const q = getQuery();
    return q ? q.matches : false;
  });

  useEffect(() => {
    const q = getQuery();
    if (!q) return;
    const handler = (e: MediaQueryListEvent) => setPrefersDark(e.matches);
    q.addEventListener('change', handler);
    return () => q.removeEventListener('change', handler);
  }, []);

  return prefersDark;
}

// ─── Component ────────────────────────────────────────────────────────────────

export interface FlamegraphProps {
  /** Maximum number of stack frames, used to compute canvas height. */
  maxStackDepth: number;
  /**
   * Color theme to use.
   * - "light"  – always use the light palette.
   * - "dark"   – always use the dark palette.
   * - "system" – (default) follow the OS/browser preference via
   *              `prefers-color-scheme`. Automatically re-renders when the
   *              user toggles their OS appearance.
   */
  theme?: ThemeMode;
  /** Callback fired when the user manually cycles the theme toggle. */
  onThemeChange?: (mode: ThemeMode) => void;
  /** Optional SVG content to render inside the flamegraph canvas. */
  children?: React.ReactNode;
}

/**
 * Flamegraph renders a performance flamegraph inside an SVG canvas that
 * automatically adapts to the user's dark/light mode preference.
 *
 * Theme precedence:
 *   explicit prop  >  system preference  >  light (default)
 *
 * When `theme="system"` (the default) the component listens for
 * `prefers-color-scheme` media-query changes and re-renders without a page
 * reload.
 *
 * @example
 * // Follows the OS setting automatically:
 * <Flamegraph maxStackDepth={20} />
 *
 * @example
 * // Always dark:
 * <Flamegraph maxStackDepth={20} theme="dark" />
 *
 * @example
 * // Let the user switch manually and receive the new mode:
 * <Flamegraph maxStackDepth={20} onThemeChange={(mode) => console.log(mode)} />
 */
export const Flamegraph: React.FC<FlamegraphProps> = ({
  maxStackDepth,
  theme: themeProp = 'system',
  onThemeChange,
  children,
}) => {
  const systemPrefersDark = useColorScheme();
  const [manualTheme, setManualTheme] = useState<ThemeMode>(themeProp);

  // Keep local state in sync when the parent changes the prop.
  useEffect(() => {
    setManualTheme(themeProp);
  }, [themeProp]);

  const resolvedTheme = resolveTheme(manualTheme, systemPrefersDark);
  const isDark = resolvedTheme === DARK_THEME;

  // Dynamically calculate canvas height based on max stack depth to prevent
  // truncation (20px per frame row + 50px for headers/labels).
  const canvasHeight = maxStackDepth * 20 + 50;

  const handleThemeToggle = useCallback(() => {
    // Cycle: system -> light -> dark -> system
    const next: ThemeMode =
      manualTheme === 'system'
        ? 'light'
        : manualTheme === 'light'
          ? 'dark'
          : 'system';
    setManualTheme(next);
    onThemeChange?.(next);
  }, [manualTheme, onThemeChange]);

  return (
    <div
      data-testid="flamegraph-wrapper"
      data-theme={manualTheme}
      data-resolved-theme={isDark ? 'dark' : 'light'}
      style={{ display: 'inline-block' }}
    >
      <svg
        height={canvasHeight}
        style={{ backgroundColor: resolvedTheme.background }}
        data-testid="flamegraph-svg"
        aria-label="Flamegraph visualization"
        role="img"
      >
        <defs>
          <style>{buildThemeCSS(resolvedTheme)}</style>
        </defs>
        {children}
      </svg>

      {/* Theme toggle button — visually minimal, keyboard accessible */}
      <button
        type="button"
        aria-label={`Switch flamegraph theme (current: ${manualTheme})`}
        data-testid="flamegraph-theme-toggle"
        onClick={handleThemeToggle}
        style={{
          display: 'block',
          marginTop: '4px',
          padding: '4px 10px',
          fontSize: '12px',
          cursor: 'pointer',
          background: resolvedTheme.infoBarBackground,
          color: resolvedTheme.infoBarText,
          border: `1px solid ${resolvedTheme.frameBorder}`,
          borderRadius: '4px',
        }}
      >
        {manualTheme === 'system'
          ? 'Theme: System'
          : manualTheme === 'dark'
            ? 'Theme: Dark'
            : 'Theme: Light'}
      </button>
    </div>
  );
};
