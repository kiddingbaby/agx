# AGX Security（密钥/配置与信任边界）

## TL;DR

- AGX 管理 **密钥 + provider 配置**，属于高风险面：默认把本地配置目录当作“受控机密区”。
- AGX 的默认配置根目录：`~/.config/agx/`（目录权限 0700，文件通常 0600）。
- 不要把任何 `~/.config/agx/*` 或下游 tool 配置文件提交进 git 或分享给不可信对象。

## 关键路径（默认）

由 `internal/config/paths.go` 解析的默认路径：

- `~/.config/agx/keys.yaml`：密钥与 profile 存储（写入权限 0600）
- `~/.config/agx/secret`：加密 secret（写入权限 0600）
- `~/.config/agx/providers.yaml`：provider 配置
- `~/.config/agx/backups/`：切换前的自动备份（**包含下游 tool 配置，可能含明文 key**）

以及可能会被读写/同步的下游工具配置：

- `~/.claude/settings.json`
- `~/.codex/auth.json`、`~/.codex/config.toml`
- `~/.gemini/.env`、`~/.gemini/settings.json`

## 加密 secret（必须理解）

AGX secret 的来源优先级：

1. 环境变量 `AGX_SECRET`（必须严格 32 bytes）
2. 文件 `~/.config/agx/secret`
3. 若既没有 secret 也没有 `keys.yaml`，会自动生成并写入 `~/.config/agx/secret`

注意：

- 如果存在 `keys.yaml` 但缺少 secret，会报错并提示迁移命令（避免“有数据但不可解密”）。

## 最小安全检查清单（推荐）

- [ ] `~/.config/agx/` 权限是否合理（目录 0700、文件 0600）？
- [ ] 是否避免在日志/工件中输出 key material？
- [ ] 是否避免把下游 tool 的 auth/config 文件复制到仓库或共享目录？
