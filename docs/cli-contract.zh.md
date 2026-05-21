# CLI 契约

本文档定义 `agx` CLI 的**公开契约**：
- exit code
- JSON 输出 schema
- stdout / stderr 约定

这些签名属于 agx 的稳定承诺（见 [COMPATIBILITY.md](../COMPATIBILITY.md)）。
内部 state 文件布局（`~/.config/agx/...`）不在覆盖范围内。

English: [cli-contract.md](cli-contract.md).

---

## Exit code

| Code | 含义 |
| --- | --- |
| `0` | 成功 |
| `1` | agx 自身错误（校验、IO、profile 不存在、mutation 拒绝） |
| `<原生 CLI 退出码>` | 启动器（`agx run <agent>`、`agx <agent>`）原样转发底层 CLI 的退出码 |

`--version` / `--help` 在帮助场景退出 `0`；解析失败强制 help 时退出 `1`。

## 流

- **stdout** — 主输出（表格 / JSON），可以 pipe
- **stderr** — 错误、警告、建议、deprecation 通知。始终带 `Error:` 或 `warning:` 前缀
- **stdin** — 只有 `agx __api-key`（内部）会读；用户命令绝不在 stdin 上 block

`-o json` 时 stdout 一定是**单行 JSON 加 `\n`**。诊断信息仍走 stderr。

## 输出格式

所有 mutation / list 命令支持 `-o json`。不带 `-o` 出表格或纯文本（面向人）。

```bash
agx ls -o json | jq '.profiles[] | select(.current).name'
agx show work -o json | jq -r .profile.base_url
agx doctor -o json | jq '.issues[] | select(.severity == "error")'
```

`-o` 保留给未来的格式（`yaml` 可能加）。当前传 json 以外的值会在 stderr 上
打印 `Error: -o requires value json` 并退出 `1`。

---

## 命令 schema

### `agx add <name> --base-url URL --api-key KEY [--model MODEL]`

```json
{
  "profile": {
    "name": "work",
    "kind": "relay",
    "current": false,
    "base_url": "https://relay.example/v1",
    "api_key": "sk-...",
    "credential_ref": "api_key",
    "model": "",
    "provider_family": ""
  }
}
```

### `agx edit <name> [flags]`

```json
{
  "profile":         { /* managedProfileView，结构同 add */ },
  "resynced_targets": [
    { "agent": "codex", "target": "work", "config_path": "..." }
  ],
  "failed_targets":   [
    { "agent": "opencode", "target": "work", "error": "..." }
  ]
}
```

两个数组为空时省略。

### `agx rm <name>`

```json
{ "profile": { /* managedProfileView，被删除的 profile */ } }
```

### `agx ls [--all]`

```json
{
  "current": "work",
  "profiles": [
    { /* managedProfileView */ }
  ]
}
```

`current` 在无 profile 选中时省略。

### `agx show <name>`

```json
{ "profile": { /* managedProfileView */ } }
```

### `agx use <name>`

```json
{ "profile": { /* managedProfileView */ } }
```

### `agx current`

```json
{ "profile": { /* managedProfileView */ } | null }
```

`profile` 在无当前 profile 时为 `null`。

### `agx detach <agent> <profile>`

```json
{ "agent": "codex", "profile": "work" }
```

### `agx backup <agent>`

```json
{
  "agent":  "codex",
  "backup": {
    "id": "20260520T101011Z",
    "target_kind": "relay",
    "target_name": "work",
    "path": "/home/.../backups/managed/codex/relay/work/20260520T101011Z",
    "created_at": "2026-05-20T10:10:11Z"
  }
}
```

### `agx restore <agent>`

结构同 `agx backup`。恢复的 snapshot 返回在 `backup` 字段。

### `agx doctor`

```json
{
  "ok": false,
  "operation": {
    "id": "set-codex-20260520T1010...",
    "command": "set",
    "agent": "codex",
    "profile": "work",
    "stage": "started"
  },
  "issues": [
    {
      "severity": "error",
      "code": "unfinished_operation",
      "message": "...",
      "action": "agx restore codex"
    }
  ]
}
```

`operation` 在没有崩溃残留时省略。健康时 `issues` 是 `[]`。

### `agx version`

只输出纯文本，schema：

```
agx <version>
commit=<git sha>
date=<RFC3339 编译时间>
go=<go runtime>
os=<linux|darwin>
arch=<amd64|arm64>
```

### `agx completion {bash|zsh|fish|powershell}`

只输出纯文本——Cobra 生成的 shell completion 脚本。

### `agx run <agent> [profile] [-- <native args...>]` / `agx <agent> [...]`

把 stdin / stdout / stderr 直接转给原生 CLI。agx 自身在这种模式下不输出
任何 stdout。退出码 = 原生 CLI 退出码。

Profile 解析优先级（高 → 低）：

1. 位置参数 `profile`
2. `AGX_PROFILE` 环境变量（纯空白视为未设置）
3. `agx current` 选中的 profile

设置 `AGX_AUTO_BACKUP=1` 时，agx 在 exec 原生 CLI 前对当前 target 的
context 做一次 snapshot。失败以 stderr 上 `warning: ...` 提示但不阻塞启动。

---

## 稳定形状（兼容承诺）

下列 key **稳定**，不会无 deprecation 周期就被删或改名：

- `managedProfileView`：`name`、`kind`、`current`、`base_url`、`api_key`、
  `credential_ref`、`model`、`provider_family`、`agents`
- `contextBackupView`：`id`、`target_kind`、`target_name`、`path`、
  `created_at`
- `DoctorReport`：`ok`、`operation`、`issues`
- `DoctorIssue`：`severity`、`code`、`message`、`action`
- `OperationRecord`：`id`、`command`、`agent`、`profile`、`stage`、
  `config_path`、`backup_path`、`backup_id`、`started_at`、`updated_at`

新 key 可以**新增**（消费方须忽略未知 key）。删 / 改名需要大版本号。

`doctor` 的 issue code 集合是 [doctor-issues.zh.md](doctor-issues.zh.md)
里的封闭目录。测试锁住该目录与运行时 emitter 双向一致。消费方应根据
`code` 字段匹配，不要匹配 `message` 文本。
