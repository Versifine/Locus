package protocol

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

// ========== 基础类型读取测试 ==========

func TestReadByte(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected byte
		wantErr  bool
	}{
		{"零值", []byte{0x00}, 0, false},
		{"最大值", []byte{0xFF}, 255, false},
		{"普通值", []byte{0x42}, 0x42, false},
		{"空输入", []byte{}, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadByte(bytes.NewReader(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Error("应该返回错误")
				}
				return
			}
			if err != nil {
				t.Fatalf("返回错误: %v", err)
			}
			if got != tt.expected {
				t.Errorf("got %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestReadInt16(t *testing.T) {
	tests := []struct {
		name    string
		value   int16
		wantErr bool
	}{
		{"零值", 0, false},
		{"正数", 1234, false},
		{"负数", -1234, false},
		{"最大值", math.MaxInt16, false},
		{"最小值", math.MinInt16, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf [2]byte
			binary.BigEndian.PutUint16(buf[:], uint16(tt.value))
			got, err := NBTReadInt16(bytes.NewReader(buf[:]))

			if err != nil {
				t.Fatalf("返回错误: %v", err)
			}
			if got != tt.value {
				t.Errorf("got %d, want %d", got, tt.value)
			}
		})
	}

	t.Run("空输入", func(t *testing.T) {
		_, err := NBTReadInt16(bytes.NewReader([]byte{}))
		if err == nil {
			t.Error("应该返回错误")
		}
	})

	t.Run("字节不足", func(t *testing.T) {
		_, err := NBTReadInt16(bytes.NewReader([]byte{0x01}))
		if err == nil {
			t.Error("应该返回错误")
		}
	})
}

func TestReadInt32(t *testing.T) {
	tests := []struct {
		name  string
		value int32
	}{
		{"零值", 0},
		{"正数", 123456},
		{"负数", -123456},
		{"最大值", math.MaxInt32},
		{"最小值", math.MinInt32},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf [4]byte
			binary.BigEndian.PutUint32(buf[:], uint32(tt.value))
			got, err := ReadInt32(bytes.NewReader(buf[:]))
			if err != nil {
				t.Fatalf("返回错误: %v", err)
			}
			if got != tt.value {
				t.Errorf("got %d, want %d", got, tt.value)
			}
		})
	}

	t.Run("空输入", func(t *testing.T) {
		_, err := ReadInt32(bytes.NewReader([]byte{}))
		if err == nil {
			t.Error("应该返回错误")
		}
	})
}

func TestReadInt64(t *testing.T) {
	tests := []struct {
		name  string
		value int64
	}{
		{"零值", 0},
		{"正数", 1234567890123},
		{"负数", -1234567890123},
		{"最大值", math.MaxInt64},
		{"最小值", math.MinInt64},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf [8]byte
			binary.BigEndian.PutUint64(buf[:], uint64(tt.value))
			got, err := ReadInt64(bytes.NewReader(buf[:]))
			if err != nil {
				t.Fatalf("返回错误: %v", err)
			}
			if got != tt.value {
				t.Errorf("got %d, want %d", got, tt.value)
			}
		})
	}

	t.Run("空输入", func(t *testing.T) {
		_, err := ReadInt64(bytes.NewReader([]byte{}))
		if err == nil {
			t.Error("应该返回错误")
		}
	})
}

func TestReadFloat32(t *testing.T) {
	tests := []struct {
		name  string
		value float32
	}{
		{"零值", 0},
		{"正数", 3.14},
		{"负数", -2.718},
		{"最大值", math.MaxFloat32},
		{"最小正数", math.SmallestNonzeroFloat32},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf [4]byte
			binary.BigEndian.PutUint32(buf[:], math.Float32bits(tt.value))
			got, err := NBTReadFloat32(bytes.NewReader(buf[:]))
			if err != nil {
				t.Fatalf("返回错误: %v", err)
			}
			if got != tt.value {
				t.Errorf("got %f, want %f", got, tt.value)
			}
		})
	}

	t.Run("空输入", func(t *testing.T) {
		_, err := NBTReadFloat32(bytes.NewReader([]byte{}))
		if err == nil {
			t.Error("应该返回错误")
		}
	})
}

