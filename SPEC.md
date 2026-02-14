# AGX V1 技术规格（Key CRUD + Env 托管 + 键位一致性）

## 1. Scope

本次 V1 交付范围（与 `DESIGN.md` 对齐）：

1. Key 管理：在 TUI 中补齐 `Add/Edit/Delete/Activate`，并保持全键盘可操作。
2. Env 注入：由 AGX 托管，用户无需修改 shell 环境文件；启动时注入 API Key 与可选 Base URL。
3. 键位：全界面支持 Vim 风格（hjkl/jk/gg/G/`/`），同时保留方向键/Tab/Enter 的通用操作。
4. 安全：**禁止**将明文 API Key 拼接进 pane 内执行的 shell 命令串（避免历史/回显/复制泄漏）。

非目标（V1 不做）：

1. 云端同步/团队共享密钥。
2. 删除恢复（Trash/Undo）。
3. 外部 Secret Manager 对接（Vault/1Password/KMS）。

---

## 2. Data Model

### 2.1 `internal/key.Key`

现状：

- `APIKey` 字段是 AES-GCM 加密后的 base64 字符串。
- `CreatedAt` 仅在创建时写入。

V1 变更：

1. 新增 `UpdatedAt time.Time`，用于列表展示最近变更时间与 edit 逻辑验收。
2. YAML 兼容：旧数据缺失 `updated_at` 时，视为 `CreatedAt`。

```go
type Key struct {
    ID        string    `yaml:"id"`
    Provider  Provider  `yaml:"provider"`
    Name      string    `yaml:"name"`
    APIKey    string    `yaml:"api_key"`            // encrypted (base64)
    BaseURL   string    `yaml:"base_url,omitempty"` // plaintext, optional
    Tags      []string  `yaml:"tags,omitempty"`
    Active    bool      `yaml:"active"`
    CreatedAt time.Time `yaml:"created_at"`
    UpdatedAt time.Time `yaml:"updated_at,omitempty"`
}
```

---

## 3. Key Store API

文件：`internal/key/store.go`

新增/调整接口：

1. `Update(id string, patch UpdatePatch) error`
2. （可选）`Get(id string) (*Key, error)` 便于 TUI/CLI 回填

Patch 语义（避免误覆盖）：

- `Name/BaseURL/Tags`：如果 patch 标记为 set 则覆盖；未 set 则保持原值。
- `APIKey`：只有在 patch 提供明文且非空时才重新加密写入；空值代表“不修改”。
- `Provider`：V1 仅支持保持不变；若要改 provider，要求显式二次确认（先不做，避免激活语义混乱）。
- 更新 `UpdatedAt = time.Now()`；`CreatedAt` 不变。

激活语义保持不变：

- `Activate(id)`：同 provider 内唯一 active。
- `Delete(id)`：删除 active key 后不自动激活其他 key（保持现状；由用户显式选择）。

---

## 4. TUI: Key Manager 交互规格

文件：`internal/tui/keymgr.go`

### 4.1 View 状态机

维持三态，但补齐 Edit 与搜索：

- `List`
- `Form(Add|Edit)`：同一套表单；新增 `formMode` 与 `editingKeyID`
- `Confirm(Delete)`

### 4.2 List 行为

新增 key binding：

- `e`：编辑当前选中的 key item（header 行无效）。
- `/`：进入过滤输入（按 name/tags/provider 过滤 `keyRows`）。
- `gg`：跳到第一行；`G`：跳到最后一行。

保持：

- `j/k` 或 `↑/↓`：移动
- `Enter`：Activate
- `a`：Add（继承当前 provider）
- `d`：Delete（进入 Confirm）
- `Esc`：返回 Dashboard

列表展示字段：

- `*` active 标记
- `name`
- `tags`
- `UpdatedAt`（无则显示 `CreatedAt`）

### 4.3 Form 行为（Add/Edit 共用）

进入方式：

- Add：`a`，`Provider` 继承当前 provider header 或 key item 的 provider；焦点到 `Name`。
- Edit：`e`，回填 `Provider/Name/BaseURL/Tags`；`APIKey` 置空并显示提示 “留空表示不修改”；焦点到 `Name`。

保存校验：

- Add：`Name` 与 `APIKey` 必填。
- Edit：`Name` 必填；`APIKey` 可空（空=保持原值）。

导航：

- Vim：`h/l`（provider 切换）、`j/k`（字段切换，可选实现）、`Esc` 取消。
- 通用：`Tab/Shift+Tab` 切换字段，`Enter` 保存，方向键输入框内按 bubbles 默认行为。

### 4.4 Confirm(Delete) 行为

- 默认选中 `Cancel`
- `Tab` 或 `h/l` 切换按钮
- `Enter` 执行选中按钮

---

## 5. CLI: keys 子命令

文件：`cmd/agx/main.go`

V1 必须保持现有命令不破坏：

- `agx keys ls`
- `agx keys add ...`
- `agx keys activate <id|name>`
- `agx keys delete <id|name>`

V1 可选增强（P1，不阻塞）：

