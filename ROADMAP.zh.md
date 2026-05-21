# Roadmap

agx 的方向滚动快照。**只是方向，不是承诺**——可能在版本之间调整。

English: [ROADMAP.md](ROADMAP.md).

## Now（v0.1.0 基线）

v0.1.0 是单用户中转聚合器的初始基线。下一轮默认**等 issue**——
具体 feature 会随真实请求被晋升。下方 Later 仍然开放。

## Later

- **Nix flake** —— 给 Nix 用户
- **Scoop manifest** —— Windows 用户通过 WSL2 + Scoop 安装
- **审计日志** —— opt-in 追加式 mutation log，方便事后查

## Out of scope

下面是有意识的"no"。不同意可以开 issue 讨论，但默认答案是 no。

- **Native Windows 支持** —— WSL2 是支持路径；native Windows 意味要为
  agx 受众里一位数比例的人重写 flock 和 path 层
- **多用户 / 团队 profile 共享** —— agx 有意做成单用户工具；团队级
  secret 管理属于真正的 secret store（Vault、AWS SM、doppler 等）
- **Agent 编排** —— agx 一次启动一个 agent；链 / pipeline / 并行编排
  不在范围
- **特定 provider 的 OAuth 流程** —— OAuth 请直接用原生 CLI；agx 是
  中转（`base_url` + `api_key`）聚合器

## 怎么影响 roadmap

- 想法开 [Discussion](https://github.com/kiddingbaby/agx/discussions)
- bug 或 feature 请求开 issue，写具体用例
- 写 PR 直接做 **Now** 或 **Later** 的事，先关联 issue / discussion
  让我们确认匹配
