# AGENTS.md — Blob caBRAPI

## Safe migrations (mandatory)

Every database/storage change follows expand → migrate → contract, never everything at once:

1. **Additive phase**: only adds (nullable column/table, no removal or rename). New code reads with fallback to the old and writes to both (dual-write).
2. **Backfill**: migrate existing data with an idempotent script (skip already migrated) before any removal.
3. **Removal**: only delete the old after the new is validated in production (100% read on new + observation period).
4. **Rollback**: each phase has a rollback (reversible migration or forward/backward compatible).
5. **Record**: entry in `CHANGELOG.md` per phase; deploy order: migrate → backfill → deploy code → cleanup.

## Verification

- Vet: `go vet ./...`
- Build: `go build ./...`

## Roadmap (GitHub Projects)

- The roadmap and tasks per service live in **GitHub Projects** of the repository (caBRAPI). Each service (website, api, edge, blob, mcp, docs) has phase columns (Backlog -> Sprint -> Doing -> Review -> Done) and area labels (perf, a11y, security, bug, feature).
- Issues are linked to the corresponding service Project; milestones mark deliveries.
- Flow: issue created -> tagged with service + area -> prioritized in the Project -> PR closes the issue via Closes #n -> moved to Done after merge + deploy.
- Keep the Project status updated when starting/pausing/delivering; do not use parallel docs as the source of truth.