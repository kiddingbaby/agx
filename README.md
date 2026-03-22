# AGX

AI CLI 配置切换器：统一管理多 provider 的 keys + endpoints（base-url/model/高级参数），并同步到各 CLI 的原生配置文件。

AGX **不负责会话管理**；你直接运行 `codex` / `claude` / `gemini`。

## 核心能力

- 多 provider：OpenAI / Claude / Gemini（以及 OpenAI-compatible 中转）
- Key 管理：`keys.yaml` 持久化 + AES-GCM 加密（明文仅在运行时内存）
- Site/Target 管理：`providers.yaml`（官方/第三方 base-url、model、wire-api、env 等）
- 一条命令切换：`agx use <site>` → 自动绑定 + 使用当前 active key → 同步原生配置（可用 `--agents` 一次同步多 CLI）

## 依赖

- Go 1.24+

## 快速开始

```bash
go build -o .tmp/agx ./cmd/agx
./.tmp/agx --help
```

## 极速上手（推荐）

目标：多 provider / 多 key 下做到「导入快、切换短」。

```bash
agx create site openrouter --template openrouter
agx create key --site openrouter --stdin
agx use openrouter
codex                       # 直接启动 codex
```

说明：

- 第三方 site 的 key 会落在同名 profile（例如 `openai/openrouter`），并维护一个 current active key。
- 切换 active key：`agx patch key <key> --site <site> --activate`，或切换时显式指定：`agx use <site> --key <key>`。

## 文档

- 用户最短路径：`docs/user-guide.md`
- 手动验收清单（给新用户照着敲）：`docs/manual-checklist.md`

## agx.yml（可选）

默认配置文件路径（供 `agx apply` / `agx sync` 自动发现）：

- `~/.config/agx/agx.yml` / `~/.config/agx/agx.yaml`

显式指定配置（更可控）：

```bash
agx apply agx.yml
agx apply --paste               # 直接粘贴 YAML
cat agx.yml | agx apply         # 自动识别 stdin（不需要 --stdin）
```

生成 agx.yml 模板（可选）：

```bash
agx init
```

说明：模板默认不包含任何 key（避免误读环境变量/明文落盘）；你可以用 `agx create key` 管理 key，或在 agx.yml 里启用 `keys:` 导入。

`assets:` 段里的路径语义建议这样理解：

- `assets-root`：相对路径解析锚点，不代表资产 owner
- `system-prompt-path`：system prompt 文件或目录源
- `skills.source`：待镜像的 skills 目录源

它们可以共用一个根，也可以分别写绝对路径。当前 `agent-stack` 示例里，system prompt 来自主线 `workspace/`，skills 仍可来自 bridge-kept 的 skill 资产目录；`agx` 只负责分发，不把这些资产重新定义成自己的产品边界。

agx.yml 格式示例（可批量导入 keys/targets/bindings/profiles）：

```yaml
keys:
  - provider: openai
    profile: default
    name: oai-01
    key-env: OPENAI_KEY_1      # 或 key-file: /path/to/key 或 key: "sk-..."
    tags: [work, primary]
    activate: true

targets:
  - name: openrouter
    family: openai
    kind: openai-compatible
    access: third_party
    base-url: https://openrouter.ai/api/v1
    wire-api: responses
    requires-openai-auth: false

bindings:
  - family: openai
    target: openrouter

profiles:
  - provider: openai
    profile: default
    strategy: round_robin
```

## 配置文件位置

- `~/.config/agx/keys.yaml`（AES-GCM 加密保存 key）
- `~/.config/agx/providers.yaml`（site/target 定义 + family 绑定）
- `~/.config/agx/secret`（或用 `AGX_SECRET` 提供 32 bytes）

## 常用命令

```bash
agx                         # Show status (bindings + active keys)
agx status                  # Same as above
agx use <site>              # Switch current site (supports --key, --agents)
agx undo                    # Undo last switch (restore previous configs)
agx get sites               # List sites
agx create site ...         # Create site
agx patch site ...          # Patch site
agx delete site ...         # Delete/reset site
agx get keys                # List keys (default to current site)
agx create key ...          # Add/import keys
agx patch key ...           # Rename/tag/activate key
agx delete key ...          # Remove key
agx sync                    # Sync global agent assets (system prompts / skills / MCP)
agx apply [agx.yml]         # Apply config into AGX stores
agx init                    # Generate agx.yml template
```

## Provider/Target

- 内建官方 site：`openai` / `claude` / `gemini`（内部 target 名：`*-official`）
- 可覆盖官方默认高级参数：`agx patch site openai|claude|gemini ...`（`--reset` 恢复默认）
- 自定义 target 落在 `~/.config/agx/providers.yaml`
- 可选字段：`base-url` / `model` / `env` / `wire-api` / `requires-openai-auth`

## 配置优先级（避免漂移）

- AGX 配置中心：`~/.config/agx/keys.yaml` + `~/.config/agx/providers.yaml`
- Shell 环境变量：默认不读取；仅当你显式指定（如 `env:VAR` / `--api-key-env VAR` / `--api-key-file PATH`）才会读取
- 切换时：`agx use <site>` 会写入各 CLI 原生配置（Codex/Claude/Gemini）

## 同步到原生配置

- `claude`：写入 `~/.claude/settings.json`
- `codex`：写入 `~/.codex/auth.json`、维护 `~/.codex/config.toml`（managed provider block）
- `gemini`：写入 `~/.gemini/settings.json`、维护 `~/.gemini/.env`

## 架构分层（当前）

```text
cmd
  -> interfaces (cli)
    -> app/bootstrap
      -> usecase
        -> domain + ports
          -> adapters (configfile/keyfile/toolconfig)
```

## 目录概览

```text
cmd/agx/
internal/
  adapters/        # configfile, keyfile, toolconfig
  app/             # bootstrap + container
  config/          # paths + secret provider
  domain/          # agent, key, provider
  interfaces/      # cli
  ports/           # repository interfaces
  usecase/         # key/provider/switch + unified errors
```

## 回归基线

```bash
go test ./...
bash tests/integration/smoke-go.sh
```

`tests/e2e/cli-smoke.sh` 会在临时 `HOME` 下执行最小 CLI 冒烟（不污染本机配置）。

## Docs

- 入口：`docs/README.md`
- 上手与操作：`docs/user-guide.md`
- 配置格式：`docs/config-format.md`
- UX 设计：`docs/ux.md`
- 视角页：`docs/product.md`、`docs/architecture.md`、`docs/ops.md`

## Workflow

- 使用说明：`docs/workflow.md`
- 配置源：`.workflow/config/workflow.yml`
- 当前只保留 `.workflow/docs/` 下的活跃文档；archive/report/state 等运行留档不纳入仓库基线
