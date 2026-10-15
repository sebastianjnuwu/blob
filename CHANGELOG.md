# Changelog

All notable changes to this project will be documented in this file.

The format follows Keep a Changelog principles, organized by date.

## [2026-10-14]

### Removed
- Master token legado (`BLOB_TOKEN_SECRET`): auth agora é só `BLOB_API_KEYS` com escopos; token do website migrado para entrada `read,write`. `BLOB_SIGNED_URL_SECRET` (HMAC) inalterado.

### Added
- Chaves de API no banco (`api_keys`: hash sha256, escopos, revoke) + segredo HMAC persistido (`settings`): `MAIL_ADMIN` rotula chaves criadas via CLI (`blob admin create`, mostra o token uma vez; `list`/`revoke`). Sem rotas HTTP de admin e sem auto-geração no boot. Remove `BLOB_API_KEYS`/`BLOB_SIGNED_URL_SECRET` do env.
- Compatibilidade S3 (auth + objetos): verificação SigV4 (header `Authorization` ou query presigned, stdlib, com teste contra signer real `@smithy/signature-v4`) aceita em todas as rotas privadas junto ao Bearer; shim `PUT/GET/DELETE/HEAD /{bucket}/{key}`, `CreateBucket`, `ListBuckets`/`ListObjectsV2` (XML), ETag/Content-Length/Range. Demos funcionais em `examples/s3/js` e `examples/s3/go`.
- Credenciais estilo S3 (`access_key_id` + `secret`, colunas novas em `api_keys`): `blob admin create --label X --scopes ... --s3` (`--bearer` é o padrão; importa com `--access-key/--secret` ou `--token`); `list` exibe o access key; segredo fica no servidor para verificação SigV4. Bearer atual inalterado.
- Shim S3 completo para SDKs: `CopyObject` (PUT + `X-Amz-Copy-Source`, compartilha o arquivo via refcount, `REPLACE` de metadata), sobrescrita no PUT mesma-chave, `X-Amz-Acl: public-read` + GET/HEAD anônimo, `X-Blob-Expires-At` com enforcement, presigned URLs, multipart S3 (`Create/UploadPart/Complete/Abort`, `ListParts`, `ListMultipartUploads`), demos JS e Go validadas ao vivo.

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
