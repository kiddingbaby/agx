# AGX Troubleshooting（故障定位）

## TL;DR（先跑这四条）

```bash
agx --help
agx status
agx get sites
agx get keys --site openai
```

## 常见问题

### 1) 误以为 AGX 负责启动/会话管理

AGX 不负责会话管理；请直接运行 `codex` / `claude` / `gemini`。

### 2) `use` 失败：No active key

解决：

- 先导入 key：`agx create key --site <site> --stdin`（或用 `--api-key-env/--api-key-file`）
- 再切换站点：`agx use <site>`

### 3) `apply` 失败：agx.yml YAML 解析或字段不符合预期

定位路径：

1. 先确认 agx.yml 文件路径是否正确（或是否在 `--paste` 模式下粘贴完整 YAML）。
2. 再检查 YAML 是否可解析（缩进/引号/列表符号）。
3. 若你启用了 `key-env` / `key-file`：确认环境变量/文件确实存在。

### 4) 解密失败 / secret 不匹配

常见原因：

- 你更换了 `~/.config/agx/secret` 或环境变量 `AGX_SECRET`，导致历史加密数据无法解密。

建议：

- 保持同一台机器上的 secret 稳定；需要迁移时用同一 secret。
- 若你确认不需要旧数据：清理 `~/.config/agx/` 后重新 `agx apply` 导入（注意备份）。

### 5) 原生 CLI 配置没有更新

定位路径：

- 重试切换：`agx use <site>`
- 检查目标配置文件是否可写：
  - Codex：`~/.codex/auth.json`、`~/.codex/config.toml`
  - Claude：`~/.claude/settings.json`
  - Gemini：`~/.gemini/settings.json`、`~/.gemini/.env`

### 6) 交互选择不可用（CI/非 TTY）

提示：

- `agx create key --stdin` 在无 TTY 时需要 piped stdin；CI 建议改用显式 flags（例如 `agx create key <name> --site <site> --api-key-env VAR --activate`）。

### 7) `codex` 报错：Missing environment variable: OPENAI_API_KEY

原因：

- 你当前的 `~/.codex/config.toml` 里某个 `model_providers.*` block 配了 `env_key = "OPENAI_API_KEY"`，Codex 会强制要求该环境变量存在（不会从 `~/.codex/auth.json` 读取）。

解决：

- 升级到最新 AGX 后重新执行一次：`agx use <site>`（AGX 会写回一个不依赖 `env_key` 的 managed block）
- 或手动从 `~/.codex/config.toml` 中移除 `env_key = "OPENAI_API_KEY"`（只要你使用的是 `auth.json` 存 key 的模式）

## 反馈/交接时建议附带的信息

- `agx --help` 输出
- `agx get sites`、`agx get keys --site <site>` 输出（必要时脱敏）
- 相关配置文件路径是否存在/可写（不要贴明文 key）
