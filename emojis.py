import os
import sys
import argparse

# Comprehensive list of slop emojis and their preferred text replacements
replacements = {
    "✅": "[OK]",
    "❌": "[FAIL]",
    "✓": "[OK]",
    "✗": "[FAIL]",
    "⚡": "[READY]",
    "🚀": "[START]",
    "🔍": "[SEARCH]",
    "➡️": "->",
    "⬅️": "<-",
    "🎯": "[TARGET]",
    "📍": "[LOC]",
    "🔧": "[TOOL]",
    "📊": "[STATS]",
    "📋": "[LIST]",
    "▶️": "[PLAY]",
    "📖": "[DOC]",
    "👋": "[HELLO]",
    "📡": "[NET]",
    "✨": "*",
    "🔥": "[CRITICAL]",
    "💡": "[INFO]",
    "🚧": "[WORK-IN-PROGRESS]",
    "🧪": "[TEST]",
    "🔒": "[SECURE]",
    "🔓": "[UNSECURE]",
    "🔗": "[LINK]",
    "🛠️": "[FIX]",
    "📦": "[PKG]",
    "🚀": "[DEPLOY]",
    "🚨": "[ALERT]",
    "🧹": "[CLEANUP]",
    "📝": "[LOG]",
    "🛡️": "[GUARD]",
    "🤖": "[BOT]",
    "🐛": "[BUG]",
    "🏷️": "[TAG]",
    "🎨": "[UI]",
    "🏁": "[DONE]",
    "🏥": "[HEALTH]",
    "🏠": "[HOME]",
    "🏗️": "[BUILD]",
    "🚢": "[SHIP]",
    "🧬": "[GEN]",
    "🧪": "[TEST]",
    "🌡️": "[METRIC]",
}

def clean_file(filepath, check_only=False):
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            content = f.read()
        
        new_content = content
        found_emojis = []
        for emoji, replacement in replacements.items():
            if emoji in content:
                found_emojis.append(emoji)
                new_content = new_content.replace(emoji, replacement)
        
        if content != new_content:
            if check_only:
                print(f"FAILED: Redundant emojis found in {filepath}: {', '.join(found_emojis)}")
                return True
            
            with open(filepath, 'w', encoding='utf-8') as f:
                f.write(new_content)
            print(f"FIXED: Updated {filepath}")
        return False
    except Exception as e:
        print(f"Error processing {filepath}: {e}")
        return False

def main():
    parser = argparse.ArgumentParser(description="Scan and remove redundant emojis and slop from the codebase.")
    parser.add_argument("path", nargs="?", default=".", help="Directory to scan (default: current directory)")
    parser.add_argument("--check", action="store_true", help="Exit with error code if emojis are found without modifying files")
    parser.add_argument("--fix", action="store_true", help="Automatically fix found emojis (default behavior if --check is not set)")
    
    args = parser.parse_args()
    
    is_check_mode = args.check and not args.fix
    found_any = False
    
    extensions = {".md", ".go", ".ts", ".js", ".tsx", ".jsx", ".toml", ".yml", ".yaml", ".json"}
    exclude_dirs = {".git", "node_modules", "vendor", "dist", "out", "coverage", ".next", ".vscode"}

    for root, dirs, files in os.walk(args.path):
        # Prune excluded directories
        dirs[:] = [d for d in dirs if d not in exclude_dirs]
        
        for file in files:
            if any(file.endswith(ext) for ext in extensions):
                filepath = os.path.join(root, file)
                if clean_file(filepath, check_only=is_check_mode):
                    found_any = True

    if is_check_mode and found_any:
        print("\nStatic check failed: Redundant emojis/slop detected in codebase.")
        print("Please run 'python emojis.py --fix' to clean the codebase.")
        sys.exit(1)
    
    if not found_any:
        print("No redundant emojis found.")

if __name__ == "__main__":
    main()
