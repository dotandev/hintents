# Flamegraph Quick Start

## TL;DR

```bash
# Generate interactive HTML flamegraph (default)
erst debug --profile <tx-hash>

# Open the generated file in your browser
open <tx-hash>.flamegraph.html
```

## Commands

| Command | Output | Description |
|---------|--------|-------------|
| `erst debug --profile <tx>` | `.flamegraph.html` | Interactive HTML (default) |
| `erst debug --profile --profile-format html <tx>` | `.flamegraph.html` | Interactive HTML (explicit) |
| `erst debug --profile --profile-format svg <tx>` | `.flamegraph.svg` | Raw SVG with dark mode |

## Interactive Features

| Action | How To |
|--------|--------|
| **View details** | Hover over any frame |
| **Zoom in** | Click on a frame |
| **Zoom out** | Click "Reset Zoom" button |
| **Search** | Type in search box, press Enter or click "Search" |
| **Clear search** | Click "Clear" button |
| **Dark mode** | Automatically adapts to system theme |

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Enter` | Search (when in search box) |
| `Esc` | Clear search box |

## What You'll See

The flamegraph shows:
- **Width** = Time/resources consumed (wider = more expensive)
- **Height** = Call stack depth (higher = deeper nesting)
- **Color** = Different functions (no semantic meaning)
- **Tooltip** = Function name, file, duration, percentage

## Example Workflow

1. **Run profiling**:
   ```bash
   erst debug --profile abc123def456
   ```

2. **Open HTML file**:
   ```bash
   open abc123def456.flamegraph.html
   ```

3. **Explore**:
   - Hover to see which functions are expensive
   - Click to zoom into hot paths
   - Search for specific function names
   - Look for wide bars (performance bottlenecks)

## Troubleshooting

### No flamegraph generated?
- Ensure `--profile` flag is set
- Check that simulation completed successfully
- Look for error messages in output

### HTML file won't open?
- Try a different browser (Chrome, Firefox, Safari)
- Check file permissions
- Ensure file wasn't corrupted during transfer

### Interactive features not working?
- Enable JavaScript in your browser
- Check browser console for errors
- Try a modern browser (Chrome 88+, Firefox 78+, Safari 14+)

### Dark mode not working?
- Check system theme settings
- Try toggling system dark mode
- Some browsers may not support `prefers-color-scheme`

## Tips

- **Wide bars** = Performance bottlenecks (investigate these first)
- **Tall stacks** = Deep call chains (may indicate recursion)
- **Search** = Find specific functions across the entire trace
- **Zoom** = Focus on specific code paths
- **Compare** = Generate flamegraphs for different transactions to compare

## Learn More

- [Full Documentation](INTERACTIVE_FLAMEGRAPH.md)
- [Live Demo](examples/sample_flamegraph.html)
- [Flamegraph Concepts](https://www.brendangregg.com/flamegraphs.html)
