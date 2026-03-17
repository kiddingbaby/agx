# AGX 重构路线图（执行结果）

> 截止 2026-02-15，SPEC/TODO 的 8 个任务已按 workflow 分波次执行。  
> 2026-03-12：已移除 TUI，当前为 CLI-only。

## 1. 执行波次

- Wave 1: Task 1（domain/key + ports 边界）
- Wave 2: Task 2-3（keyfile adapter + provider/config adapter）
- Wave 3: Task 4-5（usecase errors + app/bootstrap/config）
- Wave 4: Task 6-7（interfaces/cli）
- Wave 5: Task 8（文档收敛 + 回归基线）

## 2. 已完成里程碑

1. 分层结构落地（interfaces/usecase/domain/ports/adapters）
2. `main.go` 收敛为薄入口
3. secret/path 生命周期集中到 `config` + `app.Bootstrap`
4. usecase 错误模型统一，接口层映射收敛
5. 文档与目录结构与代码现状对齐

## 3. 固化回归基线

### 3.1 单测基线

```bash
go test ./...
```

### 3.2 CLI 冒烟基线

```bash
bash tests/integration/smoke-go.sh
```

脚本内部会执行：

- 构建 `cmd/agx`
- 临时 HOME 下 key add/activate/ls
- `agx status`
- `agx use <name>` 同步到各 CLI 原生配置文件的路径校验

## 4. 风险与后续优化项

1. `toolconfig` 写入各 CLI 原生配置，需持续关注上游配置格式变更

## 5. 建议门禁（PR/CI）

- 必跑：`go test ./...`
- 必跑：`bash tests/integration/smoke-go.sh`
- 变更触达 `internal/usecase` 时：校验 `errors.go` 统一错误模型不被旁路
