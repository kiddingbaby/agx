# keyfile

`keyfile/` 负责 key 存储与 AES-GCM 加解密。
它承载 `keys.yaml` 的读写与密文处理，是 AGX 的关键安全 adapter。

约束：

- 明文 key 只在运行时内存存在
- 持久化与加解密细节留在这里，业务规则继续放在 `../../domain/key/` 与 `../../usecase/`
