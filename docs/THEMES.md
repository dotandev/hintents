# Color-Blind Accessible Themes

Erst supports multiple color themes to ensure accessibility for users with color vision deficiencies.

## Available Themes

### `default`
Standard color palette using red, green, yellow, blue, cyan, and magenta.

### `deuteranopia`
Optimized for red-green color blindness (most common form). Uses blue, yellow, cyan, and magenta to avoid red-green confusion.

### `protanopia`
Optimized for red-blind color vision deficiency. Similar to deuteranopia, avoids red-green combinations.

### `tritanopia`
Optimized for blue-yellow color blindness. Uses red, green, and magenta to avoid blue-yellow confusion.

### `high-contrast`
Bold, high-contrast colors for improved visibility in all conditions. Useful for low-light environments or visual impairments.

## Usage

### Command Line Flag

Use the `--theme` flag with any command that produces colored output:

```bash
# Debug command with deuteranopia theme
erst debug <tx-hash> --theme deuteranopia

# Trace viewer with high-contrast theme
erst trace execution.json --theme high-contrast

# Protanopia theme
erst debug <tx-hash> --theme protanopia --network testnet
```

### Environment Variable

Set the `ERST_THEME` environment variable to apply a theme globally:

```bash
export ERST_THEME=deuteranopia
erst debug <tx-hash>
```

### Auto-Detection

If no theme is specified, Erst will auto-detect an appropriate theme:
- If `ERST_THEME` is set, use that theme
- If `COLORTERM=truecolor`, use default theme
- Otherwise, use high-contrast theme for maximum compatibility

## Color Mappings

### Default Theme
- Success: Green
- Error: Red
- Warning: Yellow
- Info: Blue

### Deuteranopia/Protanopia Themes
- Success: Cyan
- Error: Magenta
- Warning: Yellow
- Info: Blue

### Tritanopia Theme
- Success: Green
- Error: Red
- Warning: Magenta
- Info: Cyan

### High-Contrast Theme
- Success: Bold Green
- Error: Bold Red
- Warning: Bold Yellow
- Info: Bold Cyan

## Testing Your Theme

Use demo mode to test color output without network access:

```bash
erst debug --demo --theme deuteranopia
erst debug --demo --theme high-contrast
```

## Disabling Colors

To disable all colors (e.g., for logging or CI):

```bash
export NO_COLOR=1
erst debug <tx-hash>
```

Or force colors even in non-TTY environments:

```bash
export FORCE_COLOR=1
erst debug <tx-hash> | tee output.log
```

## Accessibility Best Practices

1. **Use semantic indicators**: Erst uses text indicators like `[OK]`, `[!]`, and `[X]` in addition to colors
2. **Test with different themes**: Verify your terminal output is readable with all themes
3. **Respect NO_COLOR**: Always honor the NO_COLOR environment variable
4. **Provide context**: Don't rely solely on color to convey information

## Contributing

When adding new colored output:
1. Use semantic color functions: `Success()`, `Error()`, `Warning()`, `Info()`
2. Test with all themes to ensure accessibility
3. Include text indicators alongside colors
4. Document color usage in code comments
