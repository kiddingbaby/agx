# Contributing

感谢愿意为 agx 贡献代码。本文覆盖开发环境、构建、验证与提交流程。用户使用说明见 [`docs/user-guide.md`](docs/user-guide.md)。

## Prerequisites

- Go 1.24+（建议通过 `mise` 安装，已在 `mise.toml` 中声明）
- [Task](https://taskfile.dev/) 作为统一入口
- 可选：`golangci-lint`、`docker`、`bats`（`task dev:setup` 会处理）

## Setup

```bash
task dev:setup
task build
```

`task build` 输出到 `~/.cache/agx/bin/agx`，`task install` 覆盖 `~/.local/bin/agx`，可用 `BINDIR=...` 覆盖安装目录。

## Common tasks

```bash
task help              # 列出全部任务
task build             # 构建
task install           # 安装到 BINDIR
task verify            # 默认本地回归（必跑）
task verify:agents     # 本机真实 agent CLI 兼容性（缺 CLI 自动跳过）
task verify:docker     # 容器矩阵
task bench             # CLI 冷启动耗时预算
task docs:man          # 重新生成 man pages
task release:check     # GoReleaser 配置校验
task clean
```

## Verification matrix

| 改动范围 | 至少需要跑 |
| --- | --- |
| CLI 流程或原生配置同步 | `task verify` |
| 真实 agent CLI 兼容性 | `task verify:agents` |
| CLI 帮助 / 子命令树调整 | `task verify` + `task docs:man` |
| 发布前 | `task verify` + `task release:check` |

CI 默认运行 `task verify`、`task release:check`，以及 `docker buildx bake docker-matrix --progress=plain`。

## Public contract

公开合同的范围（哪些稳定、哪些是实现细节）以 [COMPATIBILITY.md](COMPATIBILITY.md)
为准；具体 CLI / JSON shape 见 [`docs/cli-contract.md`](docs/cli-contract.md)，
doctor issue 代码见 [`docs/doctor-issues.md`](docs/doctor-issues.md)。

## Pull request checklist

- 一条 PR 只解决一个明确问题
- PR 描述列出实际跑过的命令
- 改 CLI 行为：跑 `task verify`
- 用户可见行为变化：同步更新文档

## Code of Conduct

参与本项目即代表你同意 [Code of Conduct](CODE_OF_CONDUCT.md)。
