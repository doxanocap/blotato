# Запуск на существующем VPS

Это подготовка: приложение и новый маршрут ещё не опубликованы. `release.sh` для этой поставки не запускать.

- `infra` использует существующий `caddy:2-alpine`; бинарник Linux amd64 подключается только для чтения. В контейнере нет опубликованных портов.
- SQLite и её служебные файлы находятся в `data/blotato/` на хосте, отдельно от бинарника. Контейнер работает как `65532:65532`; на одну базу запускается один процесс.
- Caddy убирает префикс `/blotato` перед передачей запроса. Планируемый callback: `https://kadirbekovvv.me/blotato/webhook/instagram`. Путь Swagger и его ссылки нужно подтвердить с worker-1 до публикации.
- Контейнер перезапускается Docker через `restart: unless-stopped`. Логи: `docker compose logs --tail=100 blotato`; состояние: `docker compose --profile blotato ps`.

## Перед выкладкой

Worker-1 передаёт проверенный статический бинарник Linux amd64 и точные внутренние пути Swagger и его спецификации. Проверить запуск бинарника в образе `caddy:2-alpine`, работу Swagger под внешним префиксом `/blotato` и отсутствие ссылок на корневые пути другого приложения. До этого Swagger не публиковать.

На хосте подготовлены каталоги `<root>/apps/blotato` и `<root>/data/blotato`. В первом создать `.env` по `instagram.env.example`, права `0600`, владелец root. Заполнить все обязательные поля, включая `OPENROUTER_API_KEY`, и указать `https://graph.facebook.com/v26.0` в `INSTAGRAM_API_BASE`. Не передавать секреты через командную строку или Git. Compose задаёт режим, адрес и путь БД самостоятельно.

## Выкладка после синхронизации

Команды ниже выполняются на хосте из `<root>/infra`, указанного в `PROFILE_HOST`. Бинарник и обновлённые `docker-compose.yml` и `Caddyfile` передаются отдельно, без `rsync --delete`. Перед заменой сохранить текущие файлы и предыдущий бинарник вне SQLite. Не заменять работающий бинарник на месте: передать новый файл с другим именем, затем остановить только `blotato` и переименовать файл.

1. Из локального `craft/apps/infra` передать бинарник, путь к которому указан в `BLOTATO_BINARY`, и подготовленные конфигурации. `PROFILE_HOST` читается из ignored `.env` и не выводится:

   ```sh
   set -eu
   target=$(awk -F= '$1 == "PROFILE_HOST" { sub(/^[^=]*=/, ""); print; exit }' .env)
   without_scheme=${target#ssh://}
   remote=${without_scheme%%/*}
   remote_dir=/${without_scheme#*/}
   test "${target#ssh://}" != "$target" && test "$remote" != "$without_scheme"
   test -s "$BLOTATO_BINARY"
   scp "$BLOTATO_BINARY" "$remote:$remote_dir/apps/blotato/blotato.next"
   scp docker-compose.yml "$remote:$remote_dir/infra/docker-compose.yml.next"
   scp Caddyfile "$remote:$remote_dir/infra/Caddyfile.next"
   ```

   На хосте установить владельца и права: `chown root:root ../apps/blotato/blotato.next` и `chmod 0755 ../apps/blotato/blotato.next`.
2. Проверить конфигурации до замены:

   ```sh
   docker compose --env-file .env -f docker-compose.yml.next config --quiet
   docker exec -i infra-caddy-1 caddy validate --config /dev/stdin --adapter caddyfile < Caddyfile.next
   ```

3. Сохранить текущие файлы и, если это обновление, остановить Blotato и сохранить предыдущий бинарник. Затем заменить конфигурацию и создать контейнер заново:

   ```sh
   cp -p docker-compose.yml docker-compose.yml.previous
   cp -p Caddyfile Caddyfile.previous
   if docker inspect infra-blotato-1 >/dev/null 2>&1; then
     docker stop infra-blotato-1
   fi
   if test -f ../apps/blotato/blotato; then
     cp -p ../apps/blotato/blotato ../apps/blotato/blotato.previous
   fi
   mv ../apps/blotato/blotato.next ../apps/blotato/blotato
   cp docker-compose.yml.next docker-compose.yml
   cat Caddyfile.next > Caddyfile
   docker compose --env-file .env --profile blotato up -d --no-deps --force-recreate blotato
   docker exec infra-caddy-1 caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile
   docker compose --profile blotato ps
   docker compose logs --tail=100 blotato
   ```

   Caddyfile смонтирован как отдельный файл, поэтому его содержимое записывается в существующий inode. Контейнер Blotato пересоздаётся, чтобы подключить новый inode бинарника. `profile` и Caddy не пересоздаются. Файлы `.previous` предназначены для одного отката; перед следующим выпуском сохранить их отдельно, если они ещё нужны.
4. Проверить публичные Swagger и webhook по согласованным путям, затем отдельно проверить profile и его существующие маршруты. Не отправлять реальные сообщения до настройки Meta и общей синхронизации.

## Откат

Из `<root>/infra`:

```sh
docker compose --profile blotato stop blotato
cat Caddyfile.previous > Caddyfile
docker exec infra-caddy-1 caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile
cp docker-compose.yml.previous docker-compose.yml
```

При первом выпуске оставить Blotato остановленным. При обновлении, после проверки совместимости схемы БД с предыдущим бинарником у worker-1, вернуть бинарник и предыдущий Compose с сервисом Blotato, затем запустить:

```sh
cp -p ../apps/blotato/blotato.previous ../apps/blotato/blotato
docker compose --env-file .env --profile blotato up -d --no-deps --force-recreate blotato
```

Каталог `data/blotato` и `.env` не удалять.
