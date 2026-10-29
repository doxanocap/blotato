# Release and rollback

## Prepare

- Obtain a tested static Linux amd64 binary and its SHA-256. Check the hash before transfer.
- Create `<root>/apps/blotato/.env` on the VPS from [instagram.env.example](instagram.env.example), owned by root with mode `0600`.
- Required: `BLOTATO_ADMIN_TOKEN`, `INSTAGRAM_ACCOUNT_ID`, `INSTAGRAM_ACCESS_TOKEN`, `META_APP_SECRET`, `META_VERIFY_TOKEN`, `OPENROUTER_API_KEY`.
- Use `INSTAGRAM_API_BASE=https://graph.facebook.com/v26.0`. Keep Jev disabled unless its real-provider use is intended.
- Check free disk space and compare shared Compose/Caddy files before replacing them. SQLite stays in `<root>/data/blotato`, outside the binary directory.

## Deploy

From the separate `craft/apps/infra` checkout:

```sh
shasum -a 256 /absolute/path/to/verified-blotato-linux-amd64
BLOTATO_BIN=/absolute/path/to/verified-blotato-linux-amd64 ./blotato/release.sh
```

The script checks prerequisites, transfers the selected binary and configuration, saves `.previous` copies, recreates only Blotato and reloads Caddy. It preserves SQLite and other application containers. The next release overwrites the one rollback set.

## Verify

```sh
curl -fsS -o /dev/null -w 'swagger: %{http_code}\n' https://kadirbekovvv.me/blotato/swagger/index.html
curl -fsS https://kadirbekovvv.me/blotato/swagger/doc.json | python3 -c 'import json,sys; print("basePath:", json.load(sys.stdin)["basePath"])'
curl -sS -o /dev/null -w 'workflows without token: %{http_code}\n' https://kadirbekovvv.me/blotato/api/v1/workflows
curl -sS -o /dev/null -w 'unsigned webhook: %{http_code}\n' -X POST https://kadirbekovvv.me/blotato/webhook/instagram
```

Expect 200 for Swagger, basePath `/blotato`, 401 without an administrator token and 403 for the unsigned webhook. Swagger must load `/blotato/swagger/doc.json` and target `/blotato/api/v1/workflows`.

With administrator access, create a test rule and record its ID. From `<root>/infra` on the VPS, restart only Blotato, then read the rule again:

```sh
docker compose --env-file .env --profile blotato restart blotato
docker compose --env-file .env --profile blotato ps blotato
```

The rule must persist. Verify the exact callback challenge using the configured verify token without exposing it in logs. Check Meta subscriptions and real delivery separately; see the [verification guide](../docs/tasks/2026-1029-plan-verification.md).

## Roll back

From `<root>/infra` on the VPS:

```sh
docker compose --env-file .env --profile blotato stop blotato
cat Caddyfile.previous > Caddyfile
docker compose --env-file .env exec -T caddy caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile
cp docker-compose.yml.previous docker-compose.yml
```

For a first deployment, leave Blotato stopped. For an upgrade, confirm that the previous binary supports the current SQLite schema, then restore it:

```sh
cp -p ../apps/blotato/blotato.previous ../apps/blotato/blotato
docker compose --env-file .env --profile blotato up -d --no-deps --force-recreate blotato
```

Preserve `data/blotato` and `.env` in either case.
