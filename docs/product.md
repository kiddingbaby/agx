# AGX（产品视角）

## 解决什么问题

把多 provider 的 **“keys + endpoint(target) + 原生 CLI 配置同步”** 统一成一个 CLI 入口：

- key 管理：集中存储、加密、激活与删除
- provider 配置：把第三方/官方 target 显式化，并同步到各 CLI 原生配置

## 最短用户旅程

1. `agx create site <name>` 用模板建站（官方站点 `openai|claude|gemini` 内建）
2. `agx create key --site <site> --stdin` 批量导入 key（粘贴即可）
3. `agx use <site>` 一条命令切换站点并同步原生配置
4. 直接运行 `codex` / `claude` / `gemini`

更完整的按场景路径见：`docs/user-guide.md`。
