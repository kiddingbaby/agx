# AGX Upgrade（版本演进与迁移提示）

## 当前阶段声明

AGX 处于快速演进期，默认不承诺历史兼容层。升级策略以“锁定 commit + 跑回归 + 检查本地配置”作为最小原则。

## 升级前检查（必做）

```bash
go test ./...
bash tests/integration/smoke-go.sh
bash tests/e2e/cli-smoke.sh
```

## 常见迁移点：已有 `keys.yaml` 但缺少 secret

如果你已经有 `~/.config/agx/keys.yaml`，但没有 `~/.config/agx/secret`（或未设置 `AGX_SECRET`），AGX 会拒绝启动并提示迁移命令。

建议做法：

- 先确认 `AGX_SECRET`（严格 32 bytes）来源可靠且可备份
- 然后按提示把它写入 `~/.config/agx/secret`（文件权限建议 0600）

## 配置路径变更

默认路径集中在 `~/.config/agx/`，若后续版本调整路径或文件名，应以 `docs/directory-structure.md` 与代码中的 `internal/config/paths.go` 为准。

## YAML 键名规范

AGX 的 YAML 配置键名统一为 kebab-case（例如 `base-url` / `api-key` / `created-at`）。  
本项目不提供旧 snake_case（如 `base_url`）兼容层。
