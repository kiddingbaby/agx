# AGX User Guide（用户操作指南）

目标：在多 provider / 多站点 / 多 key 场景下做到 **导入快、切换短、无漂移**。

如果你需要给用户一份“照着敲命令就能验收”的清单：见 `docs/manual-checklist.md`。

## 0) 先记住 2 个命令（就够用了）

- 切换：`agx use <site>`（官方站点缺 key 时会在当前命令内提示你粘贴 keys）
- 建站：`agx create site <name>`（中转/自定义站点：交互式选模板 + 填 base-url，可选导入 keys）
- （可选）导入 key：`agx create key --site <site> --stdin`（或先 `agx use <site>`，再省略 `--site`）
- （可选）迁移 Claude 当前配置：`agx import claude`（从 `~/.claude/settings.json` 读 key/base-url/model，并导入到 AGX）

AGX 只做一件事：**把你选中的 site + active key 同步写入各 CLI 的原生配置文件**，你再直接运行 `codex|claude|gemini`。

## 1) 没有配置（引导 / 上手）

### 1.1 OpenRouter：建站并使用（OpenAI-compatible）

```bash
agx create site openrouter --template openrouter
agx create key --site openrouter --stdin
agx use openrouter
codex
```

### 1.2 OpenAI 官方号：新增 key 并使用

（官方 site 内建，不需要 `agx create site`）

```bash
agx use openai           # 若未配置 key，会在当前命令内提示你粘贴 keys
codex
```

### 1.3 Claude 官方号：新增 key 并使用

（官方 site 内建，不需要 `agx create site`）

```bash
agx use claude           # 若未配置 key，会在当前命令内提示你粘贴 keys
claude
```

### 1.4 Gemini 官方号：新增 key 并使用

（官方 site 内建，不需要 `agx create site`）

```bash
agx use gemini           # 若未配置 key，会在当前命令内提示你粘贴 keys
gemini
```

### 1.5 自定义中转（base-url）

OpenAI-compatible：

```bash
agx create site my-oai-proxy   # 交互式：选模板/填 base-url +（可选）导入 keys
agx use my-oai-proxy
codex
```

例如 NewAPI：按其文档，OpenAI Responses 兼容的 base-url 通常为 `https://<你的newapi服务器地址>/v1`（Codex 会访问 `<base-url>/responses`，也就是 `.../v1/responses`）。

补充：如果你用的是 NewAPI / new-api 这一类“多协议中转”（同时提供 OpenAI / Anthropic / Gemini 兼容接口），通常是：

- **OpenAI-compatible（给 `codex` 用）**：`https://<host>/v1`（AGX 支持只填 `https://<host>`，会自动补 `/v1`）
- **Claude-compatible（给 `claude` 用）**：`https://<host>`（其后拼 `/v1/messages`；如果你误填了 `.../v1`，AGX 会自动去掉）
- **Gemini-compatible（给 `gemini` 用）**：`https://<host>`（其后拼 `/v1beta/models/...:generateContent`；如果你误填了 `.../v1`，AGX 会自动去掉）

如果你的中转站只提供 OpenAI-compatible（很多社区项目是这种路线），那你只需要配一个 OpenAI-compatible site，然后用 `codex` 访问各种模型即可。

NewAPI / new-api 快捷模板（推荐）：只填一次 host、粘贴一次 key，按 `--agents` 选择创建哪些 endpoint（默认 `all`，即 `codex/claude/gemini`）。

注意：`<name>` 只是前缀，不是一个可 `use` 的 site 名；请使用 `-codex/-claude/-gemini` 三选一（或配合 `--agents` 一次同步多个）。

默认 `agx use <name>-codex` **只会同步 codex**（避免污染你没用/不支持的 CLI 配置）；当你确认这个中转站支持多协议时，再用 `--agents` 选择要同步的 CLI（支持子集）：

```bash
agx create site my-newapi --template newapi
# 只需要 codex/claude（不生成 gemini endpoint）：
agx create site my-newapi-lite --template newapi --agents codex,claude

agx create key --site my-newapi-codex --stdin   # 或直接 agx use 时按提示粘贴
agx use my-newapi-codex         # codex only
agx use my-newapi-codex --agents all   # sync codex/claude/gemini (explicit)
agx use my-newapi-codex --agents codex,claude   # sync subset

codex
claude
gemini
```

Claude：

```bash
agx create site my-claude-proxy
agx use my-claude-proxy
claude
```

Gemini：

```bash
agx create site my-gemini-proxy
agx use my-gemini-proxy
gemini
```

## 2) 已配置好 key（日常使用，极简）

### 2.1 切换（必须短）

```bash
agx use <site>
```

如果你想“切换 site + 顺便换 key”（一条命令）：

