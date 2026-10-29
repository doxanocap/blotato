# Выпуск Blotato

Эта инструкция учитывает текущую конфигурацию приложения. Существующие README не обновлялись в этом раунде: сведения в них о `BLOTATO_LINK`, выборе бинарника по умолчанию и неисправленном префиксе Swagger устарели.

## До выпуска

- Worker-1 передаёт проверенный статический бинарник Linux amd64 и его SHA-256. До публикации проверить запуск в `caddy:2-alpine` и сверить хеш. Не выбирать старый бинарник автоматически.
- На VPS создать `<root>/apps/blotato/.env` по `instagram.env.example`, владелец root, права `0600`. Обязательны `BLOTATO_ADMIN_TOKEN`, `INSTAGRAM_ACCOUNT_ID`, `INSTAGRAM_ACCESS_TOKEN`, `META_APP_SECRET`, `META_VERIFY_TOKEN`, `OPENROUTER_API_KEY`. Секреты не передавать через аргументы команд и Git.
- `INSTAGRAM_API_BASE` необязателен: приложение использует `https://graph.instagram.com/v26.0` по умолчанию. Ссылка для отправки задаётся в правиле через API. Compose задаёт `BLOTATO_MODE`, `BLOTATO_DB` и `BLOTATO_LISTEN`.
- Сверить свободное место и общие Compose/Caddyfile на VPS с локальными файлами, чтобы выпуск не перезаписал чужие изменения. SQLite хранится отдельно в `<root>/data/blotato`.

## Выпуск после общей синхронизации

Из локального `craft/apps/infra`, после сверки хеша с worker-1:

```sh
shasum -a 256 /absolute/path/to/verified-blotato-linux-amd64
BLOTATO_BIN=/absolute/path/to/verified-blotato-linux-amd64 ./blotato/release.sh
```

Скрипт проверяет окружение, передаёт бинарник и общие конфигурации, сохраняет прежние версии в `.previous`, пересоздаёт только Blotato и перезагружает Caddy. SQLite не заменяется. Один комплект `.previous` перезаписывается при следующем выпуске.

## Проверка после выпуска

```sh
curl -fsS -o /dev/null -w 'swagger: %{http_code}\n' https://kadirbekovvv.me/blotato/swagger/index.html
curl -fsS https://kadirbekovvv.me/blotato/swagger/doc.json | python3 -c 'import json,sys; print("basePath:", json.load(sys.stdin)["basePath"])'
curl -sS -o /dev/null -w 'workflows without token: %{http_code}\n' https://kadirbekovvv.me/blotato/api/v1/workflows
curl -sS -o /dev/null -w 'unsigned webhook: %{http_code}\n' -X POST https://kadirbekovvv.me/blotato/webhook/instagram
```

Ожидаются 200 для Swagger и существующих маршрутов, 401 для управления без токена и 403 для неподписанного webhook. В браузере проверить, что Swagger загружает `/blotato/swagger/openapi.json` и отправляет запросы на `/blotato/automations`. Токен вводится в Swagger только по HTTPS, не в командной строке.

С токеном проверить чтение правил, создать проверочное правило и запомнить его ID. На VPS из `<root>/infra` выполнить `docker compose --env-file .env --profile blotato restart blotato`, затем снова прочитать правило и сравнить ID и поля. Это проверяет сохранность SQLite, а не доставку Meta. Настоящий webhook и Instagram-сценарий проверяются отдельно. Состояние и диагностика: `docker compose --env-file .env --profile blotato ps blotato` и `docker compose --env-file .env --profile blotato logs --tail=100 blotato`.

## Откат

На VPS из `<root>/infra`:

```sh
docker compose --env-file .env --profile blotato stop blotato
cat Caddyfile.previous > Caddyfile
docker compose --env-file .env exec -T caddy caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile
cp docker-compose.yml.previous docker-compose.yml
```

После первого выпуска Blotato остаётся остановленным. При обновлении, когда worker-1 подтвердит совместимость схемы БД с предыдущим бинарником, дополнительно:

```sh
cp -p ../apps/blotato/blotato.previous ../apps/blotato/blotato
docker compose --env-file .env --profile blotato up -d --no-deps --force-recreate blotato
```

`data/blotato` и `.env` сохраняются при откате.
