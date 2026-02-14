# AGX TODO

Generated from: SPEC.md
Branch: feat/key-env-v1
Created: 2026-02-13

## Tasks

Serial execution order (single-repo): 2 -> 3 -> 5 -> 6

### [1] in_progress - Key Store 支持 Update（含 UpdatedAt 与兼容迁移）

- File: internal/key/store.go
- Description: 为 Key 增加 UpdatedAt；实现 Update patch 语义（APIKey 空=不改，其他字段按 set 覆盖）；旧 YAML 缺失 updated_at 时兼容。
- Dependencies: []
- Steps:
  - [x] coding    @ 2026-02-14 15:09:18 (919f437)
  - [x] tidy    @ 2026-02-14 15:10:55 (919f437)
  - [x] lint    @ 2026-02-14 15:11:31 (919f437)
  - [x] review    @ 2026-02-14 15:12:51 (919f437)
  - [x] test    @ 2026-02-14 15:13:15 (919f437)
  - [x] scan    @ 2026-02-14 15:14:13 (919f437)
  - [ ] pr

---

### [2] in_progress - TUI KeyMgr 支持 Edit（e）与表单回填

- File: internal/tui/keymgr.go
- Description: 在 List 增加 `e` 进入 Edit；Form 支持 Add/Edit mode；Edit 时回填 Name/BaseURL/Tags，APIKey 置空表示保持不变；列表时间字段优先显示 UpdatedAt。
- Dependencies: [1]
- Steps:
  - [x] coding    @ 2026-02-14 15:23:34 (919f437)
  - [x] tidy    @ 2026-02-14 15:28:31 (919f437)
  - [x] lint    @ 2026-02-14 16:01:53 (512c8f7)
  - [x] review    @ 2026-02-14 16:02:34 (512c8f7)
  - [x] test    @ 2026-02-14 16:03:15 (512c8f7)
  - [x] scan    @ 2026-02-14 16:03:39 (512c8f7)
  - [ ] pr

---

### [3] in_progress - TUI KeyMgr 增加搜索过滤（/）与 gg/G 导航

- File: internal/tui/keymgr.go
- Description: 增加过滤输入（按 name/tags/provider 过滤）；支持 `gg` 到首行、`G` 到末行；保持方向键与 Vim 双轨。
- Dependencies: [2]
- Steps:
  - [x] coding    @ 2026-02-14 16:09:48 (512c8f7)
  - [x] tidy    @ 2026-02-14 16:09:48 (512c8f7)
  - [x] lint    @ 2026-02-14 16:09:49 (512c8f7)
  - [x] review    @ 2026-02-14 16:10:06 (512c8f7)
  - [x] test    @ 2026-02-14 16:10:07 (512c8f7)
  - [x] scan    @ 2026-02-14 16:10:07 (512c8f7)
  - [ ] pr

---

### [4] in_progress - Session Orchestrator 改为安全 env 注入（不拼接 export）

- File: internal/session/orchestrator.go
- Description: 移除 `export KEY=... && cmd` 拼接；改为 tmux session env + respawn-pane 启动；
  保证 pane 内命令不包含明文 secret；并限制 env 作用域到 `ai-<agent>` session。
- Dependencies: []
- Steps:
  - [x] coding    @ 2026-02-14 16:15:28 (512c8f7)
  - [x] tidy    @ 2026-02-14 16:15:28 (512c8f7)
  - [x] lint    @ 2026-02-14 16:15:29 (512c8f7)
  - [x] review    @ 2026-02-14 16:15:56 (512c8f7)
  - [x] test    @ 2026-02-14 16:15:56 (512c8f7)
  - [x] scan    @ 2026-02-14 16:15:56 (512c8f7)
  - [ ] pr

---

### [5] in_progress - 启动路径对齐（CLI/TUI）并补齐回归测试

- File: cmd/agx/main.go
- Description: 确保 CLI 与 TUI Launch 都走同一套安全注入逻辑；必要时调整 orchestrator 测试（escapeForShell 若降级为非关键）。
- Dependencies: [4]
- Steps:
  - [x] coding    @ 2026-02-14 16:16:44 (512c8f7)
  - [x] tidy    @ 2026-02-14 16:16:44 (512c8f7)
  - [x] lint    @ 2026-02-14 16:16:45 (512c8f7)
  - [x] review    @ 2026-02-14 16:17:16 (512c8f7)
  - [x] test    @ 2026-02-14 16:17:16 (512c8f7)
  - [x] scan    @ 2026-02-14 16:17:16 (512c8f7)
  - [ ] pr

---

### [7] in_progress - Bug 修复：truncate/saveForm/activateSelected

- File: internal/tui/keymgr.go, internal/key/store.go
- Description: truncate() 改用 []rune 修复 Unicode 截断；saveForm() 和 activateSelected() 捕获 error 并通过 errMsg 展示给用户（红色文字，下次按键清除）。
- Dependencies: []
- Steps:
  - [x] coding
  - [x] tidy
  - [x] lint    @ 2026-02-14 14:17:37 (919f437)
  - [x] review    @ 2026-02-14 14:25:46 (919f437)
  - [ ] test
  - [ ] scan
  - [ ] pr

---

### [8] in_progress - 性能优化：buildKeyRows/HasActive/splitTags/tui 双重创建

- File: internal/tui/keymgr.go, internal/tui/dashboard.go, internal/key/store.go, cmd/agx/tui.go
- Description: buildKeyRows() 仅在数据变更时重建；Store 新增 HasActive() 避免不必要解密；splitTags() 改用 strings.Split；tui.go 消除双重 model 创建。
- Dependencies: []
- Steps:
  - [x] coding
  - [x] tidy
  - [ ] lint
  - [ ] review
  - [ ] test
  - [ ] scan
  - [ ] pr

---

### [9] in_progress - UX：动态列宽 + 清理 import hack

- File: internal/tui/keymgr.go, internal/tui/dashboard.go
- Description: 列表宽度根据 m.width 动态计算；删除 dashboard.go 中 _= table.New /_ = key.NewBinding 及对应 import。
- Dependencies: []
- Steps:
  - [x] coding
  - [x] tidy
  - [ ] lint
  - [ ] review
  - [ ] test
  - [ ] scan
  - [ ] pr

---

### [6] in_progress - 文档更新：默认不要求设置 AGX_SECRET

- File: README.md
- Description: 更新 quickstart，强调 secret 默认自动生成到 `~/.config/agx/secret`；AGX_SECRET 仅作为可选覆盖；说明 env 注入由 AGX 托管无需改 shell rc。
- Dependencies: []
- Steps:
  - [x] coding    @ 2026-02-14 16:17:45 (512c8f7)
  - [x] tidy    @ 2026-02-14 16:17:45 (512c8f7)
  - [x] lint    @ 2026-02-14 16:17:46 (512c8f7)
  - [x] review    @ 2026-02-14 16:18:23 (512c8f7)
  - [x] test    @ 2026-02-14 16:18:23 (512c8f7)
  - [x] scan    @ 2026-02-14 16:18:23 (512c8f7)
  - [ ] pr
