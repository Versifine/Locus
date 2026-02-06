# 🎯 Locus

<p align="center">
  <b>Minecraft 仿生 AI Agent</b><br>
  用 Go 编写的智能代理，目标是构建具有"灵魂"的仿生 AI——像人类一样感知、思考、学习、行动
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go" alt="Go 1.21+">
  <img src="https://img.shields.io/badge/Minecraft-Java%20Edition-62B47A?style=flat-square" alt="Minecraft Java">
  <img src="https://img.shields.io/badge/LLM-DeepSeek%20%7C%20Qwen%20%7C%20GPT-purple?style=flat-square" alt="LLM Support">
  <img src="https://img.shields.io/badge/Status-WIP-yellow?style=flat-square" alt="Status">
</p>

---
> **注意：这是个人学习/实验项目，暂时不接受外部贡献。**  
> 欢迎 Fork 和使用，但 Issue 和 PR 可能不会及时处理。
---

## ✨ 特性

- 🔌 **TCP 代理** - 透明转发 Minecraft 客户端和服务器之间的流量
- 🔍 **协议解析** - 解析 Minecraft 协议，理解游戏世界
- 💬 **AI 聊天** - 游戏内聊天接入 LLM，智能对话
- 🤖 **Bot 控制** - LLM 驱动的自主玩家，能移动、交互、完成任务
- 📊 **流量可视化** - 实时仪表盘展示数据包（规划中）

---

## 🧠 仿生架构

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
│                 Protocol + Bot + World State                 │
└─────────────────────────────────────────────────────────────┘
```

| 层级 | 名称 | 对应人类 | 确定性 |
|------|------|----------|--------|
| L0 | 身体 | 感官 + 肌肉 | 高，可规划 |
| L1 | 技能 | 肌肉记忆 | 中，分阶段 |
| L2 | 思考 | 本能/直觉/理性 | 低，研究性 |
| L3 | 记忆 | 工作/长期记忆 | 低，研究性 |
| L4 | 情绪/人格 | 情感/性格 | 低，研究性 |
| L5 | 意识 | 自我觉察 | 探索性 |

---

## 🔬 研究方向

| 课题 | 描述 | 灵感来源 |
|------|------|----------|
| Mind Loop | 持续运行的意识循环，不依赖外部输入 | 认知科学、意识理论 |
| 快/慢系统 | 双系统架构：快速反射 + 深度思考 | Kahneman《思考，快与慢》 |
| 情绪与人格 | 情绪状态影响决策，持久的人格特征 | 情感计算、Damasio |
| 记忆系统 | 工作记忆 + 情景记忆 + 语义记忆 + 程序性记忆 | 认知神经科学 |
| 肌肉记忆学习 | 从预定义 → 模仿 → 强化学习 → 自主创造 | 运动学习、强化学习 |

---

## 🏗️ 系统架构

```
[MC 客户端] ◄──► [Locus 代理] ◄──► [MC 服务器]
                    │
                    ├── 协议解析
                    ├── 世界状态
                    └── AI 大脑 (LLM)
```

---

## 🚀 快速开始

### 前置要求

- Go 1.21+
- Minecraft Java Edition 服务器（离线模式）
- LLM API Key（DeepSeek / Qwen / 其他）

### 编译

```bash
go build -o locus ./cmd/locus
```

### 配置

编辑 `configs/config.yaml`：

```yaml
listen:
  host: "0.0.0.0"
  port: 25565

backend:
  host: "127.0.0.1"
  port: 25566

llm:
  provider: "deepseek"
  api_key: "your-api-key"
```

### 运行

```bash
./locus
```

然后在 Minecraft 客户端连接 `localhost:25565`。

---

## 📅 路线图

| 版本 | 功能 | 状态 |
|------|------|------|
| v0.1 | TCP 透明代理 | ✅ 完成 |
| v0.2 | 协议解析 | 🔄 开发中 |
| v0.3 | 聊天机器人 | ⬜ 计划中 |
| v0.4 | 世界感知 | ⬜ 计划中 |
| v0.5 | Bot 控制 | ⬜ 计划中 |
| v0.6 | 智能行为 | ⬜ 计划中 |

### v0.2 协议解析进度

- ✅ 基础包结构 (VarInt, VarLong, String, UUID...)
- ✅ 握手 & 状态机
- ✅ 登录流程
- ✅ NBT 解析器
- ✅ 系统聊天消息 (S→C)
- 🔄 玩家聊天消息 (S→C)
- 🔄 客户端聊天消息 (C→S)

---

## 🛠️ 技术栈

- **语言**: Go
- **协议**: Minecraft Java Edition Protocol
- **LLM**: DeepSeek / Qwen / GLM / Kimi（可切换）
- **参考**: [wiki.vg](https://wiki.vg/Protocol)

---

## 📄 License

MIT License
