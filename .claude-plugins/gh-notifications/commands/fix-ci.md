---
description: Fix CI failures on a PR
argument-hint: "<PR_URL>"
---

# Fix CI Failures

Investigate and fix CI failures on a PR.

## Steps

### 1. Identify failed checks

```bash
gh pr checks $ARGUMENTS
```

### 2. Get failure logs

```bash
gh run view <run_id> --log-failed
```

### 3. Identify root cause

Determine the cause from logs:
- Test failure → test code or implementation issue
- Lint failure → code style issue
- Build failure → compilation error, dependency issue
- Flaky test → re-run failed jobs: `gh run rerun <run_id> --failed`

### 4. Checkout branch

```bash
gh pr checkout $ARGUMENTS
```

### 5. Implement fix

Apply the appropriate fix and commit.

### 6. Push

```bash
git push
```

### 7. Verify CI

```bash
gh pr checks $ARGUMENTS --watch
```

Confirm all checks pass.
