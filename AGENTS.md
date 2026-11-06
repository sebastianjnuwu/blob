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


## Roadmap (GitHub Projects)

- The roadmap and tasks per service live in **GitHub Projects** of the repository (caBRAPI). Each service (website, api, edge, blob, mcp, docs) has phase columns (Backlog -> Sprint -> Doing -> Review -> Done) and area labels (perf, a11y, security, bug, feature).
- Issues are linked to the corresponding service Project; milestones mark deliveries.
- Flow: issue created -> tagged with service + area -> prioritized in the Project -> PR closes the issue via Closes #n -> moved to Done after merge + deploy.
- Keep the Project status updated when starting/pausing/delivering; do not use parallel docs as the source of truth.
