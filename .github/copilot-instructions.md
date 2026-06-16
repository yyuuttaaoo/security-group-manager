# Copilot Safety Instructions

This repository has recovered from accidental destructive git operations. Treat local changes and untracked files as user-owned work.

## Destructive Commands

Never run these commands unless the user explicitly asks for the exact operation and confirms the data loss risk in writing:

- `git reset --hard`
- `git clean -fd`
- `git clean -fdx`
- `git checkout -- .`
- `git restore .`
- `rm -rf`
- `find ... -delete`

Do not suggest bypassing permission prompts or shell guards for these operations.

## Before Risky Changes

Before any broad git, filesystem, dependency, formatting, or generated-code operation:

1. Run `git status --short --branch`.
2. Explain which files may be changed or deleted.
3. Prefer creating a recoverable checkpoint:
   - `git diff > /tmp/security-group-manager-wip.patch`
   - or ask the user to approve a temporary commit.
4. Ask for explicit confirmation before continuing.

## Recovery Bias

When local changes conflict with a task:

- Preserve the user's work.
- Do not revert unrelated changes.
- Prefer merging or making a backup patch over overwriting files.
- If unsure whether a file is user-created, assume it is.

## Secrets

Never commit real secrets, private keys, OAuth credentials, certificates, local config files, or deployment-only files. Keep these local:

- `config.yaml`
- `cert/`
- `bin/`
- `server.log`
- `docs/deploy_local.md`
