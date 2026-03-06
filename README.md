# API Blob

| Método | Rota | Privado | Descrição |
|--------|------|---------|-----------|
| `POST` | `/blob` |  `true` | Upload simples |
| `POST` | `/blob/multipart` |  `true` | Inicia upload multipart |
| `PUT` | `/blob/:id/part` |  `true` | Upload de parte |
| `POST` | `/blob/:id/complete` |  `true` | Finaliza multipart |
| `GET` | `/blob` |  `true` | Listar blobs |
| `GET` | `/blob/:id` |  `false` | Download |
| `HEAD` | `/blob/:id` |  `false` | Metadados |
| `DELETE` | `/blob/:id` |  `true` | Deletar |
| `GET` | `/health` |  `false` | Healthcheck |
| `GET` | `/` |  `false` | Hello, World |