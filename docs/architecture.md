# AGX 架构（As-Is，2026-03-12）

> 本文以当前仓库代码为准。

## 1. 分层结构

```text
cmd/agx
  -> internal/interfaces/cli
    -> internal/app/bootstrap
      -> internal/usecase
        -> internal/domain + internal/ports
          -> internal/adapters/{keyfile,configfile,toolconfig}
```

## 2. 关键职责

| 层 | 目录 | 职责 |
| --- | --- | --- |
| cmd | `cmd/agx` | 进程入口与退出码 |
| interfaces | `internal/interfaces/cli` | 参数解析、错误展示 |
| app | `internal/app` | 依赖装配（paths/secret/repo/services） |
| usecase | `internal/usecase` | 业务编排与统一错误模型 |
| domain | `internal/domain` | 核心模型与规则（agent/key/provider） |
| ports | `internal/ports` | 外部能力抽象（repo/provider/toolconfig） |
| adapters | `internal/adapters` | IO/系统实现（YAML+AES-GCM + providers.yaml + 各 CLI 原生配置写入） |

## 3. 关键调用链

### 3.1 CLI 启动

1. `cmd/agx/main.go` 调 `app.Bootstrap()`
2. 生成 `interfaces/cli.Root`
3. `Root.Execute(args)` 分发到 site/add/k/apply/use/status
4. 通过 usecase 调用 ports，最终落到 adapters

### 3.3 Switch 链路（按 name 切换）

1. `SwitchService.SwitchByName(name)`
2. 从 `ProviderService.GetTarget/UseTarget` 解析并更新 family binding
3. 从 `KeyService.GetActive` 获取解密 key（按 `provider/profile`）
4. `toolconfig.Syncer.Apply` 写入各 CLI 原生配置文件

## 4. 错误模型（统一）

`internal/usecase/errors.go` 统一定义并复用：

- `NoActiveKeyError`
- `KeyNotFoundError`

接口层统一通过 `errors.As/Is` 映射文案，避免散落分支。

## 5. 安全边界

- API Key 明文仅存在于 usecase 运行时内存
- 持久化使用 AES-GCM（adapter/keyfile）
- 同步时会把 key 写入各 CLI 原生配置文件（与用户手动配置同等风险边界）
- secret 生命周期统一由 `config.SecretProvider` 管理

## 6. 兼容策略

本仓库当前不保留历史兼容层，直接使用新分层路径：

- key 存储实现：`internal/adapters/keyfile`
