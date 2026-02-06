# Locus 任务看板

> 📋 状态说明：⬜ 待办 | 🔄 进行中 | ✅ 完成

---

## In Progress

### T020: 解析 System Chat Message (S→C) 🔄

> 解析并结构化 `system_chat` 包，为后续 Hook 做准备

**已确认包 ID (Protocol 774)**：
- `system_chat` = `0x77`

**包结构 (参考 1.21.9 / 预计一致)**：
- `content`: anonymousNbt
- `isActionBar`: bool

**步骤**：
1. 在 `internal/protocol/types.go` 增加 `ReadBool`
2. 新建 `internal/protocol/nbt.go`：自研 NBT 解析器（支持匿名根标签）
3. 新建 `internal/protocol/system_chat.go`（`SystemChat` + `ParseSystemChat`）
4. 解析 `content`（anonymousNbt）并转为可读格式（最少能打印文本/结构）
5. 读取 `isActionBar`，在 Proxy Play 状态记录日志（只打印，不改包）
6. 手动测试：聊天 + /help，确认日志输出内容

---

## Backlog

### v0.3 - 聊天拦截 + LLM 集成

> 目标：实现聊天消息拦截、LLM 集成、AI 自动回复

#### 聊天拦截
- [ ] T020: 解析 System Chat Message (S→C)
- [ ] T021: 解析 Chat Message (C→S)
- [ ] T022: 解析 Player Chat Message (S→C)
- [ ] T023: Hook 机制框架
- [ ] T024: 聊天拦截配置

#### LLM 集成
- [ ] T025: LLM 客户端封装（HTTP 调用 DeepSeek）
- [ ] T026: LLM 配置（config.yaml）

#### 整合
- [ ] T027: 聊天 Hook 实现（拦截 → LLM → 异步回复）
- [ ] T028: 回复注入（构造 C→S 聊天包）
- [ ] T029: 集成测试

---

## Done

### v0.3 - 聊天拦截（阶段性） ✅

- [x] T019: 抓包确认 1.21.11 聊天包 ID ✅ (2026-02-04)
  - ProtocolVersion=774
  - C→S `chat_message` = `0x08`
  - C→S `chat_command` = `0x06`
  - C→S `chat_command_signed` = `0x07`
  - S→C `system_chat` = `0x77`
  - S→C `player_chat` = `0x3f`

### v0.2.2 - 协议增强 ✅ (2026-02-04)

- [x] T030: 实现协议压缩/解压支持 ✅ (2026-02-04)
  - [x] 实现 zlib 压缩/解压工具
  - [x] 改造 ReadPacket/WritePacket 支持阈值
  - [x] Proxy 正确处理 Set Compression (0x03)
  - [x] 禁用 Nagle 算法 (TCP_NODELAY) 修复延迟问题

### v0.1 - TCP 透明代理 ✅ (2026-01-31)
- [ ] T020: 解析 System Chat Message (S→C)
- [ ] T021: 解析 Chat Message (C→S)
- [ ] T022: 解析 Player Chat Message (S→C)
- [ ] T023: Hook 机制框架
- [ ] T024: 聊天拦截配置

#### LLM 集成
- [ ] T025: LLM 客户端封装（HTTP 调用 DeepSeek）
- [ ] T026: LLM 配置（config.yaml）

#### 整合
- [ ] T027: 聊天 Hook 实现（拦截 → LLM → 异步回复）
- [ ] T028: 回复注入（构造 C→S 聊天包）
- [ ] T029: 集成测试

---

## Done

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

> **T018 结论**：当前设计对 Proxy 模式够用。状态跟踪不影响转发正确性，
> 最坏情况是漏掉日志/Hook。Bot 模式需重新设计。
