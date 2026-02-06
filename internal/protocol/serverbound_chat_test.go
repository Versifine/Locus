package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func buildChatMessagePayload(message string, timestamp, salt int64, offset int32, checksum byte) []byte {
	var buf bytes.Buffer
	// Message (String)
	writeVarInt(&buf, int32(len(message)))
	buf.WriteString(message)
	// Timestamp (Int64)
	binary.Write(&buf, binary.BigEndian, timestamp)
	// Salt (Int64)
	binary.Write(&buf, binary.BigEndian, salt)
	// Offset (VarInt)
	writeVarInt(&buf, offset)
	// Checksum (Byte)
	buf.WriteByte(checksum)
	return buf.Bytes()
}

func TestParseChatMessage(t *testing.T) {
	tests := []struct {
		name      string
		message   string
		timestamp int64
		salt      int64
		offset    int32
		checksum  byte
	}{
		{
			name:      "普通消息",
			message:   "Hello World",
			timestamp: 1234567890,
			salt:      9876543210,
			offset:    0,
			checksum:  0x42,
		},
		{
			name:      "空消息",
			message:   "",
			timestamp: 0,
			salt:      0,
			offset:    0,
			checksum:  0,
		},
		{
			name:      "中文消息",
			message:   "你好世界",
			timestamp: 1700000000000,
			salt:      123456,
			offset:    5,
			checksum:  0xFF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := buildChatMessagePayload(tt.message, tt.timestamp, tt.salt, tt.offset, tt.checksum)
			reader := bytes.NewReader(payload)

			chat, err := ParseChatMessage(reader)
			if err != nil {
				t.Fatalf("ParseChatMessage() 返回错误: %v", err)
			}

			if chat.ChatMessage != tt.message {
				t.Errorf("ChatMessage = %q, 期望 %q", chat.ChatMessage, tt.message)
			}
			if chat.Timestamp != tt.timestamp {
				t.Errorf("Timestamp = %d, 期望 %d", chat.Timestamp, tt.timestamp)
			}
			if chat.Salt != tt.salt {
				t.Errorf("Salt = %d, 期望 %d", chat.Salt, tt.salt)
			}
			if chat.Offset != tt.offset {
				t.Errorf("Offset = %d, 期望 %d", chat.Offset, tt.offset)
			}
			if chat.Checksum != tt.checksum {
				t.Errorf("Checksum = %d, 期望 %d", chat.Checksum, tt.checksum)
			}
		})
	}
}

func buildChatCommandPayload(command string) []byte {
	var buf bytes.Buffer
	writeVarInt(&buf, int32(len(command)))
	buf.WriteString(command)
	return buf.Bytes()
}

func TestParseChatCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{"简单命令", "help"},
		{"带参数命令", "give Steve diamond 64"},
		{"空命令", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := buildChatCommandPayload(tt.command)
			reader := bytes.NewReader(payload)

			cmd, err := ParseChatCommand(reader)
			if err != nil {
				t.Fatalf("ParseChatCommand() 返回错误: %v", err)
			}

			if cmd.Command != tt.command {
				t.Errorf("Command = %q, 期望 %q", cmd.Command, tt.command)
			}
		})
	}
}

func buildChatCommandSignedPayload(
	command string,
	timestamp, salt int64,
	argSigs []ArgumentSignature,
	messageCount int32,
	checksum byte,
) []byte {
	var buf bytes.Buffer
	// Command
	writeVarInt(&buf, int32(len(command)))
	buf.WriteString(command)
	// Timestamp
	binary.Write(&buf, binary.BigEndian, timestamp)
	// Salt
	binary.Write(&buf, binary.BigEndian, salt)
	// ArgumentSignatures
	writeVarInt(&buf, int32(len(argSigs)))
	for _, sig := range argSigs {
		writeVarInt(&buf, int32(len(sig.Name)))
		buf.WriteString(sig.Name)
		buf.Write(sig.Signature[:])
	}
	// MessageCount
	writeVarInt(&buf, messageCount)
	// Checksum
	buf.WriteByte(checksum)
	return buf.Bytes()
}

func TestParseChatCommandSigned_NoSignatures(t *testing.T) {
	payload := buildChatCommandSignedPayload(
		"msg Steve Hello",
		1234567890,
		9876543210,
		[]ArgumentSignature{}, // 无签名
		0,
		0x42,
	)

	reader := bytes.NewReader(payload)
	cmd, err := ParseChatCommandSigned(reader)
	if err != nil {
		t.Fatalf("ParseChatCommandSigned() 返回错误: %v", err)
	}

	if cmd.Command != "msg Steve Hello" {
		t.Errorf("Command = %q, 期望 %q", cmd.Command, "msg Steve Hello")
	}
	if cmd.Timestamp != 1234567890 {
		t.Errorf("Timestamp = %d, 期望 1234567890", cmd.Timestamp)
	}
	if cmd.Salt != 9876543210 {
		t.Errorf("Salt = %d, 期望 9876543210", cmd.Salt)
	}
	if len(cmd.ArgumentSignatures) != 0 {
		t.Errorf("len(ArgumentSignatures) = %d, 期望 0", len(cmd.ArgumentSignatures))
	}
	if cmd.MessageCount != 0 {
		t.Errorf("MessageCount = %d, 期望 0", cmd.MessageCount)
	}
	if cmd.Checksum != 0x42 {
		t.Errorf("Checksum = %d, 期望 0x42", cmd.Checksum)
	}
}

func TestParseChatCommandSigned_WithSignatures(t *testing.T) {
	var sig [256]byte
	for i := range sig {
		sig[i] = byte(i)
	}

	payload := buildChatCommandSignedPayload(
		"msg Steve Hello",
		1234567890,
		9876543210,
		[]ArgumentSignature{
			{Name: "message", Signature: sig},
		},
		5,
		0xFF,
	)

	reader := bytes.NewReader(payload)
	cmd, err := ParseChatCommandSigned(reader)
	if err != nil {
		t.Fatalf("ParseChatCommandSigned() 返回错误: %v", err)
	}

	if len(cmd.ArgumentSignatures) != 1 {
		t.Fatalf("len(ArgumentSignatures) = %d, 期望 1", len(cmd.ArgumentSignatures))
	}
	if cmd.ArgumentSignatures[0].Name != "message" {
		t.Errorf("ArgumentSignatures[0].Name = %q, 期望 %q", cmd.ArgumentSignatures[0].Name, "message")
	}
	if cmd.ArgumentSignatures[0].Signature != sig {
		t.Error("ArgumentSignatures[0].Signature 不匹配")
	}
	if cmd.MessageCount != 5 {
		t.Errorf("MessageCount = %d, 期望 5", cmd.MessageCount)
	}
}

func TestParseChatMessage_IncompleteData(t *testing.T) {
	// 只有消息，没有后续字段
	var buf bytes.Buffer
	writeVarInt(&buf, 5)
	buf.WriteString("Hello")
	// 缺少 timestamp, salt, offset, checksum

	reader := bytes.NewReader(buf.Bytes())
	_, err := ParseChatMessage(reader)
	if err == nil {
		t.Error("期望返回错误，但没有")
	}
}
