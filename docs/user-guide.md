# 使用说明

本文件只覆盖日常使用命令和运行时行为。
项目边界、安装方式和路径入口见 `../README.md`。

## 新增与编辑

```bash
agx add relay-a --base-url https://relay.example/v1 --api-key sk-xxx
agx edit relay-a --api-key sk-rotated
agx edit relay-a --base-url https://relay.example/v1
agx edit relay-a --bind codex,claude
agx edit relay-a --unbind codex
```

- 在终端中直接运行 `agx add` 时，缺失字段会继续提问
- 在终端中直接运行 `agx edit relay-a` 时，会先显示当前 relay，再询问要修改的字段
- `add` 默认只创建 relay，不会自动修改任何 agent 配置
- `edit` 会自动同步当前绑定到该 relay 的 agent
- `--bind` / `--unbind` 接受逗号分隔 agent 列表，可重复出现；实现会合并、归一化、去重
- 允许多个 relay 复用同一个 `base_url`

## 查看 relay

```bash
agx ls
agx ls --agent codex
agx show relay-a
```

- `ls` 默认列出全部 relay，并显示 `agents=-|codex,claude`
- `ls --agent codex` 切到 agent 视角，列出全部 relay，并标出当前绑定项
- `show` 显示 relay 详情、当前绑定 agent 和最近一次同步信息

## 绑定到 agent

```bash
agx edit relay-a --bind codex
agx edit relay-a --bind claude,gemini
agx edit relay-a --unbind codex
```

- `--bind` 只影响目标 agent 的当前绑定
- 同一个 relay 可以同时被多个 agent 使用
- `--bind` / `--unbind` 会先建立目标 agent 的备份
- `agx edit relay-b --bind codex` 会把 `codex` 从旧 relay 自动切到 `relay-b`
- `agx edit relay-a --unbind codex` 只在 `codex` 当前绑定该 relay 时成功，否则直接报错

## 备份与恢复

```bash
agx backup ls --agent codex
agx restore --agent codex
agx restore --agent codex --to BACKUP_ID
```

- 每个 agent 只保留最近 5 条同步历史
- `restore` 只恢复目标 agent 的配置
- `restore` 不会切换已运行中的 agent 会话

## 检查与删除

```bash
agx doctor
agx rm relay-a
```

- `doctor` 检查未完成操作、缺失的 relay 绑定和缺失的备份文件
- 仍被 agent 绑定的 relay 不能删除

## JSON 输出

这些命令支持 `-o json`：

- `add`
- `edit`
- `ls`
- `show`
- `restore`
- `backup ls`
- `doctor`
- `rm`

## 运行时文件

- relays: `~/.config/agx/profiles/`
- state: `~/.config/agx/state.yaml`
- state lock: `~/.config/agx/state.yaml.lock`
- lock: `~/.config/agx/agx.lock`
- operation journal: `~/.config/agx/ops/current.yaml`
- backups: `~/.config/agx/backups/<agent>/`

当前版本里，agent 当前绑定记录在 `state.yaml` 的 `codex`、`claude`、`gemini` 节点里；Codex 已注册 relay 列表记录在 `codex-profiles`。这些仍是首轮实现细节，不属于公开 CLI 合同。

## 约束

- `relay name` 不能为空，且不能包含空白字符
- `base_url` 必须是 `http://` 或 `https://`
- 同一类 agent 的多个窗口共享同一份用户级配置
