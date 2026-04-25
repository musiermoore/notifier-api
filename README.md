# notifier-api

Cloud API for auth, notes, reminders, sync, and client integration.

## Planned Modules

- `auth`: user registration, login, refresh, logout
- `items`: global notes and reminders
- `sync`: incremental upload and download for offline clients
- `devices`: desktop installations and notification preferences
- `telegram`: Telegram account linking

## First Rule

The API is the only service that talks to PostgreSQL directly.

## Current Endpoints

- `GET /health`
- `GET /v1/meta`
- `POST /v1/auth/register`
- `POST /v1/auth/login`
- `GET /v1/auth/me`
- `POST /v1/auth/logout`
- `GET /v1/items`
- `POST /v1/items`
- `GET /v1/items/{id}`
- `PATCH /v1/items/{id}`
- `DELETE /v1/items/{id}`
- `GET /v1/sync/pull?since=RFC3339`

## Auth Model

This MVP uses opaque bearer session tokens stored in PostgreSQL. That keeps the first version simple for web, Telegram, and desktop clients while we are still shaping sync behavior.
