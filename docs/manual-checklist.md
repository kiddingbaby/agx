# AGX Manual Checklist（手动验收清单）

> 目的：让第一次接触 AGX 的用户，手动跑一遍“多 provider / 多站点 / 多 key”的关键路径，确认 **导入快、切换短、无漂移、可回滚**。

## 安全提醒（先看）

- `agx use <site>` 会直接改写这些原生配置文件（请避免在它们运行中切换；必要时重启对应 CLI）：
  - codex：`~/.codex/auth.json`、`~/.codex/config.toml`
  - claude：`~/.claude/settings.json`
  - gemini：`~/.gemini/settings.json`、`~/.gemini/.env`
- 每次切换前会自动备份到 `~/.config/agx/backups/`；出问题可用 `agx undo` 回滚到上一次切换前。

## 0) 基线检查（必做）

```bash
agx status
agx get sites
```

预期：
- 能看到 `openai / claude / gemini` 三个官方站点（内建，无需建站）。

## 1) OpenAI 官方号（官方站点 + 1 把 key）

把 `<OPENAI_KEY>` 换成你的 key（或随便填一个非空字符串用于验证写入）。

```bash
agx create key oai-1 --site openai --api-key <OPENAI_KEY> --activate
agx use openai
codex
```

检查点（可选）：

```bash
grep -n 'auth_mode' ~/.codex/auth.json
grep -n 'model_provider = "agx"' ~/.codex/config.toml
```

## 2) OpenRouter（OpenAI-compatible 中转 + 2 把 key + 快速切换 key）

把 `<OPENROUTER_KEY_1>` / `<OPENROUTER_KEY_2>` 换成你的 key（或随便填非空字符串）。

```bash
agx create site openrouter --template openrouter --no-keys
agx create key or-1 --site openrouter --api-key <OPENROUTER_KEY_1> --activate
agx create key or-2 --site openrouter --api-key <OPENROUTER_KEY_2>
agx use openrouter
codex
```

“切换站点 + 顺便换 key”（一条命令）：

```bash
agx use openrouter --key or-2
```

检查点（可选）：

```bash
grep -n 'base_url' ~/.codex/config.toml
grep -n 'wire_api' ~/.codex/config.toml
```

## 3) 多 key 轮换策略（round_robin）

```bash
cat <<'YAML' | agx apply --stdin
profiles:
  - provider: openai
    profile: openrouter
    strategy: round_robin
YAML
agx use openrouter
agx status
agx use openrouter
agx status
```

预期：
- 两次 `agx openrouter` 后，`agx status` 里 openai 绑定的 active key 会在 `or-1 / or-2` 间轮换。

## 4) 站点高级参数（wire-api / requires-openai-auth）

```bash
agx patch site openrouter --wire-api responses --requires-openai-auth
agx use openrouter
```

检查点（可选）：

```bash
grep -n 'wire_api' ~/.codex/config.toml
grep -n 'requires_openai_auth' ~/.codex/config.toml
```

## 5) 回滚（undo）

```bash
agx undo
agx status
```

预期：
- 回到上一次切换前的状态（包含原生配置文件与 AGX 绑定/active key）。

## 6) Claude：官方 + 中转（base-url）+ env 注入 + 不漂移

把 `<CLAUDE_BASE_URL>` / `<CLAUDE_KEY>` 换成你的实际值（或随便填非空字符串用于验证写入）。

```bash
agx create site claude-proxy --template claude-proxy --base-url <CLAUDE_BASE_URL> --no-keys
agx create key cl-1 --site claude-proxy --api-key <CLAUDE_KEY> --activate
agx patch site claude-proxy --env FOO=bar --env BAR=baz --model sonnet
agx use claude-proxy
claude
```

清掉其中一个 env（验证“不漂移”：被 AGX 管理的 env 会被移除，但用户原有 env 不受影响）：

```bash
agx patch site claude-proxy --env-unset BAR
agx use claude-proxy
```

全部清掉：

```bash
agx patch site claude-proxy --clear-env
agx use claude-proxy
```

切回官方（base-url 应该被移除）：

```bash
agx create key cl-official --site claude --api-key <CLAUDE_KEY> --activate
agx use claude
claude
```

检查点（可选）：

```bash
grep -n 'ANTHROPIC_BASE_URL' ~/.claude/settings.json || true
```

## 7) Gemini：官方 + 中转（base-url）+ env 注入

把 `<GEMINI_BASE_URL>` / `<GEMINI_KEY>` 换成你的实际值（或随便填非空字符串用于验证写入）。

```bash
agx create site gemini-proxy --template gemini-proxy --base-url <GEMINI_BASE_URL> --no-keys
agx create key gm-1 --site gemini-proxy --api-key <GEMINI_KEY> --activate
agx patch site gemini-proxy --env FOO=bar --model gemini-2.0-pro
agx use gemini-proxy
gemini
```

检查点（可选）：

```bash
grep -n 'GEMINI_API_KEY' ~/.gemini/.env
grep -n 'GEMINI_BASE_URL\\|GOOGLE_GEMINI_BASE_URL' ~/.gemini/.env
```

## 8) Apply（agx.yml：显式读取 env，一次导入）

> 要点：外部 env **只在你显式指定**（`key-env`）时读取；并且只在导入时读取一次，避免漂移。

```bash
export OAI_KEY_ENV_TEST=<OPENAI_KEY>
agx apply --paste
```

粘贴（示例，把 `OAI_KEY_ENV_TEST` 改成你自己的 env 变量名）：

```yaml
keys:
  - provider: openai
    profile: default
    name: oai-env
    key-env: OAI_KEY_ENV_TEST
    activate: true
```

然后 Ctrl-D 结束粘贴，再执行：

```bash
unset OAI_KEY_ENV_TEST
agx use openai
```

## 9) 清理（可选）

> 删除第三方站点前，需要先切回官方，解除绑定。

```bash
agx use openai
agx use claude
agx use gemini
agx delete site openrouter
agx delete site claude-proxy
agx delete site gemini-proxy
```
