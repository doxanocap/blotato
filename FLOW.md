# Working approach

Ship a small backend, deploy it on the existing host and provide a reproducible demo. Track live Instagram access separately.

## Principles

1. Make it work, remove unnecessary parts, then optimize. Start with one process, SQLite and one workflow.
2. Verify the complete local flow. A real two-account Instagram demo additionally requires Meta access and actual event delivery.
3. Use the existing hosting and domain. Check HTTPS, persistent data and platform-access requirements early.
4. Work in short iterations with clear ownership and a concrete result. Resolve routine issues without adding infrastructure or endless research.
5. Commit only after approval. Preserve real development history and dates.
6. The user defines scope, decides trade-offs and checks results; agents write code. Summarize this honestly in README.

## Documents and ownership

- [Plan](docs/tasks/2026-1028-plan-start.md): assignment, architecture and decisions.
- [Init](docs/tasks/2026-1028-init.md): delivery sequence and current state.
- [Verification](docs/tasks/2026-1029-plan-verification.md): commands, expected results and Jev checks.
- [Release](docs/tasks/2026-1029-plan-release-v0.1.md): deployed artifact and remaining gaps.

The lead prepares assignments and reviews read-only. Workers own their assigned files; the user passes tasks and results. Independent work runs in parallel, with synchronization before the next dependent iteration. Do not start agents autonomously.

A worker returns `DELIVERED` with files and verification, or `BLOCKED` with a specific missing input. Keep tokens, real messages and personal data out of Git and logs.
