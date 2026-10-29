---
name: 2026-1029-plan-release-v0.1
title: "Blotato v0.1: release record"
description: >
  Record the deployed artifact, verification results and remaining live-integration gaps.
status: active
branch: master
base: null
created_at: 2026-10-29
author: doxanocap
---

### Goal

1. Deploy the MVP over HTTPS with persistent SQLite and authenticated rule management.
2. Record the released artifact and its verified boundaries.

### Scope

1. [Architecture](2026-1028-plan-start.md#schemas), [verification](2026-1029-plan-verification.md) and [deployment procedure](../../deploy/RELEASE.md).
2. Deployment completed on 2026-10-29. Live Instagram delivery and real Jev behavior remain unverified.

#### Released artifact

`data/artifacts/blotato-linux-amd64-unified`: statically linked Linux amd64, built with `CGO_ENABLED=0`.

```text
b1a1aa469dfd2cd3be9dfd1ece4aaad9bb3e764b7b1f40b7aeffe751dac912b0
```

The local hash was checked independently; worker-2 reported the same hash on the VPS after restart. Later repository changes do not automatically appear in this binary.

| Check | Reported result | Source |
|---|---|---|
| Race tests | Full suite passed | Worker-1 |
| Signed workflow against a simulated Meta API | Three sends, one run, two events, completed; duplicates and restart covered | Worker-1 |
| Swagger UI / specification | 200 / 200, basePath `/blotato` | Worker-2; public GETs also checked by research |
| Management without Bearer | 401 | Worker-2 |
| Rule create / read | 201 / 200; rule survives restart | Worker-2 |
| Callback challenge | Exact response, 200; wrong token and unsigned POST rejected with 403 | Worker-2 |
| Runtime | Protected environment, persistent SQLite, Unified Login host; Jev disabled | Worker-2 |

#### Meta status

Unified Login and a SUAT were configured. App Review reached the requirement for a verified business portfolio; the portfolio is unverified and the app unpublished. No rejection has been confirmed.

Token grants, both webhook subscriptions, Meta's acceptance of the callback, scoped-ID equivalence and real comment/message delivery are unverified. The server challenge check does not establish these results. See [Instagram integration](2026-1028-plan-start.md#instagram-integration).

#### Access and operation

- [Swagger UI](https://kadirbekovvv.me/blotato/swagger/index.html) and [specification](https://kadirbekovvv.me/blotato/swagger/doc.json) are public.
- `/blotato/api/v1/workflows` and `/blotato/api/v1/workflows/runs` require an administrator Bearer token supplied separately. Fake routes are local-only.
- [GitHub repository](https://github.com/doxanocap/blotato) was private at the last check; grant reviewer access or publish it before submission.
- SQLite uses a persistent volume. Disk loss, multiple replicas and manual resolution of `unknown` are not covered.

### Decisions

1. Use Unified Login, SUAT and `graph.facebook.com/v26.0`.
2. Keep Jev disabled until a deliberate real-provider check. A configured key is not evidence of model quality.
3. Rebuild and revalidate after product-code changes. Use the [release procedure](../../deploy/RELEASE.md) for upgrades and rollback.

### Changes

1. Published the unified binary and recorded its hash and deployment checks.
2. Separated completed deployment from unfinished Meta access and Jev evaluation.
