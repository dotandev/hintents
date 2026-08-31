// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

/**
 * Tests for Flamegraph theme utilities.
 *
 * The React component itself is not rendered here because the root jest
 * configuration uses a Node test environment (no DOM). Instead, we exercise
 * the pure exported helpers — resolveTheme and buildThemeCSS — which contain
 * the entirety of the theme logic.
 */

import {
  resolveTheme,
  buildThemeCSS,
  LIGHT_THEME,
  DARK_THEME,
  ThemeMode,
  FlamegraphTheme,
} from '../erst-ui/src/components/flamegraph-theme';

// ─── resolveTheme ──────────────────────────────────────────────────────────────

describe('resolveTheme', () => {
  describe('explicit "light" mode', () => {
    it('returns LIGHT_THEME regardless of system preference', () => {
      expect(resolveTheme('light', false)).toBe(LIGHT_THEME);
      expect(resolveTheme('light', true)).toBe(LIGHT_THEME);
    });
  });

  describe('explicit "dark" mode', () => {
    it('returns DARK_THEME regardless of system preference', () => {
      expect(resolveTheme('dark', false)).toBe(DARK_THEME);
      expect(resolveTheme('dark', true)).toBe(DARK_THEME);
    });
  });

  describe('"system" mode follows prefers-color-scheme', () => {
    it('returns LIGHT_THEME when system prefers light', () => {
      const theme = resolveTheme('system', false);
      expect(theme).toBe(LIGHT_THEME);
    });

    it('returns DARK_THEME when system prefers dark', () => {
      const theme = resolveTheme('system', true);
      expect(theme).toBe(DARK_THEME);
    });
  });

  describe('default mode falls through to system behavior', () => {
    it('treats an unknown mode as system and follows preference', () => {
      // TypeScript prevents passing arbitrary strings, but cast to exercise
      // the default branch for robustness.
      const unknownMode = 'unknown' as ThemeMode;
      expect(resolveTheme(unknownMode, true)).toBe(DARK_THEME);
      expect(resolveTheme(unknownMode, false)).toBe(LIGHT_THEME);
    });
  });
});

// ─── Theme palette contracts ───────────────────────────────────────────────────

