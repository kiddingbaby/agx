# Key Manager 操作手册

## 1. 入口

```bash
agx keys
```

## 2. 列表页（Provider Tabs + Key 列表）

- 顶部为 Provider tabs：`CLAUDE` / `OPENAI` / `GEMINI`
- 中间仅显示当前 Provider 的 keys
- `*` 表示 active key

### 2.1 快捷键

```text
h/l 或 ←/→ 或 1/2/3   切换 Provider
j/k 或 ↑/↓             上下选择 key
Enter                   激活当前 key
a                       新增 key
e                       编辑当前 key
d                       删除当前 key
i                       显示/隐藏当前 key 详情
/                       进入过滤（name/tag/provider）
gg / G                  跳到首行/末行
Esc                     返回 Dashboard
q / Ctrl+C              退出程序
```

### 2.2 详情面板（按 `i` 切换）

展示字段：

- `name`
- `id`
- `provider`
- `profile`
- `base URL`
- `tags`
- `active`
- `updated`

安全约束：

- API key 不显示明文，仅显示 `API key is hidden for security`
- Codex base URL 仅注入 `OPENAI_BASE_URL`（不再双写 `OPENAI_API_BASE`）
- Codex API key 环境变量名优先读取 `~/.codex/config.toml` 的 `model_providers.<current>.env_key`（缺失时回退 `OPENAI_API_KEY`）

## 3. 编辑页（新增 / 编辑共用）

页面标题会显示模式：`[NORMAL]` 或 `[INSERT]`

### 3.1 NORMAL 模式（默认）

```text
j/k 或 ↑/↓    切换字段焦点
h/l 或 ←/→    当焦点在 Provider 时切换 provider
i             进入 INSERT
x             清空当前字段（常用于快速清空 API Key）
Enter         保存
Esc           返回列表
```

### 3.2 INSERT 模式

```text
输入字符        编辑当前字段
Ctrl+u         清空当前字段
Enter          保存
Esc            回到 NORMAL（不离开表单）
```

## 4. 删除确认页

```text
h/l/←/→        切换 [Cancel]/[Delete]
Enter          确认
Esc            取消并返回列表
```
