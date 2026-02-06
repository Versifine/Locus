# Minecraft Protocol 774 (1.21.2 / 1.21.3 / 1.21.11)

> 📚 本文档记录 Locus 项目实测确认的协议细节，作为后续开发的真理来源。
> **状态**：Client ↔ Locus ↔ Server (Offline Mode)

---

## 1. 协议流程

**1.20.2+ 新标准**：
1. **Handshaking** (State=0)
2. **Login** (State=2)
   - C→S Login Start
   - S→C Login Success
3. **Configuration** (State=3) ✅ **新增**
   - S→C Registry Data (Locus 目前透传)
   - S→C Feature Flags
   - S→C Finish Configuration (0x03)
   - C→S Finish Configuration (0x00?) - *Client Acknowledge*
4. **Play** (State=3 -> 4 ? check actual enum value)
   - 正常游戏交互

---

## 2. 关键包 ID (Play State)

### Clientbound (S→C)

| ID | Name | Description |
|----|------|-------------|
| `0x77` | **System Chat Message** | 系统消息 / Action Bar |
| `0x3f` | **Player Chat Message** | 玩家发送的消息（带签名/无签名内容） |
| `0x24` | **Keep Alive** | 心跳包 |
| `0x6c` | **Disconnect** | 踢出玩家 |

### Serverbound (C→S)

| ID | Name | Description |
|----|------|-------------|
| `0x08` | **Chat Message** | 玩家发送的普通聊天 |
| `0x06` | **Chat Command** | 玩家发送的指令 (e.g. `/help`) |
| `0x07` | **Signed Chat Command** | 带签名的指令 |

---

## 3. 包结构定义

### 3.1 S→C System Chat (0x77)
```go
type SystemChat struct {
    Content     NBTNode // Anonymous NBT (Compound)
    IsActionBar bool    // Boolean
}
```
- **Content**: 这是一个匿名的 NBT Compound，包含 `text`、`color`、`extra` 等标准 Chat Component 字段。
- **解析注意**：必须使用支持 Anonymous Root 的 NBT 解析器。

### 3.2 S→C Player Chat (0x3f)
```go
type PlayerChat struct {
    Sender      UUID
    Index       VarInt
    HasSig      Bool
    Signature   [256]Byte (Optional)
    Message     String (Plain text)
    Timestamp   Int64
    Salt        Int64
    ... (后面还有 Previous Messages, Filter Mask, Chat Type, Network Name 等)
}
```
- **Locus 策略**：目前只解析到 `Message` 和 `UnsignedContent`，后续字段按需解析或通过 `io.Reader` 顺序读取。

### 3.3 C→S Chat Message (0x08)
```go
type ChatMessage struct {
    Message     String (Max 256)
    Timestamp   Int64
    Salt        Int64
    HasSig      Bool
    Signature   [256]Byte (Optional)
    MsgCount    VarInt
    Ack         BitSet (20 bits / 3 bytes)
}
```

---

## 4. 特殊机制

### 4.1 压缩 (Compression)
- 阈值由 **Login (0x03)** 包设定。
- 格式：`[Packet Length] [Data Length] [Data]`
- 若 `Data Length == 0`，则 `Data` 为未压缩数据；否则为 zlib 压缩数据。

### 4.2 NBT
- Minecraft NBT 使用 **Big Endian**。
- 网络包中的 NBT 通常是 **Anonymous**（无根名）。
