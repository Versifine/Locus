# Locus 架构文档

## 概述

Locus 是一个 Minecraft 代理服务器，用于在客户端和服务端之间进行数据包拦截和修改。

## 目录结构

```
locus/
├── cmd/locus/main.go      # 入口文件
├── internal/
│   ├── proxy/             # TCP 连接管理
│   ├── protocol/          # 协议解析
│   └── hook/              # 拦截器逻辑
├── configs/               # 配置文件
└── docs/                  # 文档
```

## 数据流

```
[Minecraft 客户端] <--TCP--> [Locus 代理] <--TCP--> [Minecraft 服务端]
                              │
                              ├── protocol (解码/编码)
                              └── hook (拦截/修改)
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

### 握手流程

1. 客户端发送 Handshake 包
2. 根据 NextState 进入 Status 或 Login 状态
3. Login 状态下完成身份验证后进入 Play 状态
