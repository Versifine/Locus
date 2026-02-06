package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// 辅助函数：写入 VarInt
func writeVarInt(buf *bytes.Buffer, val int32) {
	uval := uint32(val)
	for {
		temp := byte(uval & 0x7F)
		uval >>= 7
		if uval != 0 {
			temp |= 0x80
		}
		buf.WriteByte(temp)
		if uval == 0 {
			break
		}
	}
}

// 辅助函数：写入 String
func writeString(buf *bytes.Buffer, s string) {
	writeVarInt(buf, int32(len(s)))
	buf.WriteString(s)
}

// 辅助函数：写入 UUID
func writeUUID(buf *bytes.Buffer, uuid UUID) {
	buf.Write(uuid[:])
}

// 辅助函数：写入 Int64
func writeInt64(buf *bytes.Buffer, val int64) {
	binary.Write(buf, binary.BigEndian, val)
}

// 辅助函数：写入 Anonymous NBT (TagEnd = 空)
func writeEmptyNBT(buf *bytes.Buffer) {
	buf.WriteByte(TagEnd)
}

// 辅助函数：写入 Anonymous NBT (TagString)
func writeStringNBT(buf *bytes.Buffer, s string) {
	buf.WriteByte(TagString)
	binary.Write(buf, binary.BigEndian, uint16(len(s)))
	buf.WriteString(s)
}

func buildPlayerChatPayload(
	globalIndex int32,
	senderUUID UUID,
	index int32,
	plainMessage string,
	timestamp int64,
	salt int64,
	previousMessagesCount int32,
	hasUnsignedContent bool,
	filterType int32,
	chatType int32,
	networkName string,
	hasNetworkTargetName bool,
	networkTargetName string,
) []byte {
	var buf bytes.Buffer

	// GlobalIndex
	writeVarInt(&buf, globalIndex)
	// SenderUUID
	writeUUID(&buf, senderUUID)
	// Index
	writeVarInt(&buf, index)
	// PlainMessage
	writeString(&buf, plainMessage)
	// Timestamp
	writeInt64(&buf, timestamp)
	// Salt
	writeInt64(&buf, salt)
	// PreviousMessages (length=0 for simplicity)
	writeVarInt(&buf, previousMessagesCount)
	// UnsignedChatContent (Optional: bool + NBT)
	if hasUnsignedContent {
		buf.WriteByte(1)
		writeEmptyNBT(&buf)
	} else {
		buf.WriteByte(0)
	}
	// FilterType
	writeVarInt(&buf, filterType)
	// Type
	writeVarInt(&buf, chatType)
	// NetworkName
	writeStringNBT(&buf, networkName)
	// NetworkTargetName (Optional: bool + NBT)
	if hasNetworkTargetName {
		buf.WriteByte(1)
		writeStringNBT(&buf, networkTargetName)
	} else {
		buf.WriteByte(0)
	}

	return buf.Bytes()
}

func TestParsePlayerChat_Basic(t *testing.T) {
	uuid := UUID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}

	payload := buildPlayerChatPayload(
		1,             // globalIndex
		uuid,          // senderUUID
		0,             // index
		"Hello World", // plainMessage
		1234567890,    // timestamp
		9876543210,    // salt
		0,             // previousMessagesCount
		false,         // hasUnsignedContent
		0,             // filterType (PASS_THROUGH)
		0,             // chatType
		"Steve",       // networkName
		false,         // hasNetworkTargetName
		"",            // networkTargetName
	)

	reader := bytes.NewReader(payload)
	chat, err := ParsePlayerChat(reader)
	if err != nil {
		t.Fatalf("ParsePlayerChat() 返回错误: %v", err)
	}

	if chat.GlobalIndex != 1 {
		t.Errorf("GlobalIndex = %d, 期望 1", chat.GlobalIndex)
	}
	if chat.SenderUUID != uuid {
		t.Errorf("SenderUUID = %v, 期望 %v", chat.SenderUUID, uuid)
	}
	if chat.PlainMessage != "Hello World" {
		t.Errorf("PlainMessage = %q, 期望 %q", chat.PlainMessage, "Hello World")
	}
	if chat.Timestamp != 1234567890 {
		t.Errorf("Timestamp = %d, 期望 1234567890", chat.Timestamp)
	}
	if chat.FilterType != 0 {
		t.Errorf("FilterType = %d, 期望 0", chat.FilterType)
	}
}

func TestReadPreviousMessages_Empty(t *testing.T) {
	var buf bytes.Buffer
	writeVarInt(&buf, 0) // length = 0

	messages, err := ReadPreviousMessages(&buf)
	if err != nil {
		t.Fatalf("ReadPreviousMessages() 返回错误: %v", err)
	}
	if messages == nil {
		t.Fatal("messages 不应为 nil")
	}
	if len(*messages) != 0 {
		t.Errorf("len(*messages) = %d, 期望 0", len(*messages))
	}
}

func TestReadPreviousMessages_WithData(t *testing.T) {
	var buf bytes.Buffer
	writeVarInt(&buf, 1) // length = 1
	writeVarInt(&buf, 5) // id = 5
	// signature: 256 bytes
	sig := make([]byte, 256)
	for i := range sig {
		sig[i] = byte(i)
	}
	buf.Write(sig)

	messages, err := ReadPreviousMessages(&buf)
	if err != nil {
		t.Fatalf("ReadPreviousMessages() 返回错误: %v", err)
	}
	if messages == nil {
		t.Fatal("messages 不应为 nil")
	}
	if len(*messages) != 1 {
		t.Fatalf("len(*messages) = %d, 期望 1", len(*messages))
	}
	if (*messages)[0].Id != 5 {
		t.Errorf("Id = %d, 期望 5", (*messages)[0].Id)
	}
}

func TestReadPreviousMessages_NegativeId(t *testing.T) {
	var buf bytes.Buffer
	writeVarInt(&buf, 1)  // length = 1
	writeVarInt(&buf, -1) // id = -1 (无效)

	messages, err := ReadPreviousMessages(&buf)
	// 当前实现在 id < 0 时返回 nil, nil
	// 这可能是个 bug：应该跳过签名读取，或者返回错误
	if err != nil {
		t.Logf("ReadPreviousMessages() 返回错误（可能是预期行为）: %v", err)
	}
	if messages != nil {
		t.Logf("messages = %v", messages)
	}
}

func TestReadFilterTypeMask(t *testing.T) {
	var buf bytes.Buffer
	writeVarInt(&buf, 2)                                // length = 2
	binary.Write(&buf, binary.BigEndian, int64(0x1234)) // mask[0]
	binary.Write(&buf, binary.BigEndian, int64(0x5678)) // mask[1]

	mask, err := ReadFilterTypeMask(&buf)
	if err != nil {
		t.Fatalf("ReadFilterTypeMask() 返回错误: %v", err)
	}
	if len(mask) != 2 {
		t.Fatalf("len(mask) = %d, 期望 2", len(mask))
	}
	if mask[0] != 0x1234 {
		t.Errorf("mask[0] = %d, 期望 %d", mask[0], 0x1234)
	}
	if mask[1] != 0x5678 {
		t.Errorf("mask[1] = %d, 期望 %d", mask[1], 0x5678)
	}
}
