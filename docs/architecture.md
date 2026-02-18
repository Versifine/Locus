# Locus 架构文档

## 概述

Locus 是一个 Headless Minecraft Bot，作为独立玩家登录 MC 服务器，通过 LLM 驱动行为。

## 分层架构

```
┌─────────────────────────────────────────────────────┐
│  Agent（决策层）                                      │
│  LLM 选择技能："走到 Steve 身边并打招呼"              │
│  频率：每几秒一次                                     │
├─────────────────────────────────────────────────────┤
│  Skill（技能层）                                      │
│  WalkTo / LookAt / Speak / ...                       │
│  每 tick 输出 InputState（按键信号）                   │
│  频率：每 50ms (20 tick/s)                            │
├─────────────────────────────────────────────────────┤
│  Body（身体层）                                       │
│  InputState → 物理计算 → 位置/动作                    │
│  v0.6: 简单偏移    v0.7+: 物理引擎                    │
│  频率：每 tick                                        │
├─────────────────────────────────────────────────────┤
│  Bot（连接层）                                        │
│  Protocol 编解码 / 登录 / Keep-Alive / 收发包         │
│  WorldState 更新 / EventBus 事件发布                  │
├─────────────────────────────────────────────────────┤
│  MC 服务器                                            │
│  TCP (Protocol 774 / MC 1.21.11)                     │
└─────────────────────────────────────────────────────┘
```

## 核心数据流

### 感知（上行）

```
MC 服务器 → TCP 包 → Protocol 解码 → Bot 分发
  → WorldState 更新（位置/血量/实体/玩家）
  → EventBus 发布事件（聊天/战斗/...）
  → Agent 获取 Snapshot
```

### 行动（下行）

```
Agent 选择技能（LLM 决策）
  → Skill.Tick(snapshot) 输出 InputState（按键信号）
    → Body.Tick(input) 转为位置/动作
      → Bot 发 Protocol 包 → MC 服务器
```

### InputState（按键信号）

技能层的唯一输出格式，等同于玩家的键鼠输入：

```
InputState {
    Forward / Backward / Left / Right   — WASD
    Jump / Sneak / Sprint               — Space / Shift / Ctrl
    Attack / Use                        — 左键 / 右键
    Yaw / Pitch                         — 鼠标朝向
}
```

技能不直接发包、不直接改位置。它只设置"我要按什么键"。
Body 负责把按键信号转成物理位置变化和协议包。

## 记忆架构

```
工作记忆（Working Memory）  ← Tool Use 当前消息链（单次 Thinker 内）
短期记忆（Short-term Memory）← EpisodeLog（最近经历，自动开单-闭单）
长期记忆（Long-term Memory） ← MemoryStore（remember 写入 / recall 检索）
```

### 长期记忆（v0.6c）

- 条目结构：`content + tags + pos + tick + embedding + hit_count + last_hit_tick`
- 写入方式：LLM 显式 `remember(content, tags?)` + Agent 规则兜底自动记忆
- 检索方式：`recall(query, filter?)` 混合检索（关键词 + embedding）
- 返回格式（开发期）：`[{content, tags, tick, score}]`
- 时序一致性：Event/Episode/Memory 统一使用单调 TickID

## 目录结构

```
locus/
├── cmd/locus/main.go        # 入口：按 mode 启动 Bot
├── internal/
│   ├── bot/                 # 连接层（登录、保活、收发包）
│   ├── body/                # 身体层（InputState → 位置包）  [v0.6 新增]
│   ├── skill/               # 技能层（Skill 接口 + Registry）[v0.6 新增]
│   ├── agent/               # 决策层（Agent Loop + LLM 调用）
│   ├── world/               # 世界状态（Snapshot、实体、玩家）
│   ├── protocol/            # MC 协议（VarInt、Packet、NBT...）
│   ├── event/               # 事件总线（发布/订阅）
│   ├── llm/                 # LLM API 客户端（DeepSeek）
│   ├── config/              # YAML 配置加载
│   ├── logger/              # 结构化日志（slog）
│   └── proxy/               # TCP 代理（已归档）
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
