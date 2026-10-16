# S3 demo (Go)

Demo do shim S3 do Blob com `aws-sdk-go-v2` real.

```bash
go mod download
```

Crie credenciais S3 no servidor:

```bash
go run . admin create --label demo --scopes read,write,delete --s3
```

Rode contra o Blob local (ex.: porta 3099):

```bash
S3_ENDPOINT=http://127.0.0.1:3099 S3_ACCESS_KEY_ID=<access> S3_SECRET_ACCESS_KEY=<secret> S3_BUCKET=demo go run .
```

O script faz `ListBuckets → PutObject → ListObjectsV2 → HeadObject → GetObject`
e confere os bytes. `DELETE_OBJECT=true` apaga o objeto no fim.
