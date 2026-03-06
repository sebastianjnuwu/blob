# Blob API

| Método | Rota | Autenticação | Descrição |
|--------|------|-------------|-----------|
| POST   | /blob                | Admin Token | Upload  |
| GET    | /blob                | Admin Token | Lista blobs com filtros (prefixo/delimiter) |
| GET    | /blob/:id            | Público     | Download do blob (byte-range) |
| HEAD   | /blob/:id            | Público     | Metadados do blob |
| DELETE | /blob/:id            | Admin Token | Remove blob |
| GET    | /health              | Público     | Healthcheck |