```bash
agx use <site> --key <key>
```

如果你给 key 打了 tag，也可以直接按 tag 选择（AND 匹配）：

```bash
agx use <site> -l work,primary
```

### 2.2 管理 keys（一个 site 一坨）

导入多把 key（粘贴，多行空行结束）：

```bash
agx create key --site <site> --stdin
```

激活某个 key：

```bash
agx patch key <key> --site <site> --activate
```

给 key 打 tag（覆盖写入）：

```bash
agx patch key <key> --site <site> --tags work,primary
```

列出该 site 下所有 key：

```bash
agx get keys --site <site>
```

按 tag 筛 key：

```bash
agx get keys --site <site> -l work,primary
```

### 2.3 预演（dry-run，不写文件）

只看“将要写入哪些原生配置文件 + 会用哪把 key”，不实际写入：

```bash
agx use <site> --dry-run
```

交互式选择时也可以用 `--agents` 过滤列表（例如只看 Claude 的 sites）：

```bash
agx use --agents claude
```

### 2.3 一组 key 的轮换策略（可选）

```bash
cat <<'YAML' | agx apply --stdin
profiles:
  - provider: openai
    profile: <site>
    strategy: round_robin
YAML
```

## 3) 高级：覆盖官方默认（model/wire-api/env）

OpenAI（官方）示例：

```bash
agx patch site openai --model gpt-4.1
```

恢复官方默认（删除 override）：

```bash
agx patch site openai --reset
```

## 4) agx.yml（可选 / 团队化）

生成模板：

```bash
agx init
```

应用导入：

```bash
agx apply ~/.config/agx/agx.yml
```

## 5) 全局资产同步（system prompts / skills / MCP）

> 目标：把 **全局资产** 同步到各 CLI 的约定目录（`~/.codex` / `~/.claude` / `~/.gemini`），让你在任意项目里都能稳定触发 `$do/$review/$vibe-guardrails` 等 skills，并统一 system prompt。

`agx sync` 读取 `agx.yml` 里的 `assets:` 段（与 `agx apply` 无关），并执行三类动作：

- system prompt：创建/修复 `~/.codex/AGENTS.md`、`~/.claude/CLAUDE.md`、`~/.gemini/GEMINI.md` 的 symlink
- skills：镜像 `skills.source` 指定的目录到 `~/.codex/skills/tools` 与 `~/.claude/skills/tools`
- MCP（可选）：按配置 add/remove/replace MCP servers（codex/claude）

路径语义只有三层：

- `assets-root`：相对路径解析锚点
- `system-prompt-path`：system prompt 文件或目录源
- `skills.source`：要镜像到各 CLI home 的 skills 目录源

它们可以共用一个根，也可以分别写成绝对路径；`agx sync` 不要求 system prompt 和 skills 属于同一个物理子树。

示例（推荐把配置放到默认位置：`~/.config/agx/agx.yml`，之后在任意目录只需执行 `agx sync`）：

```bash
agx sync
```

排查/预览（不会写入）：

```bash
agx sync --dry-run -o json
```

## 6) 切换会写哪些文件（原生配置同步）

`agx use <site>` 会把“当前 site + active key”同步写入各 CLI 的原生配置文件：

- `codex`：`~/.codex/auth.json`、`~/.codex/config.toml`
- `claude`：`~/.claude/settings.json`
- `gemini`：`~/.gemini/settings.json`、`~/.gemini/.env`

官方 vs 中转的关键差异：中转 site 的 `target.base-url` 会被写入对应 CLI；官方 site 的 `base-url` 为空（走官方默认）。

安全网：

- 每次切换前，AGX 会自动备份受影响的文件到 `~/.config/agx/backups/`。
- 需要回滚时：`agx undo`（恢复到上一次切换前的状态）。

## 7) 代替 CC Switch（Claude Code）

CC Switch 的核心能力（切 key/base-url/model）在 AGX 里对应：

- provider 配置（base-url/model/env）→ `site`
- 多把 key 管理（含 tags）→ `keys`
- 一键切换 & 回滚 → `agx use ...` + `agx undo`

推荐迁移路径（最省事）：

1) 导入你当前 Claude Code 正在用的配置（不会写回原生配置，只是把 key/base-url/model 导入到 AGX）：

```bash
agx import claude
```

2) 切到 AGX 管理的 site：

```bash
agx use <site>
claude
```

如果你是 NewAPI/new-api 这类“多协议同域名”网关用户，建议先建通用网关 sites，再把当前 key 导入到共享 key scope：

```bash
agx create site my-gw --template newapi --base-url https://host --agents codex,claude
agx import claude --site my-gw-codex
agx use my-gw-codex --agents codex,claude
```
