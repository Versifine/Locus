package protocol

import (
	"bytes"
	"testing"
)

// buildLoginSuccessPayload 构建 LoginSuccess 包的字节数据用于测试
func buildLoginSuccessPayload(uuid UUID, username string, properties []Property) []byte {
	buf := &bytes.Buffer{}
	// UUID (16 bytes)
	buf.Write(uuid[:])
	// Username (varint length + string)
	WriteVarint(buf, int32(len(username)))
	buf.WriteString(username)
	// Properties length (varint)
	WriteVarint(buf, int32(len(properties)))
	// Properties
	for _, prop := range properties {
		WriteVarint(buf, int32(len(prop.Name)))
		buf.WriteString(prop.Name)
		WriteVarint(buf, int32(len(prop.Value)))
		buf.WriteString(prop.Value)
		if prop.Signature != nil {
			buf.WriteByte(0x01) // hasSignature = true
			WriteVarint(buf, int32(len(*prop.Signature)))
			buf.WriteString(*prop.Signature)
		} else {
			buf.WriteByte(0x00) // hasSignature = false
		}
	}
	return buf.Bytes()
}

// TestParseLoginSuccess 测试 LoginSuccess 包解析
func TestParseLoginSuccess(t *testing.T) {
	sig := "test-signature"
	tests := []struct {
		name       string
		uuid       UUID
		username   string
		properties []Property
	}{
		{
			name:       "无属性",
			uuid:       UUID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10},
			username:   "Steve",
			properties: []Property{},
		},
		{
			name:     "一个属性无签名",
			uuid:     UUID{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			username: "Notch",
			properties: []Property{
				{Name: "textures", Value: "base64data", Signature: nil},
			},
		},
		{
			name:     "一个属性有签名",
			uuid:     UUID{0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC, 0xDE, 0xF0, 0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC, 0xDE, 0xF0},
			username: "Player123",
			properties: []Property{
				{Name: "textures", Value: "base64data", Signature: &sig},
			},
		},
		{
			name:     "多个属性",
			uuid:     UUID{},
			username: "TestPlayer",
			properties: []Property{
				{Name: "textures", Value: "value1", Signature: nil},
				{Name: "other", Value: "value2", Signature: &sig},
			},
		},
		{
			name:       "空用户名",
			uuid:       UUID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10},
			username:   "",
			properties: []Property{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := buildLoginSuccessPayload(tt.uuid, tt.username, tt.properties)
			reader := bytes.NewReader(payload)

			result, err := ParseLoginSuccess(reader)
			if err != nil {
				t.Fatalf("ParseLoginSuccess() 返回错误: %v", err)
			}

			if result.UUID != tt.uuid {
				t.Errorf("UUID = %v, 期望 %v", result.UUID, tt.uuid)
			}
			if result.Username != tt.username {
				t.Errorf("Username = %q, 期望 %q", result.Username, tt.username)
			}
			if result.PropertiesLength != int32(len(tt.properties)) {
				t.Errorf("PropertiesLength = %d, 期望 %d", result.PropertiesLength, len(tt.properties))
			}
			if len(result.Properties) != len(tt.properties) {
				t.Fatalf("Properties 长度 = %d, 期望 %d", len(result.Properties), len(tt.properties))
			}

			for i, prop := range result.Properties {
				if prop.Name != tt.properties[i].Name {
					t.Errorf("Property[%d].Name = %q, 期望 %q", i, prop.Name, tt.properties[i].Name)
				}
				if prop.Value != tt.properties[i].Value {
					t.Errorf("Property[%d].Value = %q, 期望 %q", i, prop.Value, tt.properties[i].Value)
				}
				if tt.properties[i].Signature == nil {
					if prop.Signature != nil {
						t.Errorf("Property[%d].Signature 应为 nil", i)
					}
				} else {
					if prop.Signature == nil {
						t.Errorf("Property[%d].Signature 不应为 nil", i)
					} else if *prop.Signature != *tt.properties[i].Signature {
						t.Errorf("Property[%d].Signature = %q, 期望 %q", i, *prop.Signature, *tt.properties[i].Signature)
					}
				}
			}
		})
	}
}

