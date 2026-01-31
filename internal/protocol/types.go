// Package protocol 定义 Minecraft 协议的数据类型
package protocol

// Packet 表示一个 Minecraft 数据包
type Packet struct {
	Length   int32  // 包长度（不包含长度字段本身）
	PacketID int32  // 包 ID
	Data     []byte // 包数据
}

// Handshake 表示握手包的数据
type Handshake struct {
	ProtocolVersion int32  // 协议版本号
	ServerAddress   string // 服务器地址
	ServerPort      uint16 // 服务器端口
	NextState       int32  // 下一状态：1=Status, 2=Login
}

// 连接状态常量
const (
	StateHandshaking = 0
	StateStatus      = 1
	StateLogin       = 2
	StatePlay        = 3
)

// 包 ID 常量
const (
	PacketIDHandshake     = 0x00
	PacketIDStatusRequest = 0x00
	PacketIDLoginStart    = 0x00
)
