// Package hook 负责拦截器逻辑
// 将来的 AI 和数据修改逻辑放在这里
package hook

import "github.com/Versifine/Locus/internal/protocol"

// Hook 定义拦截器接口
type Hook interface {
	// OnPacket 在收到数据包时被调用
	// 返回修改后的包，或返回 nil 表示丢弃该包
	OnPacket(packet *protocol.Packet, fromClient bool) *protocol.Packet
}

// DefaultHook 默认拦截器实现，不做任何修改
type DefaultHook struct{}

// OnPacket 默认实现，直接返回原包
func (h *DefaultHook) OnPacket(packet *protocol.Packet, fromClient bool) *protocol.Packet {
	return packet
}
