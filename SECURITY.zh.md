# 安全策略

English: [SECURITY.md](SECURITY.md).

## 报告漏洞

如果你认为发现了 agx 的安全问题，**请不要直接在 GitHub 上开公开 issue**，
按以下任一私有方式联系：

- 邮件：**kiddingbaby@163.com**，主题：`agx security: <简短描述>`
- 或者使用 GitHub 的 [Private Vulnerability Reporting](https://github.com/kiddingbaby/agx/security/advisories/new)

我们目标在 **72 小时**内确认收到，对确认的高危问题在 **14 天**内提供
修复或缓解措施。

请在报告里附上：

- agx 版本（`agx version` 输出）
- 操作系统 / 架构
- 最小复现步骤
- 实际观察到的影响（攻击者能做什么）

## 范围

agx 是**单用户本地 CLI**：

- 读写 `~/.config/agx/` 下的配置以及各 agent 配置位置（`~/.codex`、
  `~/.claude` 等）
- 用户提供的 API key 以明文 YAML 存在 `~/.config/agx/profiles/`
- 启动原生 agent CLI（`codex`、`claude` 等）时把 API key 作为环境变量传入

我们视为**安全 bug 的问题**：

- 提权（agx 以用户 X 身份运行，却获得了在自身目录树之外读写的能力）
- 用户 API key 泄漏给本机其他用户、log、世界可读路径
- 路径穿越 / 任意文件写——通过 profile / target 名或 flag 值触发
- 通过精心构造的 config / state / journal 文件触发代码执行
- mutation guard / rollback 被绕过，让 agent config 留在已知坏状态

我们一般**不**视为安全问题：

- 攻击者已对 `~/.config/agx/` 有写权限就能破坏 agx——这是设计选择，
  agx 信任自己的 state 目录
- 同机同 UID 的攻击者能读你的 key。Unix 权限即权限边界；agx 用
  `0600` / `0700` mode
- 用户主动删除 AGX 目录或主动跑破坏性命令造成的数据丢失

## Disclosure

我们走 coordinated disclosure：修复就绪后，发 GitHub Security Advisory
带 CVSS 分数和 reporter 致谢（reporter 选择匿名则不致谢）。

## 支持的版本

只有**最新 minor release** 收到安全修复。pre-1.0 不是 LTS——请跟踪
`main` 或 pin 到最新 tag。
