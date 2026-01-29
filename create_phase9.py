import os
import json
import urllib.request
import re

TOKEN = os.getenv("GITHUB_TOKEN")
REPO = "dotandev/hintents"
FILE_PATH = "/Users/khabibthekillys./.gemini/antigravity/brain/da66bbd6-6d6e-4c43-b3aa-7bbc51d12976/implementation_plan.md"

def create_issue(title, body):
    url = f"https://api.github.com/repos/{REPO}/issues"
    data = json.dumps({
        "title": title,
        "body": body,
        "labels": ["new_for_wave"]
    }).encode("utf-8")
    
    req = urllib.request.Request(url, data=data)
    req.add_header("Authorization", f"token {TOKEN}")
    req.add_header("Content-Type", "application/json")
    req.add_header("Accept", "application/vnd.github.v3+json")
    
    try:
        with urllib.request.urlopen(req) as response:
            res_data = json.loads(response.read().decode())
            print(f"Created issue #{res_data['number']}: {title}")
            return res_data['number']
    except Exception as e:
        print(f"Failed to create issue {title}: {e}")
        return None

def main():
    with open(FILE_PATH, "r") as f:
        content = f.read()

    # Find the Phase 9 section
    phase9_start = content.find("### Phase 9")
    if phase9_start == -1:
        print("Phase 9 not found")
        return
        
    phase9_content = content[phase9_start:]

    # Match issues in Phase 9 (starting from 87)
    issue_pattern = re.compile(r"#### (\d+)\. (.+?)\n(.+?)(?=\n#### \d+|$)", re.DOTALL)
    
    matches = issue_pattern.findall(phase9_content)
    print(f"Found {len(matches)} issues to create for Phase 9.")

    for num, title, body in matches:
        full_title = f"{num}. {title.strip()}"
        clean_body = body.strip()
        create_issue(full_title, clean_body)

if __name__ == "__main__":
    main()
