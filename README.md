# Locus

<p align="center">
  <b>Minecraft 仿生 AI Agent</b><br>
  用 Go 编写的 Minecraft 智能体，目标是构建具有"灵魂"的仿生 AI——像人类一样感知、思考、学习、行动
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go" alt="Go 1.21+">
  <img src="https://img.shields.io/badge/MC-Java%201.21.11-62B47A?style=flat-square" alt="MC Java 1.21.11">
  <img src="https://img.shields.io/badge/LLM-DeepSeek-purple?style=flat-square" alt="LLM">
  <img src="https://img.shields.io/badge/Status-v0.4-blue?style=flat-square" alt="Status">
</p>

---
> **注意：这是个人学习/实验项目，暂时不接受外部贡献。**
> 欢迎 Fork 和使用，但 Issue 和 PR 可能不会及时处理。
---

## 它是什么

Locus 是一个 **Headless Minecraft Bot**——它自己作为一个独立玩家登录 MC 服务器，通过 LLM（大语言模型）驱动行为。

不需要真人客户端，不需要 Mod，不需要插件。Locus 直接用原生 MC 协议和服务器通信。

```
                    ┌─────────────┐
                    │  MC 服务器   │
                    └──────┬──────┘
                           │ TCP
                    ┌──────┴──────┐
                    │  Locus Bot  │  ← 独立玩家身份
                    │  Protocol   │  ← 原生 MC 协议
                    │  Agent      │  ← 事件驱动
                    │  LLM        │  ← DeepSeek API
                    └─────────────┘
```

## 当前能力 (v0.4)

- **Headless Bot** — Locus 作为独立客户端直连 MC 服务器，完整 Login → Configuration → Play 流程
- **保活机制** — Keep-Alive 应答 + 位置同步确认，Bot 稳定在线
- **协议解析** — 完整实现 MC 1.21.11 (Protocol 774) 的包读写、压缩、状态机
- **聊天拦截** — 捕获 PlayerChat / SystemChat / ChatMessage / ChatCommand
- **LLM 集成** — 调用 DeepSeek API，异步生成回复
- **聊天回复** — AI 生成的回复自动发送到游戏内
- **对话记忆** — 按玩家隔离的滑动窗口历史，支持多轮连贯对话

## 下一步 (v0.5)

- **世界感知** — 解析位置、区块、实体数据
- **视野过滤** — 只向 AI 提供视锥范围内的信息

---

## 仿生架构

> **核心理念**：不是让 AI 机械地执行脚本，而是模仿人类的认知架构——
> 有意识循环、有情绪波动、有记忆沉淀、有肌肉记忆、有直觉与深思。

```
┌─────────────────────────────────────────────────────────────┐
│                    意识 (Consciousness)                      │
│                 "我在做什么？我想要什么？"                     │
├─────────────────────────────────────────────────────────────┤
│   情绪 (Emotion)           │        人格 (Personality)       │
│   当前状态影响决策           │        长期稳定的倾向            │
├─────────────────────────────────────────────────────────────┤
│                      记忆 (Memory)                           │
│  工作记忆 ←→ 短期记忆 ←→ 长期记忆 ←→ 程序性记忆（肌肉记忆）    │
├─────────────────────────────────────────────────────────────┤
│                    思考 (Reasoning)                          │
│        Reflex (本能) → Fast (直觉) → Slow (深思)             │
├─────────────────────────────────────────────────────────────┤
│                    技能 (Skills)                             │
│            预定义 → 模仿学习 → 强化学习 → 自主创造             │
├─────────────────────────────────────────────────────────────┤
│                    身体 (Body) ← 当前阶段                    │
│          感知世界 / 执行动作 / 状态反馈 / 时间控制              │
│              Headless Bot + Protocol Layer                   │
└─────────────────────────────────────────────────────────────┘
```

| 层级 | 名称 | 对应人类 | 状态 |
|------|------|----------|------|
| L0 | 身体 | 感官 + 肌肉 | 构建中 |
| L1 | 技能 | 肌肉记忆 | 规划中 |
| L2 | 思考 | 本能/直觉/理性 | 研究中 |
| L3 | 记忆 | 工作/长期记忆 | 待研究 |
| L4 | 情绪/人格 | 情感/性格 | 待研究 |
| L5 | 意识 | 自我觉察 | 探索性 |

---

## 研究方向

| 课题 | 描述 | 灵感来源 |
|------|------|----------|
| Mind Loop | 持续运行的意识循环，不依赖外部输入 | 认知科学、意识理论 |
| 快/慢系统 | 双系统架构：快速反射 + 深度思考 | Kahneman《思考，快与慢》 |
| 情绪与人格 | 情绪状态影响决策，持久的人格特征 | 情感计算、Damasio |
| 记忆系统 | 工作记忆 + 情景记忆 + 语义记忆 + 程序性记忆 | 认知神经科学 |
| 肌肉记忆学习 | 从预定义 → 模仿 → 强化学习 → 自主创造 | 运动学习、强化学习 |

---

## 快速开始

### 前置要求

- Go 1.21+
- Minecraft Java Edition 服务器（离线模式，`online-mode=false`）
- LLM API Key（DeepSeek）

### 编译

```bash
go build -o locus ./cmd/locus
```

### 配置

编辑 `configs/config.yaml`：

```yaml
mode: "bot"

bot:
  username: "Locus"

backend:
  host: "127.0.0.1"
  port: 25565

llm:
  model: "deepseek-chat"
  api_key: "your-api-key"
  endpoint: "https://api.deepseek.com/v1/chat/completions"
  system_prompt: "你是 Minecraft 中的 AI 玩家，简短回复"
  max_tokens: 512
  temperature: 0.7
  timeout: 30
  max_history: 20  # 每个玩家保留的对话历史条数
```

### 运行

```bash
./locus
```

Bot 会自动登录配置的 MC 服务器，在游戏内和其他玩家聊天。

---

## 路线图

| 版本 | 功能 | 状态 |
|------|------|------|
| v0.1 | TCP 代理 + 配置 | ✅ 完成 |
| v0.2 | 协议解析层 | ✅ 完成 |
| v0.3 | 聊天拦截 + LLM 集成 | ✅ 完成 |
| v0.4 | Headless Bot + 保活 + 对话记忆 | ✅ 完成 |
| **v0.5** | **世界感知 + 视野系统** | **下一步** |
| v0.6 | 原子动作 + 状态反馈 | 规划中 |
| v0.7+ | 技能框架 + 认知架构 | 研究中 |

---

## 项目结构

```
locus/
├── cmd/locus/main.go        # 入口
├── internal/
│   ├── bot/                 # Headless Bot（v0.4）
│   ├── agent/               # AI 决策层
│   ├── protocol/            # MC 协议解析与构造
│   ├── event/               # 事件总线
│   ├── llm/                 # LLM API 客户端
│   ├── config/              # 配置加载
│   ├── logger/              # 日志
│   └── proxy/               # TCP 代理（已归档）
├── configs/config.yaml      # 运行配置
└── docs/                    # 文档 + 研究笔记
```

## 技术栈

- **语言**: Go
- **协议**: Minecraft Java Edition 1.21.11 (Protocol 774)
- **LLM**: DeepSeek（OpenAI 兼容格式，可切换）
- **参考**: [minecraft.wiki](https://minecraft.wiki/w/Java_Edition_protocol) / [minecraft-data](https://github.com/PrismarineJS/minecraft-data)

---

## License

MIT License
