# Changelog

All notable changes to this project will be documented in this file.

The format follows Keep a Changelog principles, organized by date.

## [2026-10-14]

### Removed
- Master token legado (`BLOB_TOKEN_SECRET`): auth agora é só `BLOB_API_KEYS` com escopos; token do website migrado para entrada `read,write`. `BLOB_SIGNED_URL_SECRET` (HMAC) inalterado.

### Added
- Chaves de API no banco (`api_keys`: hash sha256, escopos, revoke) + segredo HMAC persistido (`settings`): `MAIL_ADMIN` rotula a chave admin criada no primeiro boot (impressa uma vez no log). Endpoints `GET/POST /admin/keys` e `DELETE /admin/keys/:id` (escopo `admin`). Remove `BLOB_API_KEYS`/`BLOB_SIGNED_URL_SECRET` do env.

### Changed
- Logger unificado com o da API (`src/lib/logger`): tags de cor (`<red>`, `<cyan>`...), categorias (`Server`, `Database`, `Environment`, `HTTP`, `Queue`), helper `StatusCode`. Chamadas migradas de `functions.Info/Warn/Error/Debug` (16 arquivos + `main.go`); `src/functions/logger.go` removido.

## [2026-06-29]

### Added
- Added `view_count` tracking for inline file views (`/blob/:id/view`).

### Changed
- Updated file serving flow to use explicit counters per route:
	- `/blob/:id/download` increments `download_count`
	- `/blob/:id/view` increments `view_count`
- Metrics summary now includes `total_views`.
- Updated API documentation examples and schema with `view_count`.

### Fixed
- Added legacy-safe counter increment using `COALESCE(..., 0) + 1`.
- Added startup backfill to normalize `NULL` values in `download_count` and `view_count` to `0`.
