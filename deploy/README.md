# Deployment

The current VPS uses one Caddy reverse proxy and a dedicated Blotato container. Caddy removes `/blotato` before forwarding requests to the service.

| Host path | Purpose |
|---|---|
| `<root>/apps/blotato/blotato` | Replaceable static Linux amd64 binary |
| `<root>/apps/blotato/.env` | Runtime credentials, mode `0600` |
| `<root>/data/blotato` | Persistent SQLite volume |
| `<root>/infra` | Shared Compose and Caddy configuration |

Container settings:

```dotenv
BLOTATO_MODE=instagram
BLOTATO_LISTEN=0.0.0.0:8080
BLOTATO_DB=/data/blotato.db
SWAGGER_BASE_PATH=/blotato
```

Set the remaining variables from [instagram.env.example](instagram.env.example). Unified Login uses `https://graph.facebook.com/v26.0`; `OPENROUTER_API_KEY` is required even when Jev is disabled. The delivery link belongs to a workflow rule, not an environment variable.

Follow [RELEASE.md](RELEASE.md) to deploy, verify and roll back. The release script lives in the separate infrastructure checkout at `craft/apps/infra/blotato/release.sh`; it is not included in this repository.

[Recorded release](../docs/tasks/2026-1029-plan-release-v0.1.md) · [API and local checks](../docs/tasks/2026-1029-plan-verification.md)