describe('LIGHT_THEME palette', () => {
  it('has a white or near-white background', () => {
    expect(LIGHT_THEME.background).toBe('#ffffff');
  });

  it('has a dark text color for contrast', () => {
    // Text should be dark (low luminance) on a white background.
    expect(LIGHT_THEME.textColor).toMatch(/^#[0-3]/);
  });

  it('has frameOpacity of exactly 1 (no dimming in light mode)', () => {
    expect(LIGHT_THEME.frameOpacity).toBe(1);
  });

  it('has all required fields defined', () => {
    assertThemeComplete(LIGHT_THEME);
  });
});

describe('DARK_THEME palette', () => {
  it('has a dark background', () => {
    expect(DARK_THEME.background).toBe('#1e1e2e');
  });

  it('has a light text color for contrast', () => {
    // Text should start with #c or higher (high luminance hex).
    expect(DARK_THEME.textColor).toMatch(/^#[cdefCDEF]/);
  });

  it('has frameOpacity < 1 to soften frames on a dark background', () => {
    expect(DARK_THEME.frameOpacity).toBeGreaterThan(0);
    expect(DARK_THEME.frameOpacity).toBeLessThan(1);
  });

  it('has all required fields defined', () => {
    assertThemeComplete(DARK_THEME);
  });
});

// ─── buildThemeCSS ────────────────────────────────────────────────────────────

describe('buildThemeCSS', () => {
  describe('with LIGHT_THEME', () => {
    let css: string;
    beforeEach(() => {
      css = buildThemeCSS(LIGHT_THEME);
    });

    it('includes the light background custom property', () => {
      expect(css).toContain(`--fg-background: ${LIGHT_THEME.background}`);
    });

    it('includes the light text color custom property', () => {
      expect(css).toContain(`--fg-text: ${LIGHT_THEME.textColor}`);
    });

    it('includes svg background-color rule', () => {
      expect(css).toContain('background-color: var(--fg-background)');
    });

    it('includes text fill rule', () => {
      expect(css).toContain('fill: var(--fg-text)');
    });

    it('includes search-highlight fill rule', () => {
      expect(css).toContain('fill: var(--fg-search-highlight)');
    });

    it('includes search-highlight stroke rule', () => {
      expect(css).toContain('stroke: var(--fg-search-stroke)');
    });

    it('includes frame opacity rule', () => {
      expect(css).toContain('opacity: var(--fg-frame-opacity)');
    });

    it('embeds the correct frameOpacity value', () => {
      expect(css).toContain(`--fg-frame-opacity: ${LIGHT_THEME.frameOpacity}`);
    });
  });

  describe('with DARK_THEME', () => {
    let css: string;
    beforeEach(() => {
      css = buildThemeCSS(DARK_THEME);
    });

    it('includes the dark background custom property', () => {
      expect(css).toContain(`--fg-background: ${DARK_THEME.background}`);
    });

    it('includes the dark text color custom property', () => {
      expect(css).toContain(`--fg-text: ${DARK_THEME.textColor}`);
    });

    it('embeds the correct frameOpacity value', () => {
      expect(css).toContain(`--fg-frame-opacity: ${DARK_THEME.frameOpacity}`);
    });

    it('uses the dark search highlight color', () => {
      expect(css).toContain(
        `--fg-search-highlight: ${DARK_THEME.searchHighlight}`,
      );
    });

    it('uses the dark search stroke color', () => {
      expect(css).toContain(
        `--fg-search-stroke: ${DARK_THEME.searchHighlightStroke}`,
      );
    });
  });

  describe('CSS structure', () => {
    it('produces non-empty CSS for any valid theme', () => {
      expect(buildThemeCSS(LIGHT_THEME).length).toBeGreaterThan(0);
      expect(buildThemeCSS(DARK_THEME).length).toBeGreaterThan(0);
    });

    it('contains a :root block', () => {
      expect(buildThemeCSS(LIGHT_THEME)).toContain(':root');
      expect(buildThemeCSS(DARK_THEME)).toContain(':root');
    });

    it('contains all six expected custom properties', () => {
      const css = buildThemeCSS(LIGHT_THEME);
      const properties = [
        '--fg-background',
        '--fg-text',
        '--fg-frame-border',
        '--fg-info-bar-bg',
        '--fg-info-bar-text',
        '--fg-search-highlight',
        '--fg-search-stroke',
        '--fg-frame-opacity',
      ];
      for (const prop of properties) {
        expect(css).toContain(prop);
      }
    });

    it('targets rect[data-highlighted="true"] for search highlights', () => {
      const css = buildThemeCSS(LIGHT_THEME);
      expect(css).toContain('rect[data-highlighted="true"]');
    });
  });
});

// ─── resolveTheme + buildThemeCSS integration ─────────────────────────────────

describe('resolveTheme + buildThemeCSS integration', () => {
  it('system dark preference produces dark CSS variables', () => {
    const theme = resolveTheme('system', true);
    const css = buildThemeCSS(theme);
    expect(css).toContain(`--fg-background: ${DARK_THEME.background}`);
  });

  it('system light preference produces light CSS variables', () => {
    const theme = resolveTheme('system', false);
    const css = buildThemeCSS(theme);
    expect(css).toContain(`--fg-background: ${LIGHT_THEME.background}`);
  });

  it('explicit dark mode always produces dark CSS regardless of system', () => {
    const css1 = buildThemeCSS(resolveTheme('dark', false));
    const css2 = buildThemeCSS(resolveTheme('dark', true));
    expect(css1).toContain(`--fg-background: ${DARK_THEME.background}`);
    expect(css2).toContain(`--fg-background: ${DARK_THEME.background}`);
    expect(css1).toBe(css2);
  });

  it('explicit light mode always produces light CSS regardless of system', () => {
    const css1 = buildThemeCSS(resolveTheme('light', false));
    const css2 = buildThemeCSS(resolveTheme('light', true));
    expect(css1).toContain(`--fg-background: ${LIGHT_THEME.background}`);
    expect(css2).toContain(`--fg-background: ${LIGHT_THEME.background}`);
    expect(css1).toBe(css2);
  });
});

// ─── Helpers ──────────────────────────────────────────────────────────────────

/** Assert that every required field of a FlamegraphTheme is a non-empty string
 *  (or, for frameOpacity, a number in (0, 1]). */
function assertThemeComplete(theme: FlamegraphTheme): void {
  const stringFields: (keyof FlamegraphTheme)[] = [
    'background',
    'textColor',
    'frameBorder',
    'infoBarBackground',
    'infoBarText',
    'searchHighlight',
    'searchHighlightStroke',
  ];
  for (const field of stringFields) {
    expect(typeof theme[field]).toBe('string');
    expect((theme[field] as string).length).toBeGreaterThan(0);
  }
  expect(typeof theme.frameOpacity).toBe('number');
  expect(theme.frameOpacity).toBeGreaterThan(0);
  expect(theme.frameOpacity).toBeLessThanOrEqual(1);
}
