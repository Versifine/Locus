# Locus 任务看板

> 状态说明：⬜ 待办 | 🔄 进行中 | ✅ 完成

---

## In Progress

（无）

---

## Backlog

### v0.4 - Headless Bot（架构转折）

> 目标：Locus 作为独立客户端登录 MC 服务器，拥有自己的身份，能收聊天、调 LLM、自动回复。
> Proxy 归档，Bot 成为核心。

#### T031: Protocol 扩展 — Write 辅助函数
> 为 Bot 构造发送包提供基础设施

**内容**：
1. `types.go` 添加 `WriteUUID`, `WriteUnsignedShort`, `WriteBool`, `WriteInt64`, `WriteFloat`, `WriteDouble`
2. `types.go` 添加 `GenerateOfflineUUID(username)` — MD5("OfflinePlayer:" + username), version=3
3. 单元测试

---

#### T032: Protocol 扩展 — 包构造函数
> Bot 登录和保活需要的所有包

**内容**：
1. `handshake.go` 添加 `CreateHandshakePacket(protocolVersion, serverAddr, serverPort, nextState)`
2. `login.go` 添加 `CreateLoginStartPacket(username, uuid)`, `CreateLoginAcknowledgedPacket()`
3. 新建 `configuration.go` — `CreateClientInformationPacket`, `CreateBrandPluginMessagePacket`, `CreateKnownPacksResponsePacket`, `CreateFinishConfigurationAckPacket`
4. 新建 `keep_alive.go` — `ParseKeepAlive`, `CreateKeepAliveResponsePacket` (Play + Configuration)
5. 新建 `player_position.go` — `ParseSyncPlayerPosition`, `CreateConfirmTeleportationPacket`
6. `packet_id.go` 补充所有新增包 ID（需抓包验证 Protocol 774）

---

#### T033: Config 扩展 — Bot 配置
> 支持 bot 模式选择和 Bot 参数

**内容**：
1. `config.go` 添加 `Mode string` 和 `BotConfig{Username}`
2. `config.yaml` 添加 `mode: "bot"` 和 `bot.username: "Locus"`

---

#### T034: Agent 重构 — MessageSender 接口
> 解除 Agent 对 proxy.Server 的硬依赖

**内容**：
1. 定义 `MessageSender` 接口（`SendMsgToServer(msg string)`）
2. Agent 结构体中 `server *proxy.Server` → `sender MessageSender`
3. 确保 `proxy.Server` 和未来的 `bot.Bot` 都满足该接口
4. 现有测试通过

---

#### T035: Headless Bot 核心
> v0.4 的主体工作

**内容**：
1. 新建 `internal/bot/bot.go`
2. `Bot` 结构体：`serverAddr`, `username`, `uuid`, `conn`, `connState`, `eventBus`, `injectCh`, `mu`
3. `login()` — Handshake → LoginStart → 处理 SetCompression/LoginSuccess → 发 LoginAcknowledged
4. `handleConfiguration()` — 发 ClientInformation + Brand → 处理 KnownPacks/KeepAlive/FinishConfiguration
5. `readLoop()` — Play 态持续读包：KeepAlive 应答、位置同步确认、聊天事件发布
6. `handleInjects()` — 从 injectCh 读消息 → CreateSayChatCommand → WritePacket
7. `Start(ctx)` — 组装上述流程，阻塞直到 ctx 取消
8. `Bus()`, `SendMsgToServer(msg)` — 公开接口

---

#### T036: main.go 重写 — Bot 为主路径
> 按 config.Mode 启动 Bot 或 Proxy

**内容**：
1. `mode: "bot"` → 创建 Bot + Agent，启动 Bot
2. `mode: "proxy"` (或默认) → 保持现有 Proxy 流程
3. 验证 Bot 模式下完整流程：启动 → 登录 → 保活 → 聊天回复

---

#### T037: 端到端验收
> v0.4 整体验收

**步骤**：
1. 配置 `mode: "bot"`, 指向离线模式 MC 服务器
2. 启动 Locus，确认日志显示 Handshake → Login → Configuration → Play
3. 确认 Bot 在服务器 Tab 列表中可见
4. Bot 保持在线 > 30 秒不被踢（Keep-Alive 验证）
5. 游戏内发消息，确认 Bot 通过 LLM 回复
6. `go test ./...` 全部通过
7. 代码审查 + 提交

---

## Done

### v0.3 - LLM 集成 + 聊天回复 ✅

- [x] T027: 端到端验收 ✅ (2026-02-07)
- [x] T026: 聊天 → LLM → 回复 串联（ChatEventHandler + goroutine 异步 + SplitByRunes 长度限制 + ctx 穿透）✅ (2026-02-07)
- [x] T025: 回复注入通道（SendMsgToServer + ChatCommand 构造 + connCtx 生命周期）✅ (2026-02-07)
- [x] T024: LLM 客户端 + 配置（DeepSeek API 封装 + 单元测试）✅ (2026-02-07)
- [x] T023: Hook 机制框架（事件总线 + Agent 消费者）✅ (2026-02-06)

### v0.3.1 - 代码质量治理 ✅ (2026-02-06)

- [x] T028: 安全与正确性修复（unsafe 移除、连接泄漏、解析中断）✅ (2026-02-06)
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
