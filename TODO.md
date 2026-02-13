# AGX 任务列表

Generated from: SPEC.md
Branch: feat/ux-fix-baseurl
Created: 2026-02-13

## Tasks

### [1] pending - Key Store 添加 BaseURL 字段

- File: internal/key/store.go
- Description: Key struct 新增 BaseURL，Add() 签名新增 baseURL 参数
- Dependencies: []
- Steps:
  - [ ] coding
  - [ ] tidy
  - [ ] lint
  - [ ] review
  - [ ] test
  - [ ] scan
  - [ ] pr

---

### [2] pending - Agent 定义添加 BaseURLEnvVar

- File: internal/tui/agents.go
- Description: Agent struct 新增 BaseURLEnvVar 字段
- Dependencies: []
- Steps:
  - [ ] coding
  - [ ] tidy
  - [ ] lint
  - [ ] review
  - [ ] test
  - [ ] scan
  - [ ] pr

---

### [3] pending - Dashboard 空 session 引导

- File: internal/tui/dashboard.go
- Description: 无 session 时显示引导文案，提示按 K 管理 keys
- Dependencies: []
- Steps:
  - [ ] coding
  - [ ] tidy
  - [ ] lint
  - [ ] review
  - [ ] test
  - [ ] scan
  - [ ] pr

---

### [4] pending - Key Manager 列表导航重构

- File: internal/tui/keymgr.go
- Description: buildKeyRows 加入 provider header 行，支持 provider 级 j/k 导航
- Dependencies: [1]
- Steps:
  - [ ] coding
  - [ ] tidy
  - [ ] lint
  - [ ] review
  - [ ] test
  - [ ] scan
  - [ ] pr

---

### [5] pending - Key Manager Form 交互修复与 BaseURL 扩展

- File: internal/tui/keymgr.go
- Description: 修复 form 交互，添加 BaseURL 字段，预选 provider
- Dependencies: [1, 4]
- Steps:
  - [ ] coding
  - [ ] tidy
  - [ ] lint
  - [ ] review
  - [ ] test
  - [ ] scan
  - [ ] pr

---

### [6] pending - Launch 注入 BaseURL 环境变量

- File: cmd/agx/tui.go
- Description: launch 时若 BaseURL 非空则注入对应环境变量，CLI keys add 新增 --base-url
- Dependencies: [1, 2]
- Steps:
  - [ ] coding
  - [ ] tidy
  - [ ] lint
  - [ ] review
  - [ ] test
  - [ ] scan
  - [ ] pr

## Status

- Total: 6
- Pending: 6
- In Progress: 0
- Done: 0
