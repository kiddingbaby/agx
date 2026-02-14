# AGX V1 设计文档（Key 交互 + Env 托管）

## Problem

当前要解决的核心问题：

1. `key` 管理缺少完整 CRUD（特别是 Edit），用户需要快速键盘操作。
2. 环境变量使用体验不友好，用户不希望修改 `~/.bashrc`/`~/.zshrc` 等系统环境文件。
3. 交互风格需要统一，是否全局 Vim 风格需要明确策略。

用户价值目标（V1）：

1. 30 秒内完成 key 新增并可立即启动 agent。
2. 启动时由 AGX 自动注入所需 env，不要求用户手工 `export`。
3. 全流程键盘可达，低误操作（尤其 Delete）。

---

## Architecture

### 1. Key 管理交互架构

采用三态模型：`List` / `Form(Add|Edit)` / `Confirm(Delete)`。

#### List（主界面）

- Provider 分组展示（Claude/OpenAI/Gemini）。
- 行类型：`provider header` + `key item`。
- 显示字段：`active`、`name`、`tags`、`updated_at`。
- 支持快速过滤：`/` 进入搜索（按 name/tag/provider）。

关键操作：

- `a`：新增（默认继承当前 provider）。
- `e`：编辑（进入 Form，回填现有值）。
- `Enter`：设为 active。
- `d`：删除（进入 Confirm）。
- `/`：搜索过滤。

#### Form（新增/编辑共用）

字段顺序：

1. Provider（新增可改，编辑默认锁定或二次确认）
2. Name（必填）
3. Base URL（可选）
4. API Key（必填；编辑时支持“保持原值”）
5. Tags（可选）

表单策略：

- 新增默认聚焦 `Name`，减少无效跳转。
- 编辑回填，`API Key` 默认留空表示不改动。
- 保存时做最小校验：必填、Provider 合法、URL 格式。

#### Confirm（删除确认）

- 默认焦点 `Cancel`，防误删。
- 文案展示 `provider/name`。
- 二次确认策略：V1 使用显式确认弹窗；暂不做 Undo 队列（复杂度高）。

### 2. 统一键位策略（Vim + 通用双轨）

结论：**全界面支持 Vim 风格，但不强制仅 Vim。**

- Vim：`j/k` 上下，`h/l` 左右，`gg/G` 首尾，`/` 搜索，`Esc` 返回。
- 通用：方向键、`Tab/Shift+Tab`、`Enter` 保持可用。

理由：

1. 满足 power user 效率诉求。
2. 不牺牲新用户可用性。
3. 降低“必须会 Vim”门槛，减少支持成本。

### 3. Env 注入架构（完全由 AGX 托管）

设计目标：用户无需修改系统环境变量文件。

#### 配置与密钥来源

1. API Key / Base URL 存储：`~/.config/agx/keys.yaml`（API Key 加密）。
2. 加密主密钥：`~/.config/agx/secret` 自动生成并 `0600`。
3. `AGX_SECRET` 仅作为可选覆盖，不再作为默认前置步骤。

#### 运行时注入

- 启动 agent 时，AGX 从 active key 组装 env map：
  - Claude: `ANTHROPIC_API_KEY` / `ANTHROPIC_BASE_URL`
  - OpenAI: `OPENAI_API_KEY` / `OPENAI_API_BASE`
  - Gemini: `GOOGLE_API_KEY` / `GEMINI_BASE_URL`
- 注入范围为 AGX 启动的 agent 会话，不污染用户全局 shell 环境。

#### 安全改进（必须落地）

当前实现通过 `export KEY=... && cmd` 拼接命令，有明文暴露风险（进程参数/调试输出）。

V1 要求改为：

1. 避免把明文密钥拼进命令行字符串。
2. 使用 tmux 原生环境注入能力（可用时）或等价安全方案。
3. 日志和错误信息默认脱敏（仅显示前后少量字符）。

---

## Implementation

### 1. 功能优先级（V1 生产）

P0（必须）：

1. Key `Add/Edit/Delete/Activate/List` 完整闭环。
2. 双轨键位（Vim + 通用）在 Dashboard/KeyMgr/Form/Confirm 全覆盖。
3. AGX 托管 env 注入，默认不需要改 shell 配置文件。
4. 启动链路安全注入（不明文拼接 secret 到命令行）。
5. 关键错误提示可行动（例如“无 active key，按 K 去配置”）。

P1（建议）：

1. Key 搜索/过滤与 tags 管理优化。
2. `agx keys edit` CLI 子命令（非交互）。
3. 启动前 env preview（值脱敏）用于排障。

P2（后续）：

1. 项目级 profile（同 provider 多环境切换）。
2. 外部 Secret Manager 对接（1Password/Vault/KMS）。

### 2. 用户故事（验收口径）

1. 作为开发者，我可以在 `agx keys` 里只用键盘完成 key 新增/编辑/删除。
2. 作为开发者，我执行 `agx claude` 时不需要预先改 `~/.bashrc`，也能自动带上正确 key。
3. 作为开发者，我可以统一使用 `hjkl` 在所有界面操作，同时方向键也可工作。
4. 作为维护者，我不能在日志或命令行参数中看到完整 API Key。

### 3. 非目标（V1 不做）

1. 多人协作密钥共享与权限模型。
2. 云端同步与远程密钥托管。
3. 删除后可恢复（Undo/Trash）机制。

### 4. 约束与风险

约束：

1. 技术栈保持 Go + Bubble Tea + tmux。
2. 保持现有 keys 存储格式可兼容迁移。

风险：

1. tmux 版本差异导致环境注入能力不一致。
2. Edit 语义若处理不清，可能误覆盖 API Key。

缓解：

1. 启动时探测 tmux 能力并降级。
2. Edit 表单中“空 API Key = 保持不变”并加明确提示。

### 5. 里程碑建议

1. M1：交互补齐（`e` 编辑 + 全局键位一致性）。
2. M2：安全 env 注入改造 + 脱敏日志。
3. M3：CLI `keys edit` + 搜索过滤。
