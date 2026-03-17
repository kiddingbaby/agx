# AGX UX（UI/交互设计）

定位：AGX 不是聊天工具/会话管理器，而是 **“配置中心 + 同步器”**。  
目标用户：同时管理多个站点（官方/中转/自建）与大量 keys 的重度 AI CLI 用户。

## UX 目标（优先级从高到低）

1. **切换必须短**：用户只需要记住站点名，`agx use <site>` 即可。
2. **keys 管理必须顺手**：多把 key 的导入、激活、轮换策略可预测。
3. **无漂移**：默认不读取外部 shell env；只有显式引用时才读。
4. **站点中心**：用户以站点（target name）为中心操作，尽量不暴露“两个文件”的心智负担。
5. **可审计**：错误信息可行动；输出可脚本化（`-o json`）。

## 核心对象（面向用户的概念）

- **Site/Target（站点）**：用户自定义 name（例如 `openrouter`）。
- **Binding（绑定）**：`family -> target`。
- **Key scope**：`provider/profile`。
  - 官方：`profile=default`
  - 第三方：通常 `profile=<target-name>`（一个 base-url 下多 key 就落在同一个 profile）
  - 多协议网关（如 NewAPI/new-api）：`<site>-codex` / `<site>-claude` / `<site>-gemini` **复用同一份 keys**（只需管理一份）
    - 建站时可用 `agx create site <name> --template newapi --agents codex,claude` 只生成需要的 endpoint（避免误用不支持的 CLI）
- **Active key**：每个 `provider/profile` 只有一个 active（策略可控）。

## 配置优先级（避免漂移）

三层、可预期：

1) **运行时显式覆盖（可选）**  
例如：`agx use <site> --key <key>`（显式指定并激活该 site 的 key）

2) **AGX 配置中心（默认）**  
`~/.config/agx/keys.yaml` + `~/.config/agx/providers.yaml`

3) **用户 shell 环境（默认不参与）**  
外部 env **只有在显式指定时才读取**（agx.yml 的 `key-env` / `key: env:VAR`，或 CLI 的 `--api-key-env` 等）。

结果：切换不会“因为你今天 shell 里 export 了某个变量”而悄悄变更。

## 交互面（UI = CLI 输出 + 交互选择）

### 1) 切换（主路径）

- **输入**：`agx use <site>`
- **输出**：明确告诉用户：
  - 使用了哪个 target（name/family）
  - 该 target 对应的 profile
  - 使用了哪个 key（name + short id）
  - 同步写入了哪些下游原生配置文件（成功/失败）

错误体验（必须可行动）：

- `No active key`：提示 `agx create key --site <site> --stdin` 导入，或 `agx patch key <key> --site <site> --activate` 激活
- `target not found`：提示 `agx get sites` / `agx create site <name>`
- 提供安全网：切换前自动备份；必要时用户可用 `agx undo` 一键回滚

### 2) keys 管理（高频）

建议把 keys 的高频操作收敛成两条：

- 批量导入：`agx create key --site <site> --stdin`（粘贴即可）
- 激活选择：`agx patch key <key> --site <site> --activate`

策略设置（低频但必要）：

- 用 `agx apply` 写入 `profiles:`（`fixed|round_robin|random`）

### 3) 站点管理（低频但必须）

站点对象的典型生命周期：

1) `agx create site <name>`（模板建站）
2) `agx create key --site <name> --stdin` 导入 keys
3) `agx use <name>` 切换生效并同步

## 文案规范（输出/提示）

- 错误：以 `Error:` 开头，单行说明根因
- 指引：以 `Tip:` 开头，给 1 条可执行下一步命令
- JSON：`-o json` 永远只输出 JSON（stderr 可输出提示）

## 文件键名规范（YAML）

- 一律 kebab-case：`base-url` / `api-key` / `created-at` / `key-env` / `wire-api` …
- 参考：`docs/config-format.md`

## vNext（可选增强，不影响当前最小集）

如果要进一步降低输入成本（不引入完整 TUI），推荐两条路线：

1) **交互选择增强**：当 TTY 可用时，把“数字选项”升级为可搜索选择（例如集成 `fzf`，不存在则回退数字选择）。
2) **站点中心命令**：`agx create/patch/delete site` 已落地；后续可以把 `patch site` 做成更强的“表单式交互”。
