# notifier-api

Cloud API for auth, notes, reminders, sync, and Telegram integration.

API облака для авторизации, заметок, напоминаний, синхронизации и интеграции с Telegram.

## Rules

- The API is the only service that talks to PostgreSQL directly.
- Telegram messages are not sent by the API itself. The API prepares delivery jobs, and the Telegram bot service sends them.

- Только API работает с PostgreSQL напрямую.
- Сам API не отправляет сообщения в Telegram. API готовит задания на доставку, а отправляет их отдельный сервис Telegram-бота.

## Current Commands And Endpoints

### Auth

- `POST /v1/auth/register` - register a user
- `POST /v1/auth/login` - log in and get a bearer token
- `GET /v1/auth/me` - get current user profile
- `POST /v1/auth/logout` - close current session

- `POST /v1/auth/register` - регистрация пользователя
- `POST /v1/auth/login` - вход и получение bearer-токена
- `GET /v1/auth/me` - получить профиль текущего пользователя
- `POST /v1/auth/logout` - завершить текущую сессию

### Items

- `GET /v1/items` - list items
- `POST /v1/items` - create an item
- `GET /v1/items/{id}` - get one item
- `PATCH /v1/items/{id}` - update an item
- `DELETE /v1/items/{id}` - delete an item
- `GET /v1/sync/pull?since=RFC3339` - pull changed records for sync

- `GET /v1/items` - список записей
- `POST /v1/items` - создать запись
- `GET /v1/items/{id}` - получить одну запись
- `PATCH /v1/items/{id}` - обновить запись
- `DELETE /v1/items/{id}` - удалить запись
- `GET /v1/sync/pull?since=RFC3339` - получить измененные записи для синхронизации

### Telegram Linking

- `GET /v1/telegram/link` - get Telegram connection status
- `POST /v1/telegram/link-code` - generate a one-time Telegram link code
- `DELETE /v1/telegram/link` - unlink Telegram
- `POST /v1/telegram/consume-link` - consume a link code from the bot side

- `GET /v1/telegram/link` - получить статус подключения Telegram
- `POST /v1/telegram/link-code` - создать одноразовый код привязки Telegram
- `DELETE /v1/telegram/link` - отвязать Telegram
- `POST /v1/telegram/consume-link` - подтвердить код привязки со стороны бота

### Internal Telegram Bot Endpoints

- `GET /v1/internal/telegram/me` - resolve a Telegram chat to a linked user
- `GET /v1/internal/telegram/items` - list recent items for a linked Telegram chat
- `GET /v1/internal/telegram/deliveries/pending` - get pending Telegram deliveries
- `POST /v1/internal/telegram/deliveries/{id}/complete` - mark a delivery as sent
- `POST /v1/internal/telegram/deliveries/{id}/fail` - mark a delivery as failed

- `GET /v1/internal/telegram/me` - найти пользователя по привязанному Telegram-чату
- `GET /v1/internal/telegram/items` - получить последние записи для привязанного Telegram-чата
- `GET /v1/internal/telegram/deliveries/pending` - получить ожидающие доставки в Telegram
- `POST /v1/internal/telegram/deliveries/{id}/complete` - отметить доставку как отправленную
- `POST /v1/internal/telegram/deliveries/{id}/fail` - отметить доставку как ошибочную

### Meta And Health

- `GET /health` - health check
- `GET /v1/meta` - app metadata and supported languages

- `GET /health` - проверка здоровья сервиса
- `GET /v1/meta` - метаданные приложения и поддерживаемые языки

## Current Status

- Session auth is implemented with opaque bearer tokens stored in PostgreSQL.
- Item CRUD is implemented.
- Telegram account linking is implemented.
- Telegram delivery queue is implemented for the bot worker.

- Авторизация через opaque bearer-токены в PostgreSQL уже реализована.
- CRUD для записей уже реализован.
- Привязка Telegram-аккаунта уже реализована.
- Очередь доставок в Telegram для воркера бота уже реализована.