func TestReadFloat64(t *testing.T) {
	tests := []struct {
		name  string
		value float64
	}{
		{"零值", 0},
		{"正数", 3.141592653589793},
		{"负数", -2.718281828459045},
		{"最大值", math.MaxFloat64},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf [8]byte
			binary.BigEndian.PutUint64(buf[:], math.Float64bits(tt.value))
			got, err := NBTReadFloat64(bytes.NewReader(buf[:]))
			if err != nil {
				t.Fatalf("返回错误: %v", err)
			}
			if got != tt.value {
				t.Errorf("got %f, want %f", got, tt.value)
			}
		})
	}

	t.Run("空输入", func(t *testing.T) {
		_, err := NBTReadFloat64(bytes.NewReader([]byte{}))
		if err == nil {
			t.Error("应该返回错误")
		}
	})
}

// ========== NBT 类型读取测试 ==========

func TestReadNBTString(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
		wantErr  bool
	}{
		{
			"空字符串",
			[]byte{0x00, 0x00},
			"",
			false,
		},
		{
			"ASCII字符串",
			append([]byte{0x00, 0x05}, []byte("hello")...),
			"hello",
			false,
		},
		{
			"中文字符串",
			append([]byte{0x00, 0x06}, []byte("你好")...),
			"你好",
			false,
		},
		{
			"空输入",
			[]byte{},
			"",
			true,
		},
		{
			"长度不足",
			[]byte{0x00, 0x0A, 'h', 'i'},
			"",
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NBTReadString(bytes.NewReader(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Error("应该返回错误")
				}
				return
			}
			if err != nil {
				t.Fatalf("返回错误: %v", err)
			}
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestReadNBTByteArray(t *testing.T) {
	t.Run("正常读取", func(t *testing.T) {
		var buf bytes.Buffer
		binary.Write(&buf, binary.BigEndian, int32(4))
		buf.Write([]byte{0x01, 0x02, 0x03, 0x04})

		got, err := NBTReadByteArray(&buf)
		if err != nil {
			t.Fatalf("返回错误: %v", err)
		}
		expected := []byte{0x01, 0x02, 0x03, 0x04}
		if !bytes.Equal(got, expected) {
			t.Errorf("got %v, want %v", got, expected)
		}
	})

	t.Run("空数组", func(t *testing.T) {
		var buf bytes.Buffer
		binary.Write(&buf, binary.BigEndian, int32(0))

		got, err := NBTReadByteArray(&buf)
		if err != nil {
			t.Fatalf("返回错误: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("期望空数组, got len=%d", len(got))
		}
	})

	t.Run("数据不足", func(t *testing.T) {
		var buf bytes.Buffer
		binary.Write(&buf, binary.BigEndian, int32(10))
		buf.Write([]byte{0x01, 0x02})

		_, err := NBTReadByteArray(&buf)
		if err == nil {
			t.Error("应该返回错误")
		}
	})

	t.Run("空输入", func(t *testing.T) {
		_, err := NBTReadByteArray(bytes.NewReader([]byte{}))
		if err == nil {
			t.Error("应该返回错误")
		}
	})
}

func TestReadNBTIntArray(t *testing.T) {
	t.Run("正常读取", func(t *testing.T) {
		var buf bytes.Buffer
		binary.Write(&buf, binary.BigEndian, int32(3))
		binary.Write(&buf, binary.BigEndian, int32(100))
		binary.Write(&buf, binary.BigEndian, int32(-200))
		binary.Write(&buf, binary.BigEndian, int32(300))

		got, err := NBTReadIntArray(&buf)
		if err != nil {
			t.Fatalf("返回错误: %v", err)
		}
		expected := []int32{100, -200, 300}
		for i, v := range expected {
			if got[i] != v {
				t.Errorf("got[%d] = %d, want %d", i, got[i], v)
			}
		}
	})

	t.Run("空数组", func(t *testing.T) {
		var buf bytes.Buffer
		binary.Write(&buf, binary.BigEndian, int32(0))

		got, err := NBTReadIntArray(&buf)
		if err != nil {
			t.Fatalf("返回错误: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("期望空数组, got len=%d", len(got))
		}
	})

	t.Run("数据不足", func(t *testing.T) {
		var buf bytes.Buffer
		binary.Write(&buf, binary.BigEndian, int32(3))
		binary.Write(&buf, binary.BigEndian, int32(100))

		_, err := NBTReadIntArray(&buf)
		if err == nil {
			t.Error("应该返回错误")
		}
	})
}

func TestReadNBTLongArray(t *testing.T) {
	t.Run("正常读取", func(t *testing.T) {
		var buf bytes.Buffer
		binary.Write(&buf, binary.BigEndian, int32(2))
		binary.Write(&buf, binary.BigEndian, int64(1234567890123))
		binary.Write(&buf, binary.BigEndian, int64(-9876543210))

		got, err := NBTReadLongArray(&buf)
		if err != nil {
			t.Fatalf("返回错误: %v", err)
		}
		expected := []int64{1234567890123, -9876543210}
		for i, v := range expected {
			if got[i] != v {
				t.Errorf("got[%d] = %d, want %d", i, got[i], v)
			}
		}
	})

	t.Run("空数组", func(t *testing.T) {
		var buf bytes.Buffer
		binary.Write(&buf, binary.BigEndian, int32(0))

		got, err := NBTReadLongArray(&buf)
		if err != nil {
			t.Fatalf("返回错误: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("期望空数组, got len=%d", len(got))
		}
	})
}

// ========== NBT List 测试 ==========

func TestReadNBTList(t *testing.T) {
	t.Run("Int列表", func(t *testing.T) {
		var buf bytes.Buffer
		buf.WriteByte(TagInt) // 元素类型
		binary.Write(&buf, binary.BigEndian, int32(3))
		binary.Write(&buf, binary.BigEndian, int32(10))
		binary.Write(&buf, binary.BigEndian, int32(20))
		binary.Write(&buf, binary.BigEndian, int32(30))

		got, err := NBTReadList(&buf)
		if err != nil {
			t.Fatalf("返回错误: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("len = %d, want 3", len(got))
		}
		for i, expected := range []int32{10, 20, 30} {
			if got[i].Type != TagInt {
				t.Errorf("got[%d].Type = %d, want TagInt", i, got[i].Type)
			}
			if got[i].Value.(int32) != expected {
				t.Errorf("got[%d].Value = %d, want %d", i, got[i].Value.(int32), expected)
			}
		}
	})

	t.Run("空列表", func(t *testing.T) {
		var buf bytes.Buffer
		buf.WriteByte(TagByte) // 元素类型
		binary.Write(&buf, binary.BigEndian, int32(0))

		got, err := NBTReadList(&buf)
		if err != nil {
			t.Fatalf("返回错误: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("len = %d, want 0", len(got))
		}
	})

	t.Run("String列表", func(t *testing.T) {
		var buf bytes.Buffer
		buf.WriteByte(TagString)
		binary.Write(&buf, binary.BigEndian, int32(2))
		// "ab"
		binary.Write(&buf, binary.BigEndian, uint16(2))
		buf.WriteString("ab")
		// "cd"
		binary.Write(&buf, binary.BigEndian, uint16(2))
		buf.WriteString("cd")

		got, err := NBTReadList(&buf)
		if err != nil {
			t.Fatalf("返回错误: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("len = %d, want 2", len(got))
		}
		if got[0].Value.(string) != "ab" {
			t.Errorf("got[0] = %q, want \"ab\"", got[0].Value.(string))
		}
		if got[1].Value.(string) != "cd" {
			t.Errorf("got[1] = %q, want \"cd\"", got[1].Value.(string))
		}
	})

	t.Run("空输入", func(t *testing.T) {
		_, err := NBTReadList(bytes.NewReader([]byte{}))
		if err == nil {
			t.Error("应该返回错误")
		}
	})
}

// ========== NBT Compound 测试 ==========

func TestReadNBTCompound(t *testing.T) {
	t.Run("单字段Compound", func(t *testing.T) {
		var buf bytes.Buffer
		// field: TagInt, name="score", value=42
		buf.WriteByte(TagInt)
		binary.Write(&buf, binary.BigEndian, uint16(5))
		buf.WriteString("score")
		binary.Write(&buf, binary.BigEndian, int32(42))
		// TagEnd
		buf.WriteByte(TagEnd)

		got, err := NBTReadCompound(&buf)
		if err != nil {
			t.Fatalf("返回错误: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("len = %d, want 1", len(got))
		}
		node, ok := got["score"]
		if !ok {
			t.Fatal("缺少 'score' 字段")
		}
		if node.Type != TagInt || node.Value.(int32) != 42 {
			t.Errorf("score = %v, want Int(42)", node)
		}
	})

	t.Run("多字段Compound", func(t *testing.T) {
		var buf bytes.Buffer
		// TagString "name" = "Steve"
		buf.WriteByte(TagString)
		binary.Write(&buf, binary.BigEndian, uint16(4))
		buf.WriteString("name")
		binary.Write(&buf, binary.BigEndian, uint16(5))
		buf.WriteString("Steve")
		// TagByte "alive" = 1
		buf.WriteByte(TagByte)
		binary.Write(&buf, binary.BigEndian, uint16(5))
		buf.WriteString("alive")
		buf.WriteByte(1)
		// TagEnd
		buf.WriteByte(TagEnd)

		got, err := NBTReadCompound(&buf)
		if err != nil {
			t.Fatalf("返回错误: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("len = %d, want 2", len(got))
		}
		if got["name"].Value.(string) != "Steve" {
			t.Errorf("name = %q, want \"Steve\"", got["name"].Value.(string))
		}
		if got["alive"].Value.(byte) != 1 {
			t.Errorf("alive = %d, want 1", got["alive"].Value.(byte))
		}
	})

	t.Run("空Compound", func(t *testing.T) {
		var buf bytes.Buffer
		buf.WriteByte(TagEnd)

		got, err := NBTReadCompound(&buf)
		if err != nil {
			t.Fatalf("返回错误: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("len = %d, want 0", len(got))
		}
	})

	t.Run("嵌套Compound", func(t *testing.T) {
		var buf bytes.Buffer
		// 外层: TagCompound "inner"
		buf.WriteByte(TagCompound)
		binary.Write(&buf, binary.BigEndian, uint16(5))
		buf.WriteString("inner")
		// 内层: TagInt "val" = 99
		buf.WriteByte(TagInt)
		binary.Write(&buf, binary.BigEndian, uint16(3))
		buf.WriteString("val")
		binary.Write(&buf, binary.BigEndian, int32(99))
		buf.WriteByte(TagEnd) // 内层结束
		buf.WriteByte(TagEnd) // 外层结束

		got, err := NBTReadCompound(&buf)
		if err != nil {
			t.Fatalf("返回错误: %v", err)
		}
		inner := got["inner"].Value.(map[string]*NBTNode)
		if inner["val"].Value.(int32) != 99 {
			t.Errorf("inner.val = %d, want 99", inner["val"].Value.(int32))
		}
	})

	t.Run("空输入", func(t *testing.T) {
		_, err := NBTReadCompound(bytes.NewReader([]byte{}))
		if err == nil {
			t.Error("应该返回错误")
		}
	})
}

// ========== ReadAnonymousNBT 测试 ==========

func TestReadAnonymousNBT(t *testing.T) {
	t.Run("TagEnd", func(t *testing.T) {
		got, err := ReadAnonymousNBT(bytes.NewReader([]byte{TagEnd}))
		if err != nil {
			t.Fatalf("返回错误: %v", err)
		}
		if got.Type != TagEnd {
			t.Errorf("Type = %d, want TagEnd", got.Type)
		}
		if got.Value != nil {
			t.Errorf("Value = %v, want nil", got.Value)
		}
	})

	t.Run("TagByte", func(t *testing.T) {
		got, err := ReadAnonymousNBT(bytes.NewReader([]byte{TagByte, 0x2A}))
		if err != nil {
			t.Fatalf("返回错误: %v", err)
		}
		if got.Type != TagByte || got.Value.(byte) != 0x2A {
			t.Errorf("got %v, want Byte(42)", got)
		}
	})

	t.Run("TagInt", func(t *testing.T) {
		var buf bytes.Buffer
		buf.WriteByte(TagInt)
		binary.Write(&buf, binary.BigEndian, int32(12345))

		got, err := ReadAnonymousNBT(&buf)
		if err != nil {
			t.Fatalf("返回错误: %v", err)
		}
		if got.Type != TagInt || got.Value.(int32) != 12345 {
			t.Errorf("got %v, want Int(12345)", got)
		}
	})

	t.Run("TagString", func(t *testing.T) {
		var buf bytes.Buffer
		buf.WriteByte(TagString)
		binary.Write(&buf, binary.BigEndian, uint16(5))
		buf.WriteString("hello")

		got, err := ReadAnonymousNBT(&buf)
		if err != nil {
			t.Fatalf("返回错误: %v", err)
		}
		if got.Type != TagString || got.Value.(string) != "hello" {
			t.Errorf("got %v, want String(hello)", got)
		}
	})

	t.Run("TagCompound完整结构", func(t *testing.T) {
		var buf bytes.Buffer
		buf.WriteByte(TagCompound)
		// TagString "text" = "hi"
		buf.WriteByte(TagString)
		binary.Write(&buf, binary.BigEndian, uint16(4))
		buf.WriteString("text")
		binary.Write(&buf, binary.BigEndian, uint16(2))
		buf.WriteString("hi")
		buf.WriteByte(TagEnd)

		got, err := ReadAnonymousNBT(&buf)
		if err != nil {
			t.Fatalf("返回错误: %v", err)
		}
		if got.Type != TagCompound {
			t.Fatalf("Type = %d, want TagCompound", got.Type)
		}
		compound := got.Value.(map[string]*NBTNode)
		if compound["text"].Value.(string) != "hi" {
			t.Errorf("text = %q, want \"hi\"", compound["text"].Value.(string))
		}
	})

	t.Run("未知Tag类型", func(t *testing.T) {
		_, err := ReadAnonymousNBT(bytes.NewReader([]byte{0xFF}))
		if err == nil {
			t.Error("应该返回错误")
		}
	})

	t.Run("空输入", func(t *testing.T) {
		_, err := ReadAnonymousNBT(bytes.NewReader([]byte{}))
		if err == nil {
			t.Error("应该返回错误")
		}
	})
}

// ========== NBTNode.String() 测试 ==========

func TestNBTNodeString(t *testing.T) {
	tests := []struct {
		name     string
		node     NBTNode
		contains string
	}{
		{"Byte", NBTNode{Type: TagByte, Value: byte(42)}, "Byte(42)"},
		{"Short", NBTNode{Type: TagShort, Value: int16(300)}, "Short(300)"},
		{"Int", NBTNode{Type: TagInt, Value: int32(100000)}, "Int(100000)"},
		{"Long", NBTNode{Type: TagLong, Value: int64(9999999)}, "Long(9999999)"},
		{"Float", NBTNode{Type: TagFloat, Value: float32(1.5)}, "Float(1.5"},
		{"Double", NBTNode{Type: TagDouble, Value: float64(2.5)}, "Double(2.5"},
		{"String", NBTNode{Type: TagString, Value: "test"}, "String(test)"},
		{"ByteArray", NBTNode{Type: TagByteArray, Value: []byte{1, 2}}, "ByteArray("},
		{"IntArray", NBTNode{Type: TagIntArray, Value: []int32{1, 2}}, "IntArray("},
		{"LongArray", NBTNode{Type: TagLongArray, Value: []int64{1}}, "LongArray("},
		{"List", NBTNode{Type: TagList, Value: []*NBTNode{}}, "List("},
		{"Compound", NBTNode{Type: TagCompound, Value: map[string]*NBTNode{}}, "Compound("},
		{"Unknown", NBTNode{Type: 99, Value: nil}, "Unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.node.String()
			if !containsStr(got, tt.contains) {
				t.Errorf("String() = %q, 应该包含 %q", got, tt.contains)
			}
		})
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
