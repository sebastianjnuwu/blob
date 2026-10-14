# S3 demo (Node)

Demo do shim S3 do Blob com `@aws-sdk/client-s3` real.

```bash
npm ci
```

Crie credenciais S3 no servidor:

```bash
go run . admin create-s3 --label demo --scopes read,write,delete
```

Rode contra o Blob local (ex.: porta 3099):

```bash
S3_ACCESS_KEY_ID=<access> S3_SECRET_ACCESS_KEY=<secret> node index.js
```

O script faz `ListBuckets → PutObject → ListObjectsV2 → GetObject` e confere os bytes.

`sign-test.cjs` gera vetores de assinatura (usado no teste Go `sigv4_smithy_test.go`).
