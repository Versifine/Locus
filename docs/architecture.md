# Locus 架构文档

## 概述

Locus 是一个 Headless Minecraft Bot，作为独立玩家登录 MC 服务器，通过 LLM 驱动行为。

## 数据流

```
                    ┌─────────────┐
                    │  MC 服务器   │
                    └──────┬──────┘
                           │ TCP (MC Protocol 774)
                    ┌──────┴──────┐
                    │  Locus Bot  │
                    ├─────────────┤
                    │  Protocol   │  包读写 / 编解码 / 状态机
                    │  Bot        │  登录 / Keep-Alive / 收发包
                    │  EventBus   │  事件发布与订阅
                    │  Agent      │  订阅事件 → 调用 LLM → 执行动作
                    │  LLM        │  DeepSeek API
                    └─────────────┘
```

## 目录结构

```
locus/
├── cmd/locus/main.go        # 入口：按 mode 启动 Bot 或 Proxy
├── internal/
│   ├── bot/                 # Headless Bot（登录、保活、收发包）
│   ├── agent/               # AI 决策层（事件订阅、LLM 调用）
│   ├── protocol/            # MC 协议（VarInt、Packet、Chat、NBT...）
│   ├── event/               # 事件总线（发布/订阅）
│   ├── llm/                 # LLM API 客户端（DeepSeek）
│   ├── config/              # YAML 配置加载
│   ├── logger/              # 结构化日志（slog）
│   └── proxy/               # TCP 代理（已归档，仅用于协议调试）
├── configs/config.yaml      # 运行配置
└── docs/                    # 文档 + 研究笔记
```

## Minecraft 协议简介

### VarInt 编码

Minecraft 使用变长整数（VarInt）来节省带宽：
- 每个字节的最高位表示是否还有后续字节
- 其余 7 位用于存储数据

### 数据包格式

```
| 包长度 (VarInt) | 包ID (VarInt) | 数据 (Byte Array) |
```

压缩模式下：
```
| 包长度 (VarInt) | 原始数据长度 (VarInt) | 压缩数据 (zlib) |
```

### 连接状态机

```
Handshaking → Login → Configuration → Play
```

1. 客户端发送 Handshake 包（NextState=2 进入 Login）
2. Login：发送 LoginStart，接收 SetCompression + LoginSuccess
3. 客户端发送 LoginAcknowledged，进入 Configuration
4. Configuration：交换配置数据（Registry、Tags、KnownPacks），FinishConfiguration
5. Play：游戏态，处理 Keep-Alive、聊天、位置同步等
