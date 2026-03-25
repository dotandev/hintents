# PR Title
Refactor config lifecycle into parse/default/validate phases

## Summary
- split configuration loading into lifecycle phases: parsing, default assignment, and validation
- introduced interface-based validators so each validation rule is isolated
- added unit tests for missing required fields, invalid types, and out-of-bounds timeout values

## Validation
- `go test ./internal/config -count=1`

## Evidence Attachment
![Attach proof screenshot here](ATTACHMENT_PATH_OR_URL)

Replace `ATTACHMENT_PATH_OR_URL` with your uploaded image link.

## How to add the attachment
1. Create or collect your proof image (for example, a test run screenshot).
2. In GitHub PR description, drag and drop the image into the text editor.
3. GitHub will upload it and insert a markdown image URL.
4. Copy that URL and replace `ATTACHMENT_PATH_OR_URL` above.
