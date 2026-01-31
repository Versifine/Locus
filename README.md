# 🎯 Locus

<p align="center">
  <b>Minecraft AI Agent</b><br>
  用 Go 编写的智能代理，让 LLM 在 Minecraft 世界中自主行动
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go" alt="Go 1.21+">
  <img src="https://img.shields.io/badge/Minecraft-Java%20Edition-62B47A?style=flat-square" alt="Minecraft Java">
  <img src="https://img.shields.io/badge/LLM-DeepSeek%20%7C%20Qwen%20%7C%20GPT-purple?style=flat-square" alt="LLM Support">
  <img src="https://img.shields.io/badge/Status-WIP-yellow?style=flat-square" alt="Status">
</p>

---

## ✨ 特性

- 🔌 **TCP 代理** - 透明转发 Minecraft 客户端和服务器之间的流量
- 🔍 **协议解析** - 解析 Minecraft 协议，理解游戏世界
- 💬 **AI 聊天** - 游戏内聊天接入 LLM，智能对话
- 🤖 **Bot 控制** - LLM 驱动的自主玩家，能移动、交互、完成任务
- 📊 **流量可视化** - 实时仪表盘展示数据包（规划中）

---

## 🏗️ 架构

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
| v0.1 | TCP 透明代理 | 🔄 开发中 |
| v0.2 | 协议解析 | ⬜ 计划中 |
| v0.3 | 聊天机器人 | ⬜ 计划中 |
| v0.4 | 世界感知 | ⬜ 计划中 |
| v0.5 | Bot 控制 | ⬜ 计划中 |
| v0.6 | 智能行为 | ⬜ 计划中 |

---

## 🛠️ 技术栈

- **语言**: Go
- **协议**: Minecraft Java Edition Protocol
- **LLM**: DeepSeek / Qwen / GLM / Kimi（可切换）
- **参考**: [wiki.vg](https://wiki.vg/Protocol)

---

## 📄 License

MIT License
