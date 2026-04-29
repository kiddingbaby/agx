# 贡献说明

本文件只覆盖开发、测试和提交流程。
使用说明见 `docs/user-guide.md`。

## 开发环境

```bash
task setup
task build
```

`task build` 只更新缓存二进制，默认输出到 `~/.cache/agx/bin/agx`；`task install` 才会覆盖 `~/.local/bin/agx`。  
所以在 PR 前即使已经 `build` 或跑过测试，`command -v agx` 命中的也仍可能是旧的已安装版本。

## 目录

- `cmd/agx/`: CLI 入口
- `internal/interfaces/cli/`: 参数解析、交互、输出视图
- `internal/usecase/`: 业务编排
- `internal/domain/profile/`: profile 模型与规则
- `internal/adapters/`: 配置同步、状态持久化、锁、操作记录
- `tests/`: 合同测试、集成测试、端到端冒烟测试

## 常用命令

```bash
task test
task contract
task contract-json
task cue
task property
task check
task real-smoke
task docker-check
task docker-matrix
task release-check
task release-snapshot
task check-all
```

## 测试

- 领域规则与不变量：`task property`
- CLI 文本输出、退出码、文件系统副作用：`task contract`
- `-o json` 输出：`task contract-json`
- JSON 合同复核单独重跑：`task cue`
- 默认本地回归：`task check`
  共享一套 portable 验证，覆盖 `go test ./...`、contract、interactive PTY、CUE、shell/bats smoke
- 容器内受控环境回归：`task docker-check`
- 跨基础镜像容器回归：`task docker-matrix`
- 本机真实 agent CLI：`task real-smoke`
  当本机未安装 `codex`、`claude`、`gemini` 时会自动跳过
  这是 best-effort 兼容性检查；若失败，先区分 AGX 配置同步错误 与 上游 CLI 行为变化
- GoReleaser 配置校验：`task release-check`
- 本地发布归档预演：`task release-snapshot`
- 完整本机回归：`task check-all`
  覆盖 `task check`、`task release-check`、`task real-smoke`；不包含 Docker matrix

如果改 CLI 流程或原生配置同步，至少跑 `task check`。

## 提交 PR

- 一条 PR 只处理一个清晰问题
- 在 PR 描述里写清楚实际跑过的命令
- CLI 变化至少覆盖 `contract` 或 `check`
- JSON 变化至少覆盖 `contract-json` 或 `cue`
- 用户可见行为变化时同步更新文档
- 默认 CI 会运行宿主机 portable 验证（`task check`）、`task release-check`，以及独立的 Docker matrix 验证（`docker buildx bake docker-matrix`）
