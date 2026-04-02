# toolconfig

`toolconfig/` 负责把 AGX 当前选择同步到各 CLI 的原生配置文件。
它是 `agx use`、`agx sync` 等命令落地到 Codex/Claude/Gemini 配置的 adapter 层。

约束：

- 只做配置同步，不决定 provider/key 选择规则
- 各 CLI 的目标文件路径与写入协议应继续通过 ports/usecase 调度
