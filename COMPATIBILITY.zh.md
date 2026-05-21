# 兼容性策略

本文档定义 `agx` 公开契约的边界。配合 [docs/cli-contract.zh.md](docs/cli-contract.zh.md)
（具体形状）与 [semver](https://semver.org/lang/zh-CN/) 决定版本号 bump。

English: [COMPATIBILITY.md](COMPATIBILITY.md).

## 属于契约的部分

下列为**稳定**承诺。破坏性变更需要大版本号 bump（或对增量替换走 deprecation 周期）。

- CLI 子命令、它们的参数形状、flag 名
- exit code（见 [docs/cli-contract.zh.md](docs/cli-contract.zh.md)）
- 已记录命令的 JSON 输出形状，包括 key 名与枚举字符串值
- `agx doctor` 已显式记录的 issue `code` 值
- 以 `AGX_*` 开头的环境变量
- 标准安装路径（默认 `~/.local/bin/agx`，可用 `BINDIR` 覆盖）
- `agx run <agent>` 支持的 agent 集合：`codex`、`claude`、`gemini`、`opencode`

## **不**属于契约的部分

以下视为实现细节，任何版本（包括 patch）都可以不打招呼调整：

- `~/.config/agx/` 的磁盘布局（state、profile、context、backup、journal、lockfile）
- `state.yaml`、profile YAML、journal record 内部的字段名和 YAML key 形状
- agx 写出的 agent 专属文件格式（`~/.codex/config.toml` 的 "AGX managed"
  块、`settings.json` 形状、`.env` key 等）——这些跟随上游 agent CLI，
  上游变就跟着变
- help 文本措辞（结构 / 章节稳定，句子表述不稳定）
- 错误消息原文——脚本请匹配 `doctor` 的 issue `code`
- `internal/` 下的 Go 包——不支持当作库引入

## 版本

agx 从 v0.x 起使用 [Semantic Versioning](https://semver.org/lang/zh-CN/)：

- `v0.x.y` —— 在 minor 之间，**除契约部分外**任何东西都可能 break。
  到 1.0 后契约部分进一步收紧。
- patch release 只修 bug，不破坏契约。
- 标记 pre-release（`-rc.N`、`-alpha.N`）用于风险变更；它们之间可以与
  上一个 pre-release 不兼容。

## Deprecation

任何非纯增量的契约变更走以下流程：

1. 在 release notes 文档化新形状，并在 help 文本 + 本文件标记旧形状
   `deprecated`
2. 至少保留一个 minor release 让旧形状继续可用
3. 大版本号 bump 时移除（pre-1.0 churn 可以 minor bump 移除，但 release
   notes 要显式提到）

如果你在脚本里依赖 agx，请 pin 到 tag（`@v0.x.y`），升级前读 release notes。
