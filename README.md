# blob.cabrapi.com.br

API de armazenamento de arquivos com:

- metadados em Prisma + SQLite local
- arquivos salvos em disco local (`data/`)
- URL assinada para blobs privados
- suporte a Docker com volume persistente
- protecao contra brute force em rotas privadas

## Arquitetura

- Banco local: `data/blob.db` (SQLite)
- Arquivos: `data/blob-storage/objects/...`
- Hash de conteudo: `blake2b-512` com fallback para `sha512`

Essa estrategia evita dependencia de PostgreSQL/Redis externo no ambiente local.

## Requisitos

- Node.js 20+
- npm
- Docker (opcional)

## Instalacao Local

```bash
npm install
```

Crie o `.env` no diretorio `blob/`:

```env
PORT=3000
HOST=0.0.0.0

# SQLite local em data/
DATABASE_URL=file:./data/blob.db

# Caminho base dos arquivos (volume local)
STORAGE_PATH=data/blob-storage

# Segredo para assinatura HMAC
TOKEN_SECRET=troque-este-segredo

# Hardening de URL assinada
SIGNED_URL_TTL_MIN_SECONDS=30
SIGNED_URL_TTL_MAX_SECONDS=900
SIGN_RATE_LIMIT_MAX=20
SIGN_RATE_LIMIT_WINDOW_MS=60000
DOWNLOAD_RATE_LIMIT_MAX=120
DOWNLOAD_RATE_LIMIT_WINDOW_MS=60000

# Opcional: lista de MIMEs permitidos no upload
# ALLOWED_MIME_TYPES=image/png,image/jpeg,application/pdf

# Opcional: limite maximo de upload em bytes
MAX_UPLOAD_SIZE_BYTES=20971520

# Opcional: CORS permitido (lista separada por virgula)
# CORS_ORIGINS=https://app.example.com,https://admin.example.com
```

Prepare banco e cliente Prisma:

```bash
npm run db:prepare
```

Suba a API:

```bash
npm run dev
```

ou

```bash
npm start
```

Servidor padrao: [`http://localhost:3000`](http://localhost:3000).

## Docker com Volume

Arquivos:

- `Dockerfile`
- `docker-compose.yml`

Subir com persistencia em volume local (`./data -> /app/data`):

```bash
docker compose up --build -d
```

Parar:

```bash
docker compose down
```

Com esse volume, os dados persistem entre reinicios:

- `./data/blob.db`
- `./data/blob-storage/...`

## Endpoints

Base URL local: `http://localhost:3000`

### Health

- `GET /health`

### Upload

- `POST /blob/upload`
- `Content-Type: multipart/form-data`

Campos:

- `file` obrigatorio
- `bucket` opcional, default `default`
- `key` opcional, default nome original
- `public` opcional (`true|false`), default `false`
- `metadata` opcional, JSON em string

### Listagem

- `GET /blob?page=1&pageSize=20`
- filtro opcional: `bucket`

### URL Assinada

- `GET /blob/:id/sign?ttl=300`
- `ttl` e limitado entre `SIGNED_URL_TTL_MIN_SECONDS` e `SIGNED_URL_TTL_MAX_SECONDS`

Resposta:

```json
{
  "id": "blob-id",
  "exp": 1772766000,
  "n": "3eWk0...nonce...VQ",
  "sig": "assinatura-base64url",
  "ttl": 300,
  "url": "/blob/blob-id?exp=1772766000&n=3eWk0...nonce...VQ&sig=assinatura-base64url"
}
```

### Download

- `GET /blob/:id`
- blob publico: sem assinatura
- blob privado: exige `exp`, `n` e `sig`

Fluxo recomendado para blob privado:

1. Chamar `GET /blob/:id/sign`.
2. Usar a `url` retornada para fazer download.

Exemplo completo com `curl`:

```bash
curl "http://localhost:3000/blob/<id>/sign?ttl=120"
curl "http://localhost:3000/blob/<id>?exp=<exp>&n=<nonce>&sig=<sig>"
```

### Remocao

- `DELETE /blob/:id`
- marca o registro como removido (`deletedAt`) e tenta apagar arquivo no disco

## Seguranca

- `helmet` para headers HTTP
- `x-request-id` por requisicao
- validacao de `bucket` e `key`
- bloqueio de path traversal
- assinatura HMAC com expiracao para blob privado
- comparacao segura com `timingSafeEqual`
- assinatura inclui `nonce` unico por URL assinada
- limitacao de taxa por IP para `GET /blob/:id/sign` e `GET /blob/:id`
- whitelist opcional de MIME
- limite de upload configuravel

## Tutorial Rapido

### 1. Subir local

```bash
npm install
npm run db:prepare
npm run dev
```

### 2. Upload privado

```bash
curl -X POST "http://localhost:3000/blob/upload" \
  -F "file=@./arquivo.pdf" \
  -F "bucket=docs" \
  -F "key=contratos/arquivo.pdf"
```

### 3. Gerar URL assinada

```bash
curl "http://localhost:3000/blob/<id>/sign?ttl=120"
```

### 4. Baixar arquivo privado

```bash
curl "http://localhost:3000/blob/<id>?exp=<exp>&n=<nonce>&sig=<sig>" --output arquivo.pdf
```

### 5. Docker com volume persistente

```bash
docker compose up --build -d
```

Dados persistidos em `./data/`:

- `./data/blob.db`
- `./data/blob-storage/...`

## FAQ

### "Por que aparece um .txt do nada?"

Esse arquivo nao e criado pela API. Ele aparece quando testes manuais criam
temporarios (por exemplo, `tmp-upload.txt`).

- O projeto agora ignora `tmp-upload.txt` no `.gitignore`.
- Em producao, os arquivos reais sao salvos somente em `data/blob-storage/`.

## Estrutura em `data/`

```text
data/
  blob.db
  blob-storage/
    objects/
      ab/
        cd/
          <hash>
```

## Otimizacoes Aplicadas

- hash mais forte para fingerprint de conteudo
- consultas paginadas no Prisma para listagem
- retorno de blob duplicado por hash sem recriar registro
- contador de download incrementado sem bloquear streaming

## Comandos Uteis

```bash
npm run db:generate
npm run db:push
npm run db:prepare
npm run check
```

## Observacoes

- Nao versione o `.env` com segredo real.
- Em producao, use volume persistente para `data/`.
- Troque `TOKEN_SECRET` por um valor forte e unico.
