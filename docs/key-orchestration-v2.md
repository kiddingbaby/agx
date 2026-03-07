# AGX Key 设计（直观版）

> 这版只回答 3 个问题：
> 1) 用户操作后，系统到底怎么选 key？
> 2) Codex 为什么和 Claude/Gemini 不一样？
> 3) 多终端同时用会不会乱？

## 1. 先看结论（1 分钟）

你在 `agx` 里做的事：

1. 给某个 provider（openai/claude/gemini）加多个 key。
2. 选策略（fixed / round_robin / random）。
3. 启动 CLI（`agx codex-cli` / `agx claude-code` / `agx gemini-cli`）。

`agx` 在启动时只做一件核心事：

- 选出“这一次要用的 key”，然后把它放进对应环境变量里，让 CLI 去读。

CLI 本身不管 key 池，不管轮换；CLI 只吃“当前环境变量里的一个 key”。

## 2. 谁负责什么

- `agx` 负责：
  - 存 key 列表
  - 轮换策略
  - 选中本次 key
  - 注入环境变量
- CLI（Codex/Claude/Gemini）负责：
  - 从环境变量读 key
  - 发请求

## 3. 运行时到底怎么选 key

每次 launch 都按这条顺序：

1. 如果你传了 `--key`，就用它（最高优先级）。
2. 否则从 `agx` 的 key store（`keys.yaml`）按策略选一个。
3. 只有在 key store 没有可用 key 时，才回退到当前 shell 环境变量。

策略规则：

- `fixed`：一直用固定 key
- `round_robin`：按顺序轮
- `random`：随机选

## 4. Codex 的特殊点（重点）

Claude/Gemini：变量名基本固定。  
Codex：变量名可以在 `~/.codex/config.toml` 里配 `env_key`。

意思是：

- `agx` 先看 Codex 当前 provider 的 `env_key` 是什么名字。
- 然后把本次选中的 key 写进这个变量名。

如果没配 `env_key`，回退 `OPENAI_API_KEY`。

## 5. Base URL 规则（当前）

- Codex：只用 `OPENAI_BASE_URL`（单变量，不再双写）。
- Claude：`ANTHROPIC_BASE_URL`
- Gemini：`GEMINI_BASE_URL`

## 6. 多终端并发会不会冲突

不会乱的前提：

1. `keys.yaml` 的写入有文件锁。
2. 每次轮换和状态更新在同一次写操作里完成。

这样可以避免“两个终端同时拿到同一个轮换位”的问题。

## 7. 用户能感知到的行为

你会看到：

1. `agx keys activate xxx` 后，`keys.yaml` 的 `active/fixed_key` 会变。
2. 新开终端再 `agx keys ls`，active 状态一致。
3. 启动 CLI 后，会话里能看到对应 env 已注入。

## 8. 最小验收清单

只要下面都成立，这套设计就算过关：

1. `--key` 能覆盖默认策略。
2. `round_robin` 连续启动会按顺序换 key。
3. Codex 配了自定义 `env_key` 时，`agx` 注入的是这个自定义变量。
4. Codex 不再注入 `OPENAI_API_BASE`（只保留 `OPENAI_BASE_URL`）。
5. 多终端同时切换/启动，不出现状态错乱。
