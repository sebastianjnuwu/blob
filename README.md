
# blob.cabrapi.com.br

> Serviço HTTP para upload, listagem, visualização e remoção de arquivos (blobs), com autenticação opcional para arquivos privados.

**Principais recursos:**
- Metadados em SQLite (`Prisma`)
- Arquivos em disco local (`data/blob-storage`)
- URL assinada para blobs privados
- Limite de taxa (rate limit) para rotas sensíveis

---

## 🗂️ Tabela de Rotas

| Método | Rota                              | Pública | Admin Token | Descrição                                 |
|--------|-----------------------------------|---------|-------------|-------------------------------------------|
| GET    | `/health`                         | ✅      | -           | Healthcheck rápido                        |
| POST   | `/blob/upload`                    | ❌      | ✅          | Upload de arquivo                         |
| GET    | `/blob`                           | ✅      | (opcional)  | Lista blobs públicos (ou privados c/ token)|
| GET    | `/blob/:id`                       | ✅      | (opcional)  | Download público ou privado (token/assinada)|
| GET    | `/blob/:id/sign?ttl=120`          | ✅      | (opcional)  | Gera URL assinada para download privado   |
| DELETE | `/blob/:id`                       | ❌      | ✅          | Remove blob                               |

---

## Requisitos

- Node.js 20+
- npm
- Docker (opcional)

## Configuracao

Use o arquivo de exemplo:

```bash
cp .env.example .env
```

No Windows (PowerShell):

```powershell
Copy-Item .env.example .env
```

Ajuste obrigatoriamente:

- `TOKEN_SECRET`


## 🔐 Autenticação & Acesso

- **Token administrativo:**
	- Use o valor de `TOKEN_SECRET` no header `x-admin-token` (ou `Authorization: Bearer ...`) para operações protegidas.
	- Operações protegidas: upload, delete, listagem de blobs privados, download direto de blob privado.
- **Acesso público:**
	- Qualquer usuário pode listar e baixar blobs públicos sem autenticação.
- **Acesso privado:**
	- Blobs privados só podem ser baixados via URL assinada (válida por tempo limitado) ou diretamente pelo admin.

---

## Executar Local (Passo a Passo)

```bash
npm install
npm run db:prepare
npm run dev
```

API local:

- `http://127.0.0.1:3000`

Healthcheck rapido:

```bash
curl http://127.0.0.1:3000/health
```

## Docker (volume em `data/`)

Antes de subir o container, gere seu `.env`:

```bash
cp .env.example .env
```

```bash
docker compose up --build -d
```

Verificar status e healthcheck:

```bash
docker compose ps
docker compose logs -f blob
```

Persistencia:

- `./data/blob.db`
- `./data/blob-storage/...`


## 🚦 Exemplos de Uso

Base URL: `http://127.0.0.1:3000`

### 📤 Upload de Arquivo (Privado ou Público)

**Privado (requer token admin):**

```bash
curl -X POST "http://127.0.0.1:3000/blob/upload" \
	-H "x-admin-token: <TOKEN_SECRET>" \
	-F "file=@./arquivo.pdf" \
	-F "bucket=docs" \
	-F "public=false"
```

**Público:**

```bash
curl -X POST "http://127.0.0.1:3000/blob/upload" \
	-H "x-admin-token: <TOKEN_SECRET>" \
	-F "file=@./imagem.jpg" \
	-F "public=true"
```

### 📄 Listar Blobs

**Somente públicos:**

```bash
curl "http://127.0.0.1:3000/blob?page=1&pageSize=5"
```

**Todos (inclui privados, requer token admin):**

```bash
curl -H "x-admin-token: <TOKEN_SECRET>" "http://127.0.0.1:3000/blob?page=1&pageSize=5"
```

### 📥 Download de Blob

**Público:**

```bash
curl -L "http://127.0.0.1:3000/blob/<id>" --output arquivo.bin
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

### 🗑️ Remover Blob (admin)

```bash
curl -X DELETE -H "x-admin-token: <TOKEN_SECRET>" "http://127.0.0.1:3000/blob/<id>"
```

---


---


---


## 🔒 Segurança

- Assinatura HMAC-SHA512 com `nonce` e expiração
- Validação de `bucket`/`key` e bloqueio de path traversal
- Rate limit por IP nas rotas privadas
- Headers `x-ratelimit-remaining` e `x-ratelimit-reset`


---

## 📜 Scripts

```bash
npm run dev
npm start
npm run db:generate
npm run db:push
npm run db:prepare
npm run typecheck
npm run check
```


---

## 🛠️ Troubleshooting

- Erro de schema/DB:
	- execute `npm run db:prepare`.
- Erro de assinatura em blob privado:
	- gere novamente via `/blob/:id/sign` e use os parametros `exp`, `n`, `sig` sem alteracao.
- Upload rejeitado por MIME:
	- ajuste `ALLOWED_MIME_TYPES` no `.env` ou remova a variavel para aceitar qualquer MIME.
