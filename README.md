# hwj-macgo-0018

Idempotent request credential core for offline Go projects.

## Design invariants

- A request key is scoped by caller and namespace.
- The same key binds exactly one operation digest.
- Committed result snapshots are immutable.
- Late commits from an old execution generation must not overwrite a result after timeout takeover.
- Failure release must not lose a persisted credential.
- Aggregates carry stable identifiers, version, creation time and update time.
- Shared state is protected by mutexes and persisted updates use optimistic versions.
- All context propagation, lock waits, transactions, scans, retries and batch operations respond to cancellation.

## Layout

- cmd: executable entry, configuration assembly and offline self-check.
- domain: aggregates, value objects, state machine and domain errors.
- application: use case orchestration, transactions, idempotent and batch partial failure.
- repository: repository interfaces, local implementation, optimistic locking and deep copy isolation.
- journal: versioned, length and checksum protected write-ahead log.
- recovery: snapshot rotation, log replay, safe truncation and recovery validation.
- scheduler: background tasks, cancellation, timeout, retry and backoff.
- query: filtering, stable sorting, paging and derived queries.
- audit: audit events, hash chain and export verification.
- internal/clockidcodec: clock, identity, encoding and shared base constraints.
- tests: integration and black-box smoke tests.

## Build and test

```sh
go build ./...
go test ./...
go test -race ./...
go vet ./...
go run ./cmd/hwj-macgo-0018 --self-check
```

The offline self-check must pass before committing changes.

## Acceptance

- All domain legal and illegal state transitions are tested.
- Application tests cover at least a four-step business chain, idempotent replay, rollback, batch partial failure, context cancellation and error identity.
- Repository and recovery tests cover optimistic concurrency, deep copy, log corruption, snapshot recovery and reopening a directory.
- Concurrency tests pass under `go test -race`.
- Persistence decoding or recovery entry points have fuzz tests.
- Release acceptance runs `BENZHI` Docker without network access for linux/amd64 and linux/arm64.

See docs/PROJECT_DESIGN.md for the complete design baseline.