// TestParseLoginSuccessErrors 测试错误情况
func TestParseLoginSuccessErrors(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
	}{
		{
			name:    "空输入",
			payload: []byte{},
		},
		{
			name:    "UUID不完整",
			payload: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		},
		{
			name: "缺少用户名",
			payload: func() []byte {
				buf := &bytes.Buffer{}
				buf.Write(make([]byte, 16)) // UUID
				return buf.Bytes()
			}(),
		},
		{
			name: "缺少属性长度",
			payload: func() []byte {
				buf := &bytes.Buffer{}
				buf.Write(make([]byte, 16)) // UUID
				WriteVarint(buf, 5)
				buf.WriteString("Steve")
				return buf.Bytes()
			}(),
		},
		{
			name: "属性数据不完整",
			payload: func() []byte {
				buf := &bytes.Buffer{}
				buf.Write(make([]byte, 16)) // UUID
				WriteVarint(buf, 5)
				buf.WriteString("Steve")
				WriteVarint(buf, 1) // 声明1个属性
				// 但没有属性数据
				return buf.Bytes()
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytes.NewReader(tt.payload)
			_, err := ParseLoginSuccess(reader)
			if err == nil {
				t.Errorf("ParseLoginSuccess() 应该返回错误，但没有")
			}
		})
	}
}

// TestReadProperty 测试属性解析
func TestReadProperty(t *testing.T) {
	sig := "my-signature"
	tests := []struct {
		name     string
		expected Property
	}{
		{
			name:     "无签名属性",
			expected: Property{Name: "textures", Value: "base64value", Signature: nil},
		},
		{
			name:     "有签名属性",
			expected: Property{Name: "textures", Value: "base64value", Signature: &sig},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			WriteVarint(buf, int32(len(tt.expected.Name)))
			buf.WriteString(tt.expected.Name)
			WriteVarint(buf, int32(len(tt.expected.Value)))
			buf.WriteString(tt.expected.Value)
			if tt.expected.Signature != nil {
				buf.WriteByte(0x01)
				WriteVarint(buf, int32(len(*tt.expected.Signature)))
				buf.WriteString(*tt.expected.Signature)
			} else {
				buf.WriteByte(0x00)
			}

			reader := bytes.NewReader(buf.Bytes())
			got, err := ReadProperty(reader)
			if err != nil {
				t.Fatalf("ReadProperty() 返回错误: %v", err)
			}

			if got.Name != tt.expected.Name {
				t.Errorf("Name = %q, 期望 %q", got.Name, tt.expected.Name)
			}
			if got.Value != tt.expected.Value {
				t.Errorf("Value = %q, 期望 %q", got.Value, tt.expected.Value)
			}
			if tt.expected.Signature == nil {
				if got.Signature != nil {
					t.Error("Signature 应为 nil")
				}
			} else {
				if got.Signature == nil {
					t.Error("Signature 不应为 nil")
				} else if *got.Signature != *tt.expected.Signature {
					t.Errorf("Signature = %q, 期望 %q", *got.Signature, *tt.expected.Signature)
				}
			}
		})
	}
}

// TestReadPropertyErrors 测试属性解析错误
func TestReadPropertyErrors(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
	}{
		{"空输入", []byte{}},
		{"只有name长度", []byte{0x03}},
		{
			"缺少value",
			func() []byte {
				buf := &bytes.Buffer{}
				WriteVarint(buf, 3)
				buf.WriteString("abc")
				return buf.Bytes()
			}(),
		},
		{
			"缺少hasSignature",
			func() []byte {
				buf := &bytes.Buffer{}
				WriteVarint(buf, 3)
				buf.WriteString("abc")
				WriteVarint(buf, 3)
				buf.WriteString("def")
				return buf.Bytes()
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytes.NewReader(tt.payload)
			_, err := ReadProperty(reader)
			if err == nil {
				t.Error("ReadProperty() 应该返回错误，但没有")
			}
		})
	}
}
