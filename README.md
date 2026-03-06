

# Blob Storage API – caBRAPI

API RESTful para armazenamento, listagem, download e remoção de arquivos (blobs), com controle de acesso, expiração opcional e URLs assinadas para acesso seguro.

**Principais recursos:**
- Metadados persistidos em SQLite via Prisma ORM
- Armazenamento local em disco (`data/blob-storage`)
- URLs assinadas (HMAC-SHA512) para blobs privados
- Rate limiting configurável por rota
- Upload seguro com validação de MIME

---

## Visão Geral Técnica

- **Stack:** Node.js 20+, Express, Prisma, Multer, Zod, JWT, Docker
- **Persistência:** SQLite (padrão) ou PostgreSQL (ajustando `DATABASE_URL`)
- **Segurança:** Token admin, URLs assinadas, rate limit, bloqueio path traversal
- **Deploy:** Docker Compose ou Node.js puro

---

---


## Endpoints REST

| Método | Endpoint                       | Auth           | Descrição                                      |
|--------|-------------------------------|----------------|------------------------------------------------|
| GET    | `/health`                     | -              | Healthcheck                                     |
| POST   | `/blob/upload`                | Admin Token    | Upload de arquivo (multipart/form-data)         |
| GET    | `/blob`                       | (opcional)     | Lista blobs públicos ou privados (admin)        |
| GET    | `/blob/:id`                   | (opcional)     | Download público, privado (admin) ou via URL    |
| GET    | `/blob/:id/sign?ttl=120`      | (opcional)     | Gera URL assinada para download privado         |
| DELETE | `/blob/:id`                   | Admin Token    | Remove blob                                    |

**Observações:**
- Para uploads e operações protegidas, envie `x-admin-token: <TOKEN_SECRET>` ou `Authorization: Bearer <TOKEN_SECRET>`.
- Blobs privados só podem ser baixados via URL assinada ou token admin.

---

---


## Requisitos e Setup

- Node.js 20+
- npm
- Docker (opcional)


## Variáveis de Ambiente

Copie `.env.example` para `.env` e ajuste:

- `TOKEN_SECRET` (obrigatório, segredo forte para autenticação admin)
- `DATABASE_URL` (opcional, SQLite ou PostgreSQL)
- `STORAGE_PATH` (opcional, caminho dos arquivos)
- `ALLOWED_MIME_TYPES` (opcional, restringe tipos de arquivo)
- `MAX_UPLOAD_SIZE_BYTES` (opcional, limite de upload)



## Autenticação & Controle de Acesso

- **Admin:** Header `x-admin-token` ou `Authorization: Bearer <TOKEN_SECRET>`
- **Público:** Listagem e download de blobs públicos sem autenticação
- **Privado:** Download apenas via URL assinada (`/blob/:id/sign`) ou admin

---


## Execução Local

```bash
cp .env.example .env
npm install
npm run db:prepare
npm run dev
```

API: http://127.0.0.1:3000

Healthcheck:
```bash
curl http://127.0.0.1:3000/health
```


## Docker

```bash
cp .env.example .env
docker compose up --build -d
```

Persistência:
- Banco: `./data/blob.db`
- Arquivos: `./data/blob-storage/`



## Exemplos de Uso (cURL)

Base URL: `http://127.0.0.1:3000`



### Upload (privado)
```bash
curl -X POST "http://127.0.0.1:3000/blob/upload" \
	-H "x-admin-token: <TOKEN_SECRET>" \
	-F "file=@./arquivo.pdf" \
	-F "bucket=docs" \
	-F "public=false"
```
**Resposta:**
```json
{
	"id": "...",
	"filename": "arquivo.pdf",
	"bucket": "docs",
	"public": false,
	"url": "http://127.0.0.1:3000/blob/<id>"
}
```

### Upload (público)
```bash
curl -X POST "http://127.0.0.1:3000/blob/upload" \
	-H "x-admin-token: <TOKEN_SECRET>" \
	-F "file=@./imagem.jpg" \
	-F "public=true"
```


### Listar blobs
```bash
# Públicos
curl "http://127.0.0.1:3000/blob?page=1&pageSize=5"
# Todos (inclui privados)
curl -H "x-admin-token: <TOKEN_SECRET>" "http://127.0.0.1:3000/blob?page=1&pageSize=5"
```



---

### ⏳ Expiração de Arquivos

- Se não informar `expiresAt`, o arquivo nunca expira.
- Se informar `expiresAt` (ISO 8601 ou timestamp), o arquivo será removido automaticamente após essa data/hora.
- Após expirar, nem mesmo o admin pode acessar o arquivo (erro 410).

---


### Download de blob
```bash
# Público
curl -L "http://127.0.0.1:3000/blob/<id>" --output arquivo.ext
# Privado (admin)
curl -L -H "x-admin-token: <TOKEN_SECRET>" "http://127.0.0.1:3000/blob/<id>" --output arquivo.ext
# Privado (externo via URL assinada)
curl "http://127.0.0.1:3000/blob/<id>/sign?ttl=300"
curl -L "http://127.0.0.1:3000/blob/<id>?exp=<exp>&n=<nonce>&sig=<sig>" --output arquivo.ext
```

**Baixar imagem privada (admin):**

```bash
curl -L -H "x-admin-token: <TOKEN_SECRET>" "http://127.0.0.1:3000/blob/<id>" --output minha-imagem.jpg
```

**Baixar imagem privada (externo, via URL assinada):**

```bash
# Gere a URL assinada
curl "http://127.0.0.1:3000/blob/<id>/sign?ttl=300"
# Use a URL retornada
curl -L "http://127.0.0.1:3000/blob/<id>?exp=<exp>&n=<nonce>&sig=<sig>" --output minha-imagem.jpg
```

**Privado (admin):**

```bash
curl -L -H "x-admin-token: <TOKEN_SECRET>" "http://127.0.0.1:3000/blob/<id>" --output arquivo.bin
```

**Privado (externo, via URL assinada):**

```bash
# 1. Gere a URL assinada
curl "http://127.0.0.1:3000/blob/<id>/sign?ttl=300"
# 2. Use a URL retornada
curl -L "http://127.0.0.1:3000/blob/<id>?exp=<exp>&n=<nonce>&sig=<sig>" --output arquivo.bin
```


### Remover blob
```bash
curl -X DELETE -H "x-admin-token: <TOKEN_SECRET>" "http://127.0.0.1:3000/blob/<id>"
```


## Segurança

- URLs assinadas HMAC-SHA512 (nonce, expiração, TTL)
- Validação de bucket/key, bloqueio path traversal
- Rate limit configurável por rota
- Headers: `x-ratelimit-remaining`, `x-ratelimit-reset`
- Upload seguro: restrição de MIME opcional


## Scripts Úteis

```bash
npm run dev
npm start
npm run db:generate
npm run db:push
npm run db:prepare
npm run typecheck
npm run check
```


## Troubleshooting

- Erro de schema/DB: execute `npm run db:prepare`
- Erro de assinatura: gere nova URL via `/blob/:id/sign` e use os parâmetros retornados
- Upload rejeitado: ajuste `ALLOWED_MIME_TYPES` no `.env` ou remova para aceitar qualquer tipo

---

## Limitações e Observações

- Não há versionamento de blobs (sobrescrita = novo registro)
- Expiração só é aplicada se `expiresAt` for definido
- Não há antivírus embutido (faça validação externa se necessário)
- URLs assinadas respeitam TTL e expiração do blob

---

## Contato e Suporte

Para dúvidas técnicas, sugestões ou bugs, abra uma issue ou entre em contato com o mantenedor.
