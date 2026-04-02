# configfile

`configfile/` 负责 provider/site 配置文件的持久化与读取。
当前实现围绕 `providers.yaml` 的 registry 读写展开。

约束：

- 只做配置落盘与解析
- provider 业务规则仍由 `../../domain/provider/` 与 `../../usecase/` 决定
