# blob.cabrapi.com.br

Servico HTTP para upload, listagem e download de arquivos com:

- metadados em SQLite (`Prisma`)
- arquivos em disco local (`data/blob-storage`)
- URL assinada para blobs privados

## Requisitos

- Node.js 20+
- npm
- Docker (opcional)

## Configuracao

Crie `.env` em `blob/`:

```env
PORT=3000
HOST=0.0.0.0
DATABASE_URL=file:./data/blob.db
STORAGE_PATH=data/blob-storage
TOKEN_SECRET=troque-este-segredo-com-32-caracteres-ou-mais

SIGNED_URL_TTL_MIN_SECONDS=30
SIGNED_URL_TTL_MAX_SECONDS=900
SIGN_RATE_LIMIT_MAX=20
SIGN_RATE_LIMIT_WINDOW_MS=60000
DOWNLOAD_RATE_LIMIT_MAX=120
DOWNLOAD_RATE_LIMIT_WINDOW_MS=60000

# Opcional
# ALLOWED_MIME_TYPES=image/png,image/jpeg,application/pdf
# MAX_UPLOAD_SIZE_BYTES=20971520
# CORS_ORIGINS=https://app.example.com
```

## Executar Local

```bash
npm install
npm run db:prepare
npm run dev
```

## Docker (volume em `data/`)

```bash
docker compose up --build -d
```

Persistencia:

- `./data/blob.db`
- `./data/blob-storage/...`

## Endpoints

Base URL: `http://localhost:3000`

- `GET /health`
- `POST /blob/upload`
- `GET /blob?page=1&pageSize=20&bucket=docs`
- `GET /blob/:id/sign?ttl=120`
- `GET /blob/:id?exp=<exp>&n=<nonce>&sig=<sig>`
- `DELETE /blob/:id`

### Upload (`POST /blob/upload`)

`multipart/form-data`:

- `file` (obrigatorio)
- `bucket` (opcional)
- `key` (opcional)
- `public` (opcional: `true|false`)
- `metadata` (opcional: JSON string)

### Download Privado

1. Chame `GET /blob/:id/sign?ttl=120`
2. Use a `url` retornada para baixar o arquivo

## Seguranca

- assinatura HMAC-SHA512 com `nonce` e expiracao
- validacao de `bucket`/`key` e bloqueio de path traversal
- rate limit por IP nas rotas privadas
- headers `x-ratelimit-remaining` e `x-ratelimit-reset`

## Scripts

```bash
npm run db:generate
npm run db:push
npm run db:prepare
npm run check
```
