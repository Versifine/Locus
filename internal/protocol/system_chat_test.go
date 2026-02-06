package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// buildSystemChatPayload 构建 SystemChat 包的字节数据
// content 使用 Anonymous NBT (TagString) 格式, isActionBar 为 bool
func buildSystemChatPayload(content string, isActionBar bool) []byte {
	var buf bytes.Buffer
	// Anonymous NBT: TagString type byte + unsigned short length + string bytes
	buf.WriteByte(TagString)
	binary.Write(&buf, binary.BigEndian, uint16(len(content)))
	buf.WriteString(content)
	// Boolean
	if isActionBar {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}
	return buf.Bytes()
}

func TestParseSystemChat(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		isActionBar bool
	}{
		{
			"普通聊天消息",
			"Hello, World!",
			false,
		},
		{
			"ActionBar消息",
			"You are now AFK",
			true,
		},
		{
			"空内容",
			"",
			false,
		},
		{
			"中文消息",
			"你好世界",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := buildSystemChatPayload(tt.content, tt.isActionBar)
			reader := bytes.NewReader(payload)

			chat, err := ParseSystemChat(reader)
			if err != nil {
				t.Fatalf("ParseSystemChat() 返回错误: %v", err)
			}

			if chat.Content.Type != TagString {
				t.Errorf("Content.Type = %d, 期望 TagString(%d)", chat.Content.Type, TagString)
			}
			if chat.Content.Value.(string) != tt.content {
				t.Errorf("Content.Value = %q, 期望 %q", chat.Content.Value.(string), tt.content)
			}
			if chat.IsActionBar != tt.isActionBar {
				t.Errorf("IsActionBar = %v, 期望 %v", chat.IsActionBar, tt.isActionBar)
			}
		})
	}
}

func TestParseSystemChatWithCompoundNBT(t *testing.T) {
	// 模拟更真实的 Text Component: TagCompound { "text": "hello" }
	var buf bytes.Buffer
	buf.WriteByte(TagCompound)
	// TagString "text" = "hello"
	buf.WriteByte(TagString)
	binary.Write(&buf, binary.BigEndian, uint16(4))
	buf.WriteString("text")
	binary.Write(&buf, binary.BigEndian, uint16(5))
	buf.WriteString("hello")
	buf.WriteByte(TagEnd)
	// isActionBar = false
	buf.WriteByte(0)

	chat, err := ParseSystemChat(&buf)
	if err != nil {
		t.Fatalf("ParseSystemChat() 返回错误: %v", err)
	}

	if chat.Content.Type != TagCompound {
		t.Fatalf("Content.Type = %d, 期望 TagCompound(%d)", chat.Content.Type, TagCompound)
	}
	compound := chat.Content.Value.(map[string]*NBTNode)
	textNode, ok := compound["text"]
	if !ok {
		t.Fatal("缺少 'text' 字段")
	}
	if textNode.Value.(string) != "hello" {
		t.Errorf("text = %q, 期望 \"hello\"", textNode.Value.(string))
	}
	if chat.IsActionBar != false {
		t.Errorf("IsActionBar = %v, 期望 false", chat.IsActionBar)
	}
}

func TestParseSystemChatErrors(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
	}{
		{
			"空输入",
			[]byte{},
		},
		{
			"只有NBT没有isActionBar",
			func() []byte {
				var buf bytes.Buffer
				buf.WriteByte(TagString)
				binary.Write(&buf, binary.BigEndian, uint16(2))
				buf.WriteString("hi")
				// 缺少 isActionBar 字节
				return buf.Bytes()
			}(),
		},
		{
			"无效NBT类型",
			[]byte{0xFF},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytes.NewReader(tt.payload)
			_, err := ParseSystemChat(reader)
			if err == nil {
				t.Error("ParseSystemChat() 应该返回错误")
			}
		})
	}
}
