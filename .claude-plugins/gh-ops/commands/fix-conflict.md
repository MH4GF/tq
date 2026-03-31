---
description: Resolve merge conflicts on a PR
argument-hint: "<PR_URL>"
allowed-tools: Bash(gh pr view *), Bash(gh pr checkout *), Bash(git fetch *), Bash(git add *), Read, Edit, Grep, Glob
---

# Resolve Merge Conflicts

Resolve merge conflicts on a PR branch.

## Steps

### 1. Check conflict status and reviews

```bash
gh pr view $ARGUMENTS --json headRefName,baseRefName,mergeable,mergeStateStatus,reviews --jq '{headRefName,baseRefName,mergeable,mergeStateStatus,humanReviews:[.reviews[]|select(.author.is_bot==false and .authorAssociation!="NONE")]}'
```

Determine the resolution strategy:
- **Human reviews exist** → use merge (Step 2a). Rewriting history loses review context.
- **No human reviews** → use rebase (Step 2b). Keeps history clean.

### 2a. Resolve via merge (human reviews exist)

```bash
gh pr checkout $ARGUMENTS
git fetch origin
git merge origin/<baseRefName>
```

Resolve conflicts in each file, then:

```bash
git add <resolved files>
git merge --continue
git push
```

### 2b. Resolve via rebase (no human reviews)

```bash
gh pr checkout $ARGUMENTS
git fetch origin
git rebase origin/<baseRefName>
```

Resolve conflicts in each file, then:

```bash
git add <resolved files>
git rebase --continue
git push --force-with-lease
```

### 3. Verify

```bash
gh pr view $ARGUMENTS --json mergeable,mergeStateStatus
```

Confirm mergeable is "MERGEABLE".
