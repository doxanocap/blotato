---
name: 2026-1029-plan-verification
title: "Blotato: как проверить решение"
description: >
  Воспроизвести локальный сценарий и проверить публичный сервис, Jev и границы живой интеграции.
status: active
branch: master
base: null
created_at: 2026-10-29
author: doxanocap
---

### Goal

1. Дать одну воспроизводимую инструкцию: правило → комментарий → вопрос → email → ссылка → completed, повторы и восстановление.
2. Отделить локальный backend и опубликованный API от ещё не подтверждённых настоящих Instagram и Jev.

### Scope

1. Команды предназначены для ручной проверки. При подготовке документа тесты, сервисы, аккаунты и платный API не запускались. Локальные команды перенесены из проверенного worker-1 материала; результаты его прогонов перечислены отдельно.
2. Архитектура: [plan](2026-1028-plan-start.md#schemas). Выпуск: [v0.1](2026-1029-plan-release-v0.1.md). Исходное ТЗ допускает считать платформенную интеграцию уже готовой; настоящий адаптер является дополнительной частью демонстрации.

<a id="public-checks"></a>

#### 1. Что можно проверить публично

| Адрес / операция | Что проверяется |
|---|---|
| [Swagger UI](https://kadirbekovvv.me/blotato/swagger/index.html) | Доступная по HTTPS документация API. |
| [Спецификация](https://kadirbekovvv.me/blotato/swagger/doc.json) | Swagger 2.0, basePath `/blotato`. Info.version `1.0` не доказывает версию бинарника. |
| `GET /blotato/api/v1/workflows` | Без административного Bearer ожидается 401; с предоставленным отдельно токеном возвращает правила. |
| `GET /blotato/api/v1/workflows/runs` | Аналогичная защита; состояния и действия без email/текстов. |
| `POST /blotato/api/v1/workflows` | Требует отдельно предоставленного Bearer, создаёт постоянное правило. Без согласования на публичном сервисе не выполнять. |

Проверки без секретов и изменения состояния:

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

Ожидаются `200, 200, 401, 401`. Первые два GET самостоятельно проверены research; авторизация подтверждена отчётом worker-2. С отдельно переданным `BLOTATO_ADMIN_TOKEN` в защищённом окружении можно прочитать правила без literal-секрета в аргументах:

```sh
printf 'header = "Authorization: Bearer %s"\n' "$BLOTATO_ADMIN_TOKEN" |
  curl --config - -fsS https://kadirbekovvv.me/blotato/api/v1/workflows
```

Это наш административный токен, не SUAT и не App Secret. Не включать shell tracing, не публиковать токен и не вводить секреты в публичный Swagger. Fake-раздел общей спецификации не означает доступность: `/events` и `/fake/outcome` на VPS в Instagram-режиме **не зарегистрированы**. Демо ниже выполняется только на loopback.

Нужны Go 1.26.4 (по go.mod), `curl`, Python 3 и зависимости Go в локальном кэше. Все команды выполняются из корня Blotato. Установка пакетов не нужна.

<a id="local-start"></a>

#### 2. Локальный запуск

В первом терминале:

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

`OPENROUTER_API_KEY` обязателен даже при выключенном Jev. Здесь используется вымышленная непустая строка: при `BLOTATO_EMAIL_INTENT_ENABLED=false` клиент модели не подключается. Fake-отправитель не выполняет HTTP-запросы; адрес `127.0.0.1:9` дополнительно исключает настоящий Meta endpoint в настройках демонстрации.

Настоящий `.env` не менять и не выполнять `source .env`. Приложение читает его как необязательные значения по умолчанию; явно заданные выше переменные имеют приоритет. SQLite создаётся отдельно от рабочей базы. Для повторения с нуля остановите процесс и повторите весь блок с новым `mktemp`.

Swagger: http://127.0.0.1:18087/swagger/index.html. Спецификация: http://127.0.0.1:18087/swagger/doc.json. Остановка: `Ctrl+C` в первом терминале.

<a id="full-scenario"></a>

#### 3. Полный сценарий

**Правило и комментарий**

Во втором терминале:

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

Ожидаются `201 Created` с сохранённым правилом (`id: 1` в новой БД), `200 OK` со списком из одного правила, затем `202 Accepted` без тела. `202` означает сохранение события, а не завершение отправок: их выполняет фоновый worker.

Дождитесь `waiting_email`, прежде чем отправлять email. Эта функция опрашивает состояние до 10 секунд и проверяет количество подтверждённых действий:

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
    raise SystemExit('Ожидаемое состояние не получено: ' + json.dumps(rows))
PY
}
wait_run waiting_email 2
```

Должны появиться две строки одного `run_id`: `reply_comment` и `ask_email`, обе `accepted`; состояние `waiting_email`, ошибки пустые. Это публичный ответ, затем private reply с вопросом. Порядок запросов к платформе отдельно проверяет HTTP-тест ниже; fake не показывает настоящую переписку.

Чтобы проверить перезапуск вручную, на этом шаге нажмите `Ctrl+C` в первом терминале и снова выполните **только** `GOPROXY=off GOSUMDB=off go run ./cmd/api`. Не меняйте `BLOTATO_DB` и не повторяйте `mktemp`. Во втором терминале повторите `GET /api/v1/workflows` и `wait_run waiting_email 2`: правило и ожидание должны сохраниться без новых действий.

**Ответ, ссылка и повторы**

В том же втором терминале, где определены `wait_run` и `COMMENT`:

```sh
MESSAGE="{\"platform\":\"fake\",\"account_id\":\"demo-brand\",\"contact_id\":\"demo-person\",\"kind\":\"message\",\"event_id\":\"demo-message-1\",\"text\":\"demo@example.com\",\"occurred_at\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}"
curl -fsS -i "$DEMO_URL/events" \
  -H 'Content-Type: application/json' -d "$MESSAGE"
wait_run completed 3
```

Ответ на событие — `202 Accepted`. После обработки `GET /api/v1/workflows/runs` возвращает:

```json
[
  {"run_id":1,"state":"completed","error":"","step":"reply_comment","status":"accepted","action_error":""},
  {"run_id":1,"state":"completed","error":"","step":"ask_email","status":"accepted","action_error":""},
  {"run_id":1,"state":"completed","error":"","step":"send_link","status":"accepted","action_error":""}
]
```

Это три действия одного прохождения, а не три разных run. Просмотр состояния не раскрывает email и тексты событий. `accepted` в fake означает успех подставного отправителя; в Instagram — принятие API, а не доставку или прочтение.

Повторите те же события с теми же ID и временем:

```sh
BEFORE=$(curl -fsS "$DEMO_URL/api/v1/workflows/runs")
curl -fsS -i "$DEMO_URL/events" -H 'Content-Type: application/json' -d "$COMMENT"
curl -fsS -i "$DEMO_URL/events" -H 'Content-Type: application/json' -d "$MESSAGE"
wait_run completed 3
AFTER=$(curl -fsS "$DEMO_URL/api/v1/workflows/runs")
test "$BEFORE" = "$AFTER" && printf 'OK: повтор событий не изменил действия\n'
```

Оба повтора получают `202`, остаются один run и три действия. Полная проверка числа HTTP-отправок и записей inbox находится в существующем тесте.

<a id="jev"></a>

#### 4. Проверка интеграции Jev

**Реализованная, но выключенная по умолчанию экспериментальная функция. Настоящий вызов OpenRouter и качество модели не подтверждены.** Jev определяет намерение использовать уже найденный email; не выбирает адресата, ссылку или произвольный следующий шаг.

| Вход после вопроса | Ожидаемое поведение |
|---|---|
| `anna@example.com` | Чистый email проходит без модели и без email_decisions; создаётся send_link. |
| `Write to anna@example.com` | Go маскирует адрес как EMAIL_0; ожидаем EMAIL_0 с достаточной confidence. Продолжение только при актуальном ожидании. |
| `Напишите мне на anna@example.com` | Та же задача; качество на русском требует реального эксперимента. |
| `Do not use anna@example.com` | Ожидаем reject; waiting_email, без send_link. |
| `Maybe anna@example.com` | Ожидаем uncertain; waiting_email. |
| `old@example.com or new@example.com` | Go сохраняет invalid без вызова модели: несколько кандидатов. |
| Ошибка провайдера, timeout или неправильный JSON | Сохраняется error, ожидание открыто. Проверяется подставным сервером, не порчей настоящего ключа. |

Это ожидаемые исходы, не измеренные ответы модели. EMAIL_0 с confidence ниже 0.95 тоже не продолжает run. Reject/error не отправляют уточняющее DM и не отменяют run: чистый email ещё может продолжить его до deadline.

**Точный контракт кода:**

- `POST https://openrouter.ai/api/alpha/decisions`; запрашивается `typesafe/jev-1.14`, принимается ответ только со snapshot `typesafe/jev-1.14-20261014`.
- Версия инструкции `email-intent-v1`. `answers.intent.type=choice`, choice из `EMAIL_0/reject/uncertain`, confidence в диапазоне 0..1.
- Порог `0.95` экспериментальный, не калиброван; HTTP timeout 5 секунд.
- В `email_decisions`: event_id (FK на внутренний inbox.id), run_id, pending/done, decision, model, instruction_version, confidence.
- Go проверяет состояние waiting_email, отсутствие принятого email, время получения и текущее время до deadline. Ссылку отправляет отдельный workflow job с прежней проверкой окна Meta.
- Остальной текст после замены email уходит провайдеру: маскирование не удаляет все личные данные. Для проверки использовать вымышленные сообщения.

Sources: [Jev client](../../internal/clients/jev/client.go), [классификация](../../internal/domain/service/workflow/email_decision.go), [SQL](../../internal/repo/sqlite/email_decisions.go), [сборка](../../internal/app/api/app.go), [Domain](2026-1028-plan-start.md#domain).

**Подставной HTTP-сервер, без оплаты и отправок.** Проверяющий может отдельно запустить существующие тесты:

```sh
GOPROXY=off GOSUMDB=off go test -race -count=1 \
  -run '^(TestEmailDecisionHTTP|TestSlowEmailDecisionDoesNotBlockActionsOrExpiry|TestEmailDecisionConcurrentCleanAnswer|TestEmailDecisionTimeout|TestEarlyEmailDecision)$' \
  -v ./internal/domain/service/workflow
```

Тесты задают известные HTTP-ответы: выбор/отказ/неоднозначность, низкую confidence, неверный JSON, timeout, несколько адресов, ранний ответ и конкуренцию с чистым email. Отдельная задача не блокирует другие отправки и expiry, пока модель медлит. Это проверка Go-контракта, не качества настоящей Jev.

**Ручная проверка настоящего OpenRouter через приложение.** Только после отдельного решения пользователя о платных вызовах. Fake-режим оставляет отправки Instagram подставными, но включённый Jev вызывает настоящую модель.

**Шаг 1.** Остановить локальный процесс. В том же терминале оставить настройки [локального запуска](#local-start), включая fake и ту же SQLite. До запуска скопировать её путь для второго терминала: `printf '%s\n' "$BLOTATO_DB"`. Ввести собственный ключ приватно и включить функцию. Следующий ввод предназначен для Bash; ключ не записывается в историю:

```bash
read -r -s -p 'OpenRouter key: ' OPENROUTER_API_KEY
printf '\n'
export OPENROUTER_API_KEY
export BLOTATO_EMAIL_INTENT_ENABLED=true
GOPROXY=off GOSUMDB=off go run ./cmd/api
```

**Шаг 2.** Во втором терминале оставить DEMO_URL. Использовать новый CASE_ID для каждой пробы. Следующий скрипт создаёт отдельный run через уже созданное pricing-правило, ждёт вопроса и передаёт только вымышленный ответ:

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

Для других примеров менять CASE_ID/CASE_TEXT. Один новый ответ с одним кандидатом обычно даёт один платный запрос; сбой до записи результата может привести к повтору. Повтор сохранённого события не является новой пробой. Чистый email и несколько кандидатов не требуют модели.

**Шаг 3.** Задать DEMO_DB равным **тому же** пути BLOTATO_DB первого терминала. Использовать путь, скопированный в шаге 1; это не секрет, процесс для чтения БД останавливать не нужно. Не использовать рабочую/VPS-базу. Следующий код открывает SQLite read-only и выводит только технические поля, без email и текстов:

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

EMAIL_0 с confidence ≥0.95 и актуальным ожиданием создаёт один send_link; после fake-отправки run станет completed. SQL может успеть показать link_pending. Reject/uncertain/error/invalid оставляют waiting_email до подходящего ответа или expiry. Позднее решение сохраняется, но не изменяет закрытый run. При ошибке model может содержать запрошенный ID, а не подтверждённый snapshot.

**Шаг 4.** Остановить процесс, установить `BLOTATO_EMAIL_INTENT_ENABLED=false`. Для неплатного демо снова использовать `OPENROUTER_API_KEY=local-demo-unused` только с выключенным флагом.

[14 вымышленных примеров](../email-intent-cases.jsonl) пригодны для следующей оценки. Они не доказывают качество. Нужно отдельно измерить ошибочные принятия/пропуски, задержку и стоимость. Done/error не повторяется автоматически; pending при падении до commit может вызвать модель снова.

<a id="evidence"></a>

#### 5. Подтверждённые результаты и непроверенные границы

| Проверка | Результат и источник |
|---|---|
| Общий локальный прогон | Worker-1: `GOPROXY=off GOSUMDB=off go test -race -count=1 ./...` прошёл, exit 0. Research не повторял. |
| Подписанный путь с подставным Meta | Worker-1: TestReleaseInstagramHTTPFlow, 3 HTTP-отправки, 1 run, 2 inbox, completed; адресаты, timestamp, повторы и перезапуск. |
| Ручной fake-проход | Worker-1: 201 → 202 → waiting_email → restart → 202 → completed; 1 правило, 1 run, 2 события, 3 действия; email_decisions пустая. |
| Бинарник | Worker-1: статический Linux amd64, SHA-256 `b1a1aa469dfd2cd3be9dfd1ece4aaad9bb3e764b7b1f40b7aeffe751dac912b0`. Research сверил локальный файл; worker-2 сообщил тот же хеш на VPS после restart. |
| Выпуск | Worker-2: Swagger/spec 200, управление без токена 401, создание/чтение 201/200, правило ID 1 сохранилось после restart, challenge 200 с точным телом, неверный token и unsigned POST 403. |
| Публичная документация | Research независимо выполнил GET UI/doc.json: 200, Swagger 2.0, basePath /blotato. Это не проверка хеша процесса. |
| Meta | Окружение для Unified Login/SUAT подготовлено. App Review не завершён: на прочитанном экране business portfolio не подтверждено, приложение не опубликовано. Подтверждённого отказа нет. Обе подписки и настоящие события не подтверждены. |
| Jev | Код и тесты с подставным HTTP-сервером есть. Флаг на VPS выключен по отчёту worker-2. Настоящий вызов, качество, задержка, стоимость и калибровка порога не измерены. |

Команды общего прогона и отдельного подписанного пути, здесь не выполнялись:

```sh
GOPROXY=off GOSUMDB=off go test -race -count=1 ./...
GOPROXY=off GOSUMDB=off go test -race -count=1 \
  -run '^TestReleaseInstagramHTTPFlow$' -v ./internal/app/api
```

Источники: отчёты worker-1 о unified-бинарнике и локальном демо от 2026-10-29, отчёт worker-2 о выпуске, [HTTP-тест](../../internal/app/api/app_test.go), [тесты Jev](../../internal/domain/service/workflow/email_decision_test.go), [release](2026-1029-plan-release-v0.1.md). Принятие API не равно доставке; challenge не равен настоящему webhook; подставной ответ не измеряет модель.

### Decisions

1. Это основная инструкция «Как проверить решение». README содержит короткий запуск и ссылки; init не дублирует команды.
2. Неплатный сценарий использует fake, временную SQLite и выключенный Jev. Настоящую модель проверяет пользователь отдельно и осознанно.
3. Настоящий Instagram/App Review не объявляются завершёнными. Unknown отправки не повторяется автоматически.

### Changes

1. 2026-10-29: перенесены команды и результаты worker-1 из прежней локальной инструкции; добавлены публичные проверки и отдельная ручная проверка Jev.
2. Init и INDEX ссылаются сюда; docs/a1-local.md удалён по явному разрешению после переноса.
3. README, код и infra не менялись. Тесты, сервисы, платные вызовы и отправки не запускались.
4. Финальная подготовка публикации: проверены staged-файлы на секреты и личные данные, убраны идентификаторы приложения и личные метаданные, проверены локальные ссылки README и taskfiles. Продуктовые тесты в этой проверке повторно не запускались.
