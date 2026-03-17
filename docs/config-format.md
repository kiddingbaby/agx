# AGX Config Format（配置文件格式）

> 约定：**所有 YAML 键名使用 kebab-case**（例如 `base-url` / `api-key` / `created-at`）。
>
> 本仓不做历史兼容；旧的 snake_case（如 `base_url`）不再支持。

AGX 的“配置中心”由两份主文件 + 可选 `agx.yml` 组成：

- `~/.config/agx/keys.yaml`：keys + profile 轮转策略（**key 会被 AES-GCM 加密落盘**）
- `~/.config/agx/providers.yaml`：targets（站点）+ family bindings（当前绑定）
- `agx.yml`（可选，默认路径 `~/.config/agx/agx.yml`）：用于 `agx apply` 批量导入 keys/targets/bindings/profiles（也可包含 `assets:` 供 `agx sync` 使用）

## keys.yaml

顶层结构：

```yaml
keys: []
profiles: []
```

### keys[]

字段：

- `id`：UUID（由 AGX 生成）
- `provider`：`openai|claude|gemini`
- `profile`：key scope（通常：
  - 官方：`default`
  - 第三方站点：等于 target name，例如 `openrouter`）
- `name`：人类可读名称
- `api-key`：**加密后的密文**（不建议手动编辑）
- `base-url`：可选（用于 key 级别标注/过滤；真实同步以 target 为准）
- `tags`：可选
- `active`：是否为当前激活 key（同一 provider/profile 只应有一个）
- `created-at` / `updated-at`：时间戳

### profiles[]

字段：

- `provider`
- `name`：profile 名（等价于 `profile`）
- `strategy`：`fixed|round_robin|random`
- `fixed-key`：`strategy=fixed` 时指向某个 key `id`
- `next-index`：`strategy=round_robin` 的游标
- `updated-at`

## providers.yaml

顶层结构：

```yaml
targets: []
bindings: []
current-site: "" # 可选：当前 site/target（供 key 命令默认 scope 使用）
```

### targets[]

Target = “站点/endpoint”的显式化配置（用户最常用的对象）。

字段：

- `name`：target 名（第三方站点：你切换时用的 `<site>`；官方 target 为 `*-official`，用户侧别名为 `openai|claude|gemini`）
- `family`：`openai|claude|gemini`（面向 runtime 的 family）
- `kind`：`openai|openai-compatible|claude|gemini`（协议形态）
- `access`：`official|third_party`
- `auth`：目前为 `apikey`
- `base-url`：第三方必填；官方必须为空
- `model`：可选（固定模型）
- `env`：可选（同步到下游 CLI 的环境变量键值；**不用于存 key**）
- `wire-api`：OpenAI family 可选：`responses`（Codex CLI 目前仅支持 Responses API）
- `requires-openai-auth`：OpenAI family 可选（兼容某些中转）
- `created-at` / `updated-at`

内建官方 targets（默认存在；可被覆盖以设置 model/env/wire-api 等高级参数）：

- `openai-official` / `claude-official` / `gemini-official`

用户侧官方别名（切换时用这个更直觉）：

- `openai` / `claude` / `gemini`（会映射到对应 `*-official` target）

### bindings[]

Binding = “当前 family 用哪个 target”。

字段：

- `family`
- `target`
- `updated-at`

## agx.yml（agx apply）

agx.yml 用于一次性导入/更新配置中心（适合迁移、批量初始化、团队模板）。

顶层结构：

```yaml
keys: []
targets: []
bindings: []
profiles: []
```

### agx.yml keys[]

字段（与 keys.yaml 不同：agx.yml 可以携带明文/引用外部 secret）：

- `provider` / `profile` / `name`
- `key`：明文，或 `env:VAR` / `file:/path`
- `key-env`：从指定 env 读取（显式开启，避免漂移）
- `key-file`：从指定文件读取（显式开启）
- `base-url` / `tags` / `activate`

约束：

- `key` / `key-env` / `key-file` 三选一
- `activate: true` 会把该 key 设为该 scope 的 active

注意：

- `env:VAR` / `key-env` / `key-file` 只在 **导入时读取一次**；之后会把 key 加密写入 `keys.yaml`，切换时不会再读取外部 env（避免漂移）。

### agx.yml targets[] / bindings[] / profiles[]

字段与 `providers.yaml` / `keys.yaml` 一致（同名键）。

## agx.yml（agx sync）：assets

`agx.yml` 可选包含一个顶层 `assets:` 段，供 `agx sync` 使用（与 `agx apply` 无关；`agx apply` 会忽略未知字段）。

顶层结构（示例）：

```yaml
assets:
  skills-hub-home: "/path/to/skills-hub"  # SSOT 资产仓本地路径（建议）

  # system prompt：既支持单文件，也支持目录模式（推荐：目录模式）
  #
  # - file mode：填一个文件路径，所有 target 都会 link 到同一个文件
  # - dir mode：填一个目录路径，目录内需包含：
  #     - AGENTS.md  (codex)
  #     - CLAUDE.md  (claude)
  #     - GEMINI.md  (gemini)
  system-prompt-path: "system-prompt"     # 相对 skills-hub-home 或绝对路径
  system-prompt-links: [codex, claude, gemini] # 为空数组表示禁用

  skills:
    enabled: true
    source: "skills/tools"                      # 相对 skills-hub-home 或绝对路径
    targets: [codex, claude]
    prune: true                                 # true: 目标目录与 source 保持一致（推荐）

  mcp:
    enabled: true
    targets: [codex, claude]
    prune: true
    servers:
      - name: playwright
        command: ["npx", "-y", "@playwright/mcp@latest"]
        env: { }
      - name: context7
        command: ["npx", "-y", "@upstash/context7-mcp"]
      - name: example-http
        url: "https://example.com/mcp"
        bearer-token-env-var: "EXAMPLE_TOKEN"
```

说明：

- `skills-hub-home` 支持本地路径；git URL/自动拉取暂不内置（可先用外部脚本把仓库同步到本地路径）。
- `system-prompt-links: []` 可显式禁用 system prompt 同步（避免占位配置触发失败）。
- `mcp.servers[].command` 为数组：第一个元素为可执行文件，其余为 args（避免 shell split 漂移）。
