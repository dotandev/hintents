# Setting Up GitHub Discussions for Feature Voting

This guide explains how to enable and configure GitHub Discussions for the hintents repository to support the community feature voting process.

## Prerequisites

- Repository admin access to `dotandev/hintents`

## Step 1: Enable GitHub Discussions

1. Go to the repository: https://github.com/dotandev/hintents
2. Click on **Settings** (top navigation)
3. Scroll down to the **Features** section
4. Check the box next to **Discussions**
5. Click **Set up discussions** if prompted

## Step 2: Create the "Feature Requests" Category

Once Discussions are enabled:

1. Navigate to the **Discussions** tab in your repository
2. Click on the **Categories** section (usually visible in the sidebar or settings)
3. Click **New category** or **Edit categories**
4. Create a new category with:
   - **Name**: `Feature Requests`
   - **Description**: `Propose and vote on new features for Erst`
   - **Format**: Choose **Discussion** (allows threaded conversations)
   - **Emoji**: 💡 (or any icon you prefer)

## Step 3: Configure Category Settings (Optional)

You may want to create additional categories:

- **Q&A**: For questions and answers about using Erst
- **Show and tell**: For sharing projects built with Erst
- **General**: For general discussions about the project

## Step 4: Create a Pinned Discussion (Recommended)

Create a pinned discussion in the Feature Requests category:

**Title**: "How to Request and Vote on Features"

**Content**:
```markdown
# Welcome to Feature Requests! 🗳️

This is where the community shapes the future of Erst. Here's how it works:

## Requesting a Feature

1. **Search first**: Check if your idea already exists
2. **Create a new discussion**: Use a clear, descriptive title
3. **Explain the problem**: What are you trying to solve?
4. **Propose a solution**: How would you like it to work?

## Voting

- Use 👍 reactions on the original post to vote
- Features with the most votes get prioritized
- Avoid "+1" comments - use reactions instead!

## What Happens Next?

1. Maintainers review popular requests
2. Approved features become GitHub Issues
3. Issues are labeled with `feature` and `community-requested`
4. You can implement features yourself - just comment on the issue!

See [CONTRIBUTING.md](../blob/main/CONTRIBUTING.md#feature-requests--voting) for more details.
```

## Step 5: Update Repository Settings (Optional)

Consider enabling:
- **Require discussions to be resolved before closing** (Settings → Discussions)
- **Allow users to convert issues to discussions** (helps redirect feature requests)

## Step 6: Verify the Links

After setup, verify these links work:
- Main discussions: https://github.com/dotandev/hintents/discussions
- Feature requests category: https://github.com/dotandev/hintents/discussions/categories/feature-requests

## Alternative: Using Issues Instead

If you prefer not to use Discussions, you can use GitHub Issues with labels:

1. Create labels:
   - `feature-request` (for new feature proposals)
   - `community-requested` (for community-voted features)
   - `needs-votes` (for features awaiting community feedback)

2. Update the documentation to reference Issues instead of Discussions

3. Use issue templates to guide feature request submissions

## Need Help?

- [GitHub Docs: About Discussions](https://docs.github.com/en/discussions/quickstart)
- [GitHub Docs: Managing Categories](https://docs.github.com/en/discussions/managing-discussions-for-your-community/managing-categories-for-discussions)
