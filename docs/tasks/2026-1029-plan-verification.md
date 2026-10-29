---
name: 2026-1029-plan-verification
title: "Blotato: verification guide"
description: >
  Reproduce the workflow and check the public API, Jev and live-integration boundaries.
status: active
branch: master
base: null
created_at: 2026-10-29
author: doxanocap
---

### Goal

1. Reproduce one complete run, duplicates and restart recovery.
2. Check Jev separately from the deterministic workflow and distinguish stub results from real-provider behavior.

### Scope

1. Run commands from the repository root with Go 1.26.4, cached Go dependencies, `curl` and Python 3. No package installation is required.
2. The default walkthrough uses temporary SQLite, fake sending and disabled Jev. Real Instagram delivery remains unverified. [Architecture](2026-1028-plan-start.md#schemas) · [Release](2026-1029-plan-release-v0.1.md).

<a id="public-checks"></a>

#### 1. Public API

```sh
curl -fsS -o /dev/null -w 'Swagger: %{http_code}\n' \
  https://kadirbekovvv.me/blotato/swagger/index.html
curl -fsS -o /dev/null -w 'Specification: %{http_code}\n' \
  https://kadirbekovvv.me/blotato/swagger/doc.json
curl -sS -o /dev/null -w 'Rules without token: %{http_code}\n' \
  https://kadirbekovvv.me/blotato/api/v1/workflows
curl -sS -o /dev/null -w 'Runs without token: %{http_code}\n' \
  https://kadirbekovvv.me/blotato/api/v1/workflows/runs
```

Expected status codes: `200, 200, 401, 401`. Swagger uses basePath `/blotato`. An administrator token supplied separately allows reading rules:

```sh
printf 'header = "Authorization: Bearer %s"\n' "$BLOTATO_ADMIN_TOKEN" |
  curl --config - -fsS https://kadirbekovvv.me/blotato/api/v1/workflows
```

Keep credentials out of shell tracing and logs. Rule creation changes the deployed database and needs the operator's agreement. `/events` and `/fake/outcome` exist only in local fake mode, even though the shared specification lists them.

<a id="local-start"></a>

#### 2. Start locally

In terminal 1:

```sh
umask 077
export BLOTATO_MODE=fake
export BLOTATO_DB="$(mktemp -d /tmp/blotato-demo.XXXXXX)/blotato.sqlite"
export BLOTATO_LISTEN=127.0.0.1:18087
export BLOTATO_EMAIL_INTENT_ENABLED=false
export OPENROUTER_API_KEY=local-demo-unused
export INSTAGRAM_ACCOUNT_ID=demo-brand
export INSTAGRAM_ACCESS_TOKEN=local-demo-unused
export INSTAGRAM_API_BASE=http://127.0.0.1:9
export INSTAGRAM_REQUEST_TIMEOUT=10s
export META_APP_SECRET=local-demo-unused
export META_VERIFY_TOKEN=local-demo-unused
export BLOTATO_ADMIN_TOKEN=
export SWAGGER_BASE_PATH=/
GOPROXY=off GOSUMDB=off go run ./cmd/api
```

The key is a synthetic nonempty value: startup requires `OPENROUTER_API_KEY`, but disabled Jev does not use it. Fake sending makes no platform requests. Explicit environment variables override `.env`; do not modify or source the real file.

Open http://127.0.0.1:18087/swagger/index.html. Stop with `Ctrl+C`. Start a fresh walkthrough with a new `mktemp` database; for a recovery check, retain the existing path.

<a id="full-scenario"></a>

#### 3. Complete the workflow

**Create a rule and send a comment.** In terminal 2:

```sh
export DEMO_URL=http://127.0.0.1:18087
curl -fsS "$DEMO_URL/swagger/doc.json" -o /dev/null

curl -fsS -i "$DEMO_URL/api/v1/workflows" \
  -H 'Content-Type: application/json' \
  -d '{"keyword":"pricing","public_reply":"Sent you a DM!","email_question":"What is your email address?","link":"https://example.com/pricing"}'

curl -fsS "$DEMO_URL/api/v1/workflows"

COMMENT='{"platform":"fake","account_id":"demo-brand","contact_id":"demo-person","kind":"comment","event_id":"demo-comment-1","text":"pricing"}'
curl -fsS -i "$DEMO_URL/events" \
  -H 'Content-Type: application/json' -d "$COMMENT"
```

Expect `201` for creation (`id: 1` in a fresh database), `200` for the rule list and `202` for the event. Event acceptance means persistence; the worker sends asynchronously.

Wait for `waiting_email`. This helper polls for up to 10 seconds:

```sh
wait_run() {
  python3 - "$DEMO_URL" "$1" "$2" <<'PY'
import json, sys, time, urllib.request
base, state, count = sys.argv[1], sys.argv[2], int(sys.argv[3])
for _ in range(50):
    with urllib.request.urlopen(base + '/api/v1/workflows/runs', timeout=2) as response:
        rows = json.load(response)
    if len(rows) == count and all(r['state'] == state and r['status'] == 'accepted' for r in rows):
        print(json.dumps(rows, indent=2))
        break
    time.sleep(0.2)
else:
    raise SystemExit('Expected state not reached: ' + json.dumps(rows))
PY
}
wait_run waiting_email 2
```

Expect two rows for the same run: `reply_comment` and `ask_email`, both accepted.

**Check recovery.** Stop terminal 1 with `Ctrl+C`, then rerun only `GOPROXY=off GOSUMDB=off go run ./cmd/api`. Retain its environment and `BLOTATO_DB`; do not repeat `mktemp`. In terminal 2, read the rule list and call `wait_run waiting_email 2` again. Both rule and wait must survive without new actions.

**Answer and receive the link.** In terminal 2:

```sh
MESSAGE="{\"platform\":\"fake\",\"account_id\":\"demo-brand\",\"contact_id\":\"demo-person\",\"kind\":\"message\",\"event_id\":\"demo-message-1\",\"text\":\"demo@example.com\",\"occurred_at\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}"
curl -fsS -i "$DEMO_URL/events" \
  -H 'Content-Type: application/json' -d "$MESSAGE"
wait_run completed 3
```

Expect `202`, then:

```json
[
  {"run_id":1,"state":"completed","error":"","step":"reply_comment","status":"accepted","action_error":""},
  {"run_id":1,"state":"completed","error":"","step":"ask_email","status":"accepted","action_error":""},
  {"run_id":1,"state":"completed","error":"","step":"send_link","status":"accepted","action_error":""}
]
```

These are three actions in one run. The response excludes email and message text. Fake `accepted` means the simulated sender succeeded; platform acceptance does not prove delivery or reading.

**Replay the same events:**

```sh
BEFORE=$(curl -fsS "$DEMO_URL/api/v1/workflows/runs")
curl -fsS -i "$DEMO_URL/events" -H 'Content-Type: application/json' -d "$COMMENT"
curl -fsS -i "$DEMO_URL/events" -H 'Content-Type: application/json' -d "$MESSAGE"
wait_run completed 3
AFTER=$(curl -fsS "$DEMO_URL/api/v1/workflows/runs")
test "$BEFORE" = "$AFTER" && printf 'OK: replay did not change actions\n'
```

Both replays return `202`; the state and three actions remain unchanged. The HTTP test below also asserts exactly three outgoing requests and two inbox events.

<a id="jev"></a>

#### 4. Jev email-intent classification

**Implemented, disabled by default, real-provider quality unmeasured.** Go extracts an email; Jev decides whether the sender offered it for use. Go retains control of recipients, links and transitions.

| Reply | Expected behavior |
|---|---|
| `anna@example.com` | Deterministic acceptance; no model call or decision row |
| `Write to anna@example.com` | Mask as `EMAIL_0`; affirmative classification may continue the run |
| `Do not use anna@example.com` | `reject`; keep waiting |
| `Maybe anna@example.com` | `uncertain`; keep waiting |
| `old@example.com or new@example.com` | Local `invalid`; no model call |
| Timeout, invalid JSON or provider error | Persist `error`; keep waiting |

These are target outcomes, not measured model answers. Acceptance requires choice `EMAIL_0`, confidence at least `0.95` and an open wait. The threshold is experimental. Other text still reaches OpenRouter after masking; use synthetic messages.

[Client](../../internal/clients/jev/client.go): `POST /api/alpha/decisions`, request model `typesafe/jev-1.14`, accepted snapshot `typesafe/jev-1.14-20261014`, instruction `email-intent-v1`, HTTP timeout 5 seconds. Decisions persist in `email_decisions`. [SQL guards](../../internal/repo/sqlite/email_decisions.go) prevent stale decisions or an accepted plain email from being overwritten.

**Stub verification, no paid calls:**

```sh
GOPROXY=off GOSUMDB=off go test -race -count=1 \
  -run '^(TestEmailDecisionHTTP|TestSlowEmailDecisionDoesNotBlockActionsOrExpiry|TestEmailDecisionConcurrentCleanAnswer|TestEmailDecisionTimeout|TestEarlyEmailDecision)$' \
  -v ./internal/domain/service/workflow
```

The tests cover acceptance/rejection, ambiguity, low confidence, malformed responses, timeout, multiple candidates, early replies and a competing plain email. A slow classifier must not block ordinary sends or expiry. This checks the Go integration, not model quality.

**Real-provider check, paid.** Proceed only when real OpenRouter calls are intended. Fake mode still simulates Instagram, but enabled Jev makes real model requests.

1. Stop terminal 1. Keep the local settings and the same SQLite path. Copy `BLOTATO_DB` for the read-only query below: `printf '%s\n' "$BLOTATO_DB"`. In Bash, enter the key privately and restart:

```bash
read -r -s -p 'OpenRouter key: ' OPENROUTER_API_KEY
printf '\n'
export OPENROUTER_API_KEY
export BLOTATO_EMAIL_INTENT_ENABLED=true
GOPROXY=off GOSUMDB=off go run ./cmd/api
```

2. In terminal 2, retain `DEMO_URL`. Use a new `CASE_ID` for every trial. This creates a new contact/run, waits for the question and sends a synthetic reply:

```sh
export CASE_ID=jev-accept-1
export CASE_TEXT='Write to anna@example.com'
python3 - <<'PY'
import json, os, time, datetime, urllib.request
base, case = os.environ['DEMO_URL'], os.environ['CASE_ID']
def get_runs():
    with urllib.request.urlopen(base + '/api/v1/workflows/runs', timeout=3) as r:
        return json.load(r)
def event(kind, text):
    data = dict(platform='fake', account_id='demo-brand', contact_id=case,
                kind=kind, event_id=case + '-' + kind, text=text)
    if kind == 'message':
        data['occurred_at'] = datetime.datetime.now(datetime.timezone.utc).isoformat()
    req = urllib.request.Request(base + '/events', data=json.dumps(data).encode(),
                              headers={'Content-Type': 'application/json'})
    with urllib.request.urlopen(req, timeout=3) as r:
        assert r.status == 202, r.status
before = {r['run_id'] for r in get_runs()}
event('comment', 'pricing')
for _ in range(50):
    rows = [r for r in get_runs() if r['run_id'] not in before]
    if len(rows) == 2 and all(r['state'] == 'waiting_email' and r['status'] == 'accepted' for r in rows):
        print('run_id:', rows[0]['run_id'])
        break
    time.sleep(0.2)
else:
    raise SystemExit('Question not confirmed; check rule or use a fresh CASE_ID')
event('message', os.environ['CASE_TEXT'])
print('Message saved. Inspect email_decisions and run state below.')
PY
```

3. Set `DEMO_DB` to terminal 1's actual `BLOTATO_DB` path. Read the local database while the app runs; do not use a production database. This query prints only technical fields:

```sh
export DEMO_DB=/absolute/path/to/the/local/demo/blotato.sqlite
python3 - <<'PY'
import json, os, pathlib, sqlite3, time
uri = pathlib.Path(os.environ['DEMO_DB']).resolve().as_uri() + '?mode=ro'
con = sqlite3.connect(uri, uri=True)
con.row_factory = sqlite3.Row
query = '''SELECT d.event_id, d.run_id, d.status, d.decision, d.model,
               d.instruction_version, d.confidence, r.state,
               (SELECT count(*) FROM actions a
                WHERE a.run_id=r.id AND a.step='send_link') AS link_actions
        FROM email_decisions d JOIN inbox i ON i.id=d.event_id
        JOIN runs r ON r.id=d.run_id WHERE i.event_id=?'''
for _ in range(50):
    rows = con.execute(query, (os.environ['CASE_ID'] + '-message',)).fetchall()
    if rows and rows[0]['status'] == 'done':
        print(json.dumps([dict(r) for r in rows], indent=2))
        break
    time.sleep(0.2)
else:
    print('No completed decision: clean email creates no row; otherwise check flag and local process.')
con.close()
PY
curl -fsS "$DEMO_URL/api/v1/workflows/runs"
```

An affirmative result can create one link action and reach `completed`; the query may first show `link_pending`. Rejection, ambiguity, invalid input and errors leave the run waiting. A late result is saved but cannot change a closed run. On error, `model` may contain the requested ID rather than a confirmed snapshot.

4. Stop the app and set `BLOTATO_EMAIL_INTENT_ENABLED=false`. Use `OPENROUTER_API_KEY=local-demo-unused` again only with the flag disabled.

The [14 synthetic RU/EN cases](../email-intent-cases.jsonl) are a small smoke set. Measure false acceptance, misses, latency and cost before choosing a production threshold. A completed decision is not retried; a crash before saving a pending decision can repeat inference and its charge. Additional direct-provider commands are in the [experiment notes](../email-intent-experiment.md).

<a id="evidence"></a>

#### 5. Results and remaining gaps

| Check | Recorded result |
|---|---|
| Full race suite | Worker-1 reported exit 0; [saved output](../assets/demo/tests.txt) |
| Signed HTTP workflow with simulated Meta | Worker-1: three sends, one run, two events, completed; duplicates and restart covered |
| Manual fake walkthrough | Worker-1: rule/wait survive restart; one run, three accepted actions, no Jev decisions |
| Deployment | Worker-2: public Swagger, protected API, rule persistence and exact challenge response; [release record](2026-1029-plan-release-v0.1.md) |
| Live Instagram | App Review unfinished; permissions, subscriptions and the two-account flow unverified |
| Real Jev | Disabled on VPS; behavior, quality, latency, cost and threshold calibration unmeasured |

Run the existing suite or the signed HTTP test:

```sh
GOPROXY=off GOSUMDB=off go test -race -count=1 ./...
GOPROXY=off GOSUMDB=off go test -race -count=1 \
  -run '^TestReleaseInstagramHTTPFlow$' -v ./internal/app/api
```

### Decisions

1. Keep all reproduction commands here; README and init link to this guide.
2. Use temporary state and fake sending for local checks. Enable real model calls deliberately.
3. Keep platform acceptance, real delivery and model quality separate. Unknown sends are not automatically retried.

### Changes

1. Consolidated the validated walkthrough, public API checks and Jev verification in one guide.
2. Preserved command behavior and stable anchors; shortened repeated status notes and translated the prose to English. No tests, services or paid calls were run during this edit.
3. Added [.example.env](../../.example.env) for local fake-mode setup; real credentials remain in the ignored `.env`.