- `agx keys edit <id|name> [--name N] [--key K] [--base-url URL] [--tags T]`
  - `--key` 缺省表示不改动

---

## 6. Env 注入与启动安全

文件：`internal/session/orchestrator.go`，调用点：`cmd/agx/main.go`、`cmd/agx/tui.go`

现状风险：

- 通过 `export X=SECRET && cmd` 拼接 `shellCmd`，会让 secret 出现在 pane 内部命令串，存在回显/复制/历史风险。

V1 目标：

1. pane 内运行的命令串不包含明文 secret。
2. env 注入由 AGX 控制且仅作用于 AGX 创建的 tmux session（不污染全局 shell）。

推荐实现（tmux server/session env + respawn）：

1. 创建/复用 session：`ai-<agent>`
2. 在 session 上设置环境变量（API Key/Base URL）：
   - `tmux set-environment -t <session> <VAR> <VALUE>`
3. 创建 window（先用默认 shell/空 command 创建，避免传入复杂 shellCmd）
4. 使用 `tmux respawn-pane -k -t <session>:<window> <command>` 启动 agent 命令（命令不包含 secret）
5. attach/switch-client

说明：

- `set-environment` 仍会让 secret 进入 tmux server 的环境表；这是 V1 可接受的权衡（避免落入 shell history/输出），并且 scope 限定在 `ai-<agent>` session。

---

## 7. 测试与验收

### 7.1 单元测试

- `internal/key/store_test.go`
  - Update：不改 key 时保持 APIKey 不变；改 key 时可正确解密；UpdatedAt 更新；CreatedAt 不变。
- `internal/tui/helpers_test.go`（如需要扩展）
- `internal/session/orchestrator_test.go`
  - 若移除/弱化 `escapeForShell` 使用，需要同步测试策略（保留函数也可，但不再用于注入 secret）。

### 7.2 手工验收（最小路径）

1. `agx keys`：新增一条 key 并激活。
2. `agx keys`：按 `e` 编辑 name/tags/baseURL；APIKey 留空保存，启动仍可用。
3. `agx <agent>`：无需设置系统环境变量文件即可启动；pane 内看不到 `export XXX=...`。
4. `agx keys`：删除 key 需要显式确认，默认 Cancel。

---

## 8. V1 补充优化（代码审查发现）

代码审查中发现的 bug、性能问题和 UX 细节，全部纳入 V1 范围。

### 8.1 Bug 修复

1. **`truncate()` Unicode 截断错误** — `keymgr.go`
   - 现状：`len(s)` 按字节计数，中文名会在字符中间截断导致乱码
   - 修复：改用 `[]rune` 计算长度和截断

2. **`saveForm()` 静默吞错误** — `keymgr.go`
   - 现状：`store.Add()` 返回的 error 被忽略，用户无反馈
   - 修复：在 `KeyManagerModel` 增加 `errMsg string` 字段，保存失败时设置并在 View 中展示

3. **`activateSelected()` 静默吞错误** — `keymgr.go`
   - 同上，`store.Activate()` 错误被忽略
   - 修复：同样通过 `errMsg` 展示

### 8.2 性能 / 代码质量

1. **`buildKeyRows()` 每次按键都重建** — `keymgr.go`
   - 现状：`updateList()` 入口无条件调用 `buildKeyRows()`
   - 修复：仅在数据变更后重建（add/delete/activate/init），移除 `updateList` 开头的调用

2. **`hasActiveKey()` 不必要的解密** — `dashboard.go`
   - 现状：调用 `GetActive()` 会解密 API Key，仅为判断是否存在
   - 修复：在 Store 新增 `HasActive(provider) bool` 方法，遍历检查 `Active` 字段即可

3. **`splitTags()` 逐字符拼接** — `keymgr.go`
   - 修复：改用 `strings.Split(s, ",")` + `strings.TrimSpace`

4. **`tui.go` 双重创建 model/callbacks** — `cmd/agx/tui.go`
   - 现状：callbacks 先创建空壳，再覆盖，model 创建两次
   - 修复：先构建完整 callbacks，再创建一次 model

### 8.3 UX 细节

1. **表单保存失败无提示**
   - 与 #2 合并，在表单底部或 status bar 展示错误信息（红色文字，下次按键清除）

2. **列表宽度硬编码** — `keymgr.go`
   - 现状：`%-20s` 固定 20 字符，窄终端溢出，宽终端浪费
   - 修复：根据 `m.width` 动态计算列宽

3. **清理未使用的 import hack** — `dashboard.go`
    - 现状：`_ = table.New` / `_ = key.NewBinding` 占位
    - 修复：删除这两行及对应的 `bubbles/table` 和 `bubbles/key` import

### 8.4 修改文件清单

| 文件 | 改动项 |
| ------ | -------- |
| `internal/tui/keymgr.go` | #1 #2 #3 #4 #6 #8 #9 |
| `internal/tui/dashboard.go` | #5 #10 |
| `internal/key/store.go` | #5（新增 `HasActive`） |
| `cmd/agx/tui.go` | #7 |
