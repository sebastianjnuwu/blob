# curl examples (native API)

Shell scripts for the native Blob API (multipart form + Bearer auth).

```bash
export BLOB_URL=http://127.0.0.1:3099
export BLOB_API_KEY=<api-key>   # from: go run . admin create --label demo --scopes read,write,delete
```

| Script            | Flow                                              |
| ----------------- | ------------------------------------------------- |
| `upload.sh`       | `PUT /blob` (form file + bucket)                  |
| `download.sh`     | `GET /blob/:id/download` to a file                |
| `sign.sh`         | `POST /blob/:id/sign` (signed download/view URLs) |
| `list.sh`         | `GET /blob` (bucket filter + pagination)          |
| `delete.sh`       | `DELETE /blob/:id`                                |
| `multipart.sh`    | initiate → chunks (sha256) → complete (use arquivos múltiplos de 1MB; o servidor rejeita chunk < `BLOB_MIN_CHUNK_SIZE`) |

Each script prints pretty JSON when `jq` is available.
