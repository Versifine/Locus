Locus Work Protocol (人机协作协议)

Operator: Versifine (Human / Architect)
Navigator: AI (Copilot / LLM)

本文档定义了 Locus 项目中人类开发者与 AI 助手的协作流程与代码规范。

1. 角色定义 (Roles)

🧑‍💻 Operator (你)

决策者：决定功能优先级，决定是否接受 AI 的代码。

调试者：在 Minecraft 游戏中进行实际测试，观察现象。

审查者：绝对不要盲目粘贴代码。必须理解每一行 Go 代码的含义（尤其是并发部分）。

哲学指导：确保代码架构符合 Locus Theory 的“模块化”与“轨迹”理念。

🤖 Navigator (AI)

生成器：编写繁琐的协议解析代码 (VarInt, Packet Marshalling)。

百科全书：查询 wiki.vg 协议文档，解释 Minecraft 特定数据包的结构。

单元测试员：为复杂的逻辑（如 TCP 拆包）编写 _test.go。

橡皮鸭：当你卡住时，向我解释你的思路，我会帮你理清逻辑漏洞。

2. 交互模式 (Prompting Strategy)

为了获得最高质量的 Go 代码，请遵循以下 Prompt 模式：

✅ 模式 A：新增功能 (Feature Implementation)

公式：[角色设定] + [上下文] + [具体任务] + [约束条件]

示例：

"作为一名 Go 高性能网络专家，基于 internal/protocol 包，请为我实现 HandshakePacket 的解码器。参考 wiki.vg 的定义，它包含 Protocol Version (VarInt), Server Address (String), Server Port (UShort), Next State (VarInt)。请确保处理 EOF 错误。"

✅ 模式 B：Bug 修复 (Debugging)

公式：[现象描述] + [相关代码片段] + [错误日志] + [猜测方向]

示例：

"我在测试连接时，Locus 控制台打印了 panic: runtime error: slice bounds out of range。这是我的 decoder.go 代码 [粘贴代码]。这是堆栈信息 [粘贴 Log]。是不是我在读取 VarInt 时没有判断 buffer 剩余长度？"

✅ 模式 C：代码审查 (Refactoring)

公式：[粘贴代码] + "这段代码有优化的空间吗？特别是内存分配和并发安全方面。"

3. 代码规范 (Coding Standards)

为了保持项目像大厂开源项目一样专业，遵循以下 Go 规范：

目录结构：严格遵守 Standard Go Project Layout。internal 下的代码不可被外部引用。

命名风格：

接口名用 er 结尾 (e.g., PacketDecoder, ConnectionHandler).

私有变量用小驼峰 (bufferSize), 公有变量用大驼峰 (MaxConnections).

错误处理：

禁止 _ 忽略错误（除非你有 100% 把握）。

网络 IO 错误必须 Log 出来，方便排查。

并发控制：

所有的 go func() 必须考虑退出机制（Context 或 Channel）。

共享资源（如 map）必须加锁 (sync.RWMutex)。

4. Git 提交规范

每次完成一个阶段，使用以下格式提交：

feat: 实现 VarInt 解码器

fix: 修复 TCP 粘包导致的 Panic

docs: 更新架构图

refactor: 优化 buffer 内存复用