# Locus 任务看板

> 状态说明：⬜ 待办 | 🔄 进行中 | ✅ 完成

---

## In Progress

### v0.5a - 自身状态感知

> 目标：Bot 能感知自身状态（位置、生命、时间、在线玩家），并将状态注入 LLM 上下文
> 数据来源：`internal/protocol/1.21.11protocol.json`

**协议层（Protocol）**

- [x] T038: Packet ID 补全 — `S2CUpdateHealth`(0x66) / `S2CUpdateTime`(0x6f) / `S2CExperience`(0x65) / `S2CPlayerInfo`(0x44) / `S2CPlayerRemove`(0x43) ✅
- [x] T039: 解析 UpdateHealth（health:f32 + food:varint + foodSaturation:f32）✅ (2026-02-10)
- [x] T040: 解析 UpdateTime（age:i64 + worldTime:i64 + tickDayTime:bool）✅ (2026-02-10)
- [x] T041: 解析 Experience（experienceBar:f32 + level:varint + totalExperience:varint）✅ (2026-02-10)
- [x] T042: 解析 PlayerInfo — 仅提取 add_player 动作（UUID + name），跳过其余 bitflag 分支 ✅ (2026-02-10)
- [x] T043: 解析 PlayerRemove（players: array of UUID）✅ (2026-02-10)

**世界状态（WorldState）**

- [ ] T044: 新建 `internal/world/state.go` — WorldState 结构体（Position / Health / Food / Time / PlayerList），线程安全读写
- [ ] T045: Bot 集成 — handlePlayState 中分发新包到 WorldState 更新方法

**Agent 集成**

- [ ] T046: Agent 注入 WorldState 摘要 — 每次调 LLM 时将当前状态序列化为 system prompt 的一部分
- [ ] T047: 端到端验收 — 进入服务器后能回答"你在哪""你血量多少""现在几点了""谁在线"

---

## Done

### v0.4 - Headless Bot（架构转折）

- [x] T037: 端到端验收（ChatMessage 包构造 + 自触发过滤 + 滑动窗口记忆）✅ (2026-02-10)
- [x] T036: main.go 重写 — Bot 为主路径（switch cfg.Mode 分流）✅ (2026-02-10)
- [x] T035: Headless Bot 核心（login/configuration/play/injection 全流程）✅ (2026-02-10)
- [x] T034: Agent 重构 — MessageSender 接口 ✅ (2026-02-09)
- [x] T033: Config 扩展 — Bot 配置（Mode + BotConfig）✅ (2026-02-09)
- [x] T032: Protocol 扩展 — 包构造函数（Handshake/Login/Configuration/KeepAlive/PlayerPosition + packet_id 补全）✅ (2026-02-08)
- [x] T031: Protocol 扩展 — Write 辅助函数（WriteUUID/WriteUnsignedShort/WriteBool/WriteInt64/WriteFloat/WriteDouble + GenerateOfflineUUID）✅ (2026-02-07)

### v0.3 - LLM 集成 + 聊天回复 ✅

- [x] T027: 端到端验收 ✅ (2026-02-07)
- [x] T026: 聊天 → LLM → 回复 串联（ChatEventHandler + goroutine 异步 + SplitByRunes 长度限制 + ctx 穿透）✅ (2026-02-07)
- [x] T025: 回复注入通道（SendMsgToServer + ChatCommand 构造 + connCtx 生命周期）✅ (2026-02-07)
- [x] T024: LLM 客户端 + 配置（DeepSeek API 封装 + 单元测试）✅ (2026-02-07)
- [x] T023: Hook 机制框架（事件总线 + Agent 消费者）✅ (2026-02-06)

### v0.3.1 - 代码质量治理 ✅ (2026-02-06)

- [x] T028: 安全 with 正确性修复（unsafe 移除、连接泄漏、解析中断）✅ (2026-02-06)
- [x] T029: relayPackets 拆分 + 包 ID 常量化 ✅ (2026-02-06)
- [x] T030: 日志配置生效 + ChatMessage 字段命名修正 ✅ (2026-02-06)

- [x] T022: 解析 Player Chat Message (S→C) ✅ (2026-02-06)
- [x] T021: 解析 Chat Message (C→S) ✅ (2026-02-06)
- [x] T020: 解析 System Chat Message (S→C) ✅ (2026-02-06)
- [x] T019: 抓包确认 1.21.11 聊天包 ID ✅ (2026-02-04)

### v0.2.2 - 协议增强 ✅ (2026-02-04)

- [x] T030: 实现协议压缩/解压支持 ✅ (2026-02-04)

### v0.1 - TCP 透明代理 ✅ (2026-01-31)

- [x] T001: 初始化 Go 项目结构
- [x] T002: 实现 YAML 配置加载
- [x] T003: 实现 TCP Listener
- [x] T004: 实现双向流量转发
- [x] T005: 添加日志输出
- [x] T006: 手动测试验证

### v0.2 - 协议解析 ✅ (2026-02-03)

- [x] T007: 实现 VarInt/VarLong 编解码 ✅ (2026-01-31)
- [x] T008: 实现 Packet 读写器 ✅ (2026-02-03)
- [x] T009: 重构 Proxy，接入协议解析（能打印包 ID） ✅ (2026-02-03)
- [x] T010: 解析 Handshake 包 ✅ (2026-02-03)
- [x] T011: 解析 Login Start 包 ✅ (2026-02-03)
- [x] T012: 跟踪连接状态 (Handshaking → Login → Play) ✅ (2026-02-03)

### v0.2.1 - 地基补强 ✅ (2026-02-04)

- [x] T013: 自定义错误类型（protocol 层） ✅ (2026-02-04)
- [x] T014: 日志抽象层（slog） ✅ (2026-02-04)
- [x] T015: config 单元测试 ✅ (2026-02-04)
- [x] T016: proxy 集成测试 ✅ (2026-02-04)
- [x] T017: 优雅关闭（context + 信号处理） ✅ (2026-02-04)
- [x] T018: ConnState 设计评审 ✅ (2026-02-04)
