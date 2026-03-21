# AI Agent Instructions

## Read First

1. `README.md`
2. `docs/README.md`
3. `docs/user-guide.md`
4. `docs/directory-structure.md`
5. `internal/interfaces/cli/`
6. `internal/usecase/`

## Repo Scope

- `agx` 是独立的 env sync / provider switch 工具，不属于 Assist runtime。
- 优先保证 `agx use`、`agx sync`、`agx apply`、`agx init` 的 CLI 语义清晰，不为历史 prompt workflow 保留兼容层。

## Validation

- `go test ./...`
- `bash tests/integration/smoke-go.sh`
- 若改 CLI 流或原生配置同步，再补 `bash tests/e2e/cli-smoke.sh`
