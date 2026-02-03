package protocol

import (
	"bytes"
	"testing"
)

// buildLoginStartPayload 构建 LoginStart 包的字节数据用于测试
func buildLoginStartPayload(username string, uuid UUID) []byte {
	buf := &bytes.Buffer{}
	// 写入用户名 (Varint长度 + 字符串内容)
	WriteVarint(buf, int32(len(username)))
	buf.WriteString(username)
	// 写入 UUID (16字节)
	buf.Write(uuid[:])
	return buf.Bytes()
}

// TestParseLoginStart 测试 LoginStart 包解析
func TestParseLoginStart(t *testing.T) {
	tests := []struct {
		name     string
		username string
		uuid     UUID
	}{
		{
			name:     "标准用户名",
			username: "Steve",
			uuid:     UUID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10},
		},
		{
			name:     "最短用户名",
			username: "A",
			uuid:     UUID{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			name:     "最长用户名-16字符",
			username: "Player1234567890",
			uuid:     UUID{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
		},
		{
			name:     "包含下划线的用户名",
			username: "Player_123",
			uuid:     UUID{0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC, 0xDE, 0xF0, 0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC, 0xDE, 0xF0},
		},
		{
			name:     "全零UUID",
			username: "TestPlayer",
			uuid:     UUID{},
		},
		{
			name:     "空用户名",
			username: "",
			uuid:     UUID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := buildLoginStartPayload(tt.username, tt.uuid)
			reader := bytes.NewReader(payload)

			loginStart, err := ParseLoginStart(reader)
			if err != nil {
				t.Fatalf("ParseLoginStart() 返回错误: %v", err)
			}

			if loginStart.Username != tt.username {
				t.Errorf("Username = %q, 期望 %q", loginStart.Username, tt.username)
			}
			if loginStart.UUID != tt.uuid {
				t.Errorf("UUID = %v, 期望 %v", loginStart.UUID, tt.uuid)
			}
		})
	}
}

// TestParseLoginStartErrors 测试错误情况
func TestParseLoginStartErrors(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
	}{
		{
			name:    "空输入",
			payload: []byte{},
		},
		{
			name:    "只有用户名长度",
			payload: []byte{0x05},
		},
		{
			name: "用户名不完整",
			payload: func() []byte {
				buf := &bytes.Buffer{}
				WriteVarint(buf, 10) // 声明10字节
				buf.WriteString("ABC")
				return buf.Bytes()
			}(),
		},
		{
			name: "缺少UUID",
			payload: func() []byte {
				buf := &bytes.Buffer{}
				WriteVarint(buf, 5)
				buf.WriteString("Steve")
				return buf.Bytes()
			}(),
		},
		{
			name: "UUID不完整",
			payload: func() []byte {
				buf := &bytes.Buffer{}
				WriteVarint(buf, 5)
				buf.WriteString("Steve")
				buf.Write([]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}) // 只有8字节
				return buf.Bytes()
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytes.NewReader(tt.payload)
			_, err := ParseLoginStart(reader)
			if err == nil {
				t.Errorf("ParseLoginStart() 应该返回错误，但没有")
			}
		})
	}
}

// TestParseLoginStartRealPacket 测试真实的 LoginStart 包数据
func TestParseLoginStartRealPacket(t *testing.T) {
	// 模拟真实的 LoginStart 包
	// Username: "Steve", UUID: 全零
	payload := []byte{
		0x05,                    // 用户名长度 = 5
		'S', 't', 'e', 'v', 'e', // "Steve"
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // UUID 前8字节
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // UUID 后8字节
	}

	reader := bytes.NewReader(payload)
	loginStart, err := ParseLoginStart(reader)
	if err != nil {
		t.Fatalf("ParseLoginStart() 返回错误: %v", err)
	}

	if loginStart.Username != "Steve" {
		t.Errorf("Username = %q, 期望 \"Steve\"", loginStart.Username)
	}

	expectedUUID := UUID{}
	if loginStart.UUID != expectedUUID {
		t.Errorf("UUID = %v, 期望 %v", loginStart.UUID, expectedUUID)
	}
}

// TestParseLoginStartWithOfflineUUID 测试离线模式的 UUID
func TestParseLoginStartWithOfflineUUID(t *testing.T) {
	// 离线模式下，UUID 通常是基于用户名生成的
	username := "Notch"
	// 这是一个示例 UUID
	uuid := UUID{0x06, 0x9a, 0x79, 0xf4, 0x44, 0xe9, 0x4b, 0x2c, 0x98, 0x30, 0xa5, 0x75, 0x26, 0x2d, 0x8c, 0x85}

	payload := buildLoginStartPayload(username, uuid)
	reader := bytes.NewReader(payload)

	loginStart, err := ParseLoginStart(reader)
	if err != nil {
		t.Fatalf("ParseLoginStart() 返回错误: %v", err)
	}

	if loginStart.Username != username {
		t.Errorf("Username = %q, 期望 %q", loginStart.Username, username)
	}
	if loginStart.UUID != uuid {
		t.Errorf("UUID = %v, 期望 %v", loginStart.UUID, uuid)
	}
}
