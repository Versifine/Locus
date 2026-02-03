package protocol

import (
	"bytes"
	"testing"
)

// buildHandshakePayload 构建握手包的字节数据用于测试
func buildHandshakePayload(protocolVersion int32, serverAddress string, serverPort uint16, nextState int32) []byte {
	buf := &bytes.Buffer{}
	WriteVarint(buf, protocolVersion)
	// 写入字符串长度和内容
	WriteVarint(buf, int32(len(serverAddress)))
	buf.WriteString(serverAddress)
	// 写入端口 (Big Endian)
	buf.WriteByte(byte(serverPort >> 8))
	buf.WriteByte(byte(serverPort & 0xFF))
	// 写入 nextState
	WriteVarint(buf, nextState)
	return buf.Bytes()
}

// TestPerseHandShake 测试握手包解析
func TestPerseHandShake(t *testing.T) {
	tests := []struct {
		name            string
		protocolVersion int32
		serverAddress   string
		serverPort      uint16
		nextState       int32
	}{
		{
			name:            "标准状态查询请求",
			protocolVersion: 764, // 1.20.2
			serverAddress:   "localhost",
			serverPort:      25565,
			nextState:       1, // Status
		},
		{
			name:            "标准登录请求",
			protocolVersion: 764,
			serverAddress:   "mc.example.com",
			serverPort:      25565,
			nextState:       2, // Login
		},
		{
			name:            "自定义端口",
			protocolVersion: 764,
			serverAddress:   "play.server.net",
			serverPort:      19132,
			nextState:       2,
		},
		{
			name:            "旧版本协议",
			protocolVersion: 47, // 1.8.9
			serverAddress:   "oldserver.com",
			serverPort:      25565,
			nextState:       1,
		},
		{
			name:            "IP地址",
			protocolVersion: 764,
			serverAddress:   "192.168.1.100",
			serverPort:      25565,
			nextState:       2,
		},
		{
			name:            "IPv6地址",
			protocolVersion: 764,
			serverAddress:   "::1",
			serverPort:      25565,
			nextState:       1,
		},
		{
			name:            "端口为0",
			protocolVersion: 764,
			serverAddress:   "test.com",
			serverPort:      0,
			nextState:       1,
		},
		{
			name:            "最大端口号",
			protocolVersion: 764,
			serverAddress:   "test.com",
			serverPort:      65535,
			nextState:       2,
		},
		{
			name:            "空服务器地址",
			protocolVersion: 764,
			serverAddress:   "",
			serverPort:      25565,
			nextState:       1,
		},
		{
			name:            "长服务器地址",
			protocolVersion: 764,
			serverAddress:   "very.long.subdomain.example.minecraft.server.domain.com",
			serverPort:      25565,
			nextState:       2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := buildHandshakePayload(tt.protocolVersion, tt.serverAddress, tt.serverPort, tt.nextState)

			handshake, err := PerseHandShake(payload)
			if err != nil {
				t.Fatalf("PerseHandShake() 返回错误: %v", err)
			}

			if handshake.ProtocolVersion != tt.protocolVersion {
				t.Errorf("ProtocolVersion = %d, 期望 %d", handshake.ProtocolVersion, tt.protocolVersion)
			}
			if handshake.ServerAddress != tt.serverAddress {
				t.Errorf("ServerAddress = %q, 期望 %q", handshake.ServerAddress, tt.serverAddress)
			}
			if handshake.ServerPort != tt.serverPort {
				t.Errorf("ServerPort = %d, 期望 %d", handshake.ServerPort, tt.serverPort)
			}
			if handshake.NextState != tt.nextState {
				t.Errorf("NextState = %d, 期望 %d", handshake.NextState, tt.nextState)
			}
		})
	}
}

// TestPerseHandShakeErrors 测试错误情况
func TestPerseHandShakeErrors(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
	}{
		{
			name:    "空输入",
			payload: []byte{},
		},
		{
			name:    "只有协议版本",
			payload: []byte{0xFC, 0x05}, // 764 的 varint 编码
		},
		{
			name:    "缺少端口",
			payload: func() []byte {
				buf := &bytes.Buffer{}
				WriteVarint(buf, 764)
				WriteVarint(buf, 9)
				buf.WriteString("localhost")
				return buf.Bytes()
			}(),
		},
		{
			name:    "缺少nextState",
			payload: func() []byte {
				buf := &bytes.Buffer{}
				WriteVarint(buf, 764)
				WriteVarint(buf, 9)
				buf.WriteString("localhost")
				buf.WriteByte(0x63)
				buf.WriteByte(0xDD) // 25565
				return buf.Bytes()
			}(),
		},
		{
			name:    "端口不完整-只有一个字节",
			payload: func() []byte {
				buf := &bytes.Buffer{}
				WriteVarint(buf, 764)
				WriteVarint(buf, 9)
				buf.WriteString("localhost")
				buf.WriteByte(0x63) // 只有一个字节
				return buf.Bytes()
			}(),
		},
		{
			name:    "字符串长度声明大于实际",
			payload: func() []byte {
				buf := &bytes.Buffer{}
				WriteVarint(buf, 764)
				WriteVarint(buf, 100) // 声明100字节
				buf.WriteString("short")
				return buf.Bytes()
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := PerseHandShake(tt.payload)
			if err == nil {
				t.Errorf("PerseHandShake() 应该返回错误，但没有")
			}
		})
	}
}

// TestPerseHandShakeRealPacket 测试真实的握手包数据
func TestPerseHandShakeRealPacket(t *testing.T) {
	// 模拟真实的 Minecraft 1.20.2 握手包
	// Protocol: 764, Address: "localhost", Port: 25565, NextState: 1
	payload := []byte{
		0xFC, 0x05, // 764 (protocol version)
		0x09,                                                       // string length = 9
		'l', 'o', 'c', 'a', 'l', 'h', 'o', 's', 't',                 // "localhost"
		0x63, 0xDD, // 25565 (port, big endian)
		0x01, // nextState = 1 (status)
	}

	handshake, err := PerseHandShake(payload)
	if err != nil {
		t.Fatalf("PerseHandShake() 返回错误: %v", err)
	}

	if handshake.ProtocolVersion != 764 {
		t.Errorf("ProtocolVersion = %d, 期望 764", handshake.ProtocolVersion)
	}
	if handshake.ServerAddress != "localhost" {
		t.Errorf("ServerAddress = %q, 期望 \"localhost\"", handshake.ServerAddress)
	}
	if handshake.ServerPort != 25565 {
		t.Errorf("ServerPort = %d, 期望 25565", handshake.ServerPort)
	}
	if handshake.NextState != 1 {
		t.Errorf("NextState = %d, 期望 1", handshake.NextState)
	}
}

// TestPerseHandShakeWithFMLMarker 测试带有 Forge Mod Loader 标记的地址
func TestPerseHandShakeWithFMLMarker(t *testing.T) {
	// Forge 客户端会在地址后加上 \x00FML\x00 或类似标记
	tests := []struct {
		name          string
		serverAddress string
	}{
		{
			name:          "FML标记",
			serverAddress: "localhost\x00FML\x00",
		},
		{
			name:          "FML2标记",
			serverAddress: "localhost\x00FML2\x00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := buildHandshakePayload(764, tt.serverAddress, 25565, 2)

			handshake, err := PerseHandShake(payload)
			if err != nil {
				t.Fatalf("PerseHandShake() 返回错误: %v", err)
			}

			if handshake.ServerAddress != tt.serverAddress {
				t.Errorf("ServerAddress = %q, 期望 %q", handshake.ServerAddress, tt.serverAddress)
			}
		})
	}
}
