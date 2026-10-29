<!-- dox-sync:managed-v1 -->
# Blotato

Read [FLOW.md](FLOW.md) and [Init](docs/tasks/2026-1028-init.md) first. The [Plan](docs/tasks/2026-1028-plan-start.md) contains the original assignment and architecture.

- Ship a cheap, working MVP in short, verified iterations.
- `plan` describes the problem and solution; `init` tracks execution. Multiple taskfiles on one day are intentional in this project.
- Read existing code and respect component ownership. Agree shared contracts before parallel implementation.
- The lead discusses decisions, prepares assignments, inspects read-only and reconciles results. Workers implement changes; the user passes assignments between agents. Do not start agents autonomously.
- Complete only the assigned iteration, then wait for synchronization. Research runs alongside implementation.
- Commit only with explicit user approval; push requires authorization too. Stage only relevant changes, never secrets or unrelated work.
- Name applicable skills in worker assignments. Skills grant no additional permissions.
- Local tests, documentation and callback challenge checks do not prove live Instagram delivery.
- Preserve the original assignment verbatim. Describe AI use in README; do not create AI_USAGE.md.
