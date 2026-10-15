# AGENTS.md — Blob caBRAPI

## Migrações seguras (obrigatório)

Toda mudança de banco/storage segue expand → migrate → contract, nunca tudo de uma vez:

1. **Fase aditiva**: só adiciona (coluna/tabela nullable, sem remover nem renomear). O código novo lê com fallback pro antigo e escreve nos dois (dual-write).
2. **Backfill**: migra os dados existentes com script idempotente (pula os já migrados) antes de qualquer remoção.
3. **Remoção**: só apaga o antigo depois do novo validado em produção (leitura 100% no novo + período de observação).
4. **Rollback**: cada fase tem volta (migration reversível ou compatível pra frente e pra trás).
5. **Registro**: entrada no `CHANGELOG.md` por fase; ordem no deploy: migrate → backfill → deploy do código → limpeza.

## Verificação

- Vet: `go vet ./...`
- Build: `go build ./...`
