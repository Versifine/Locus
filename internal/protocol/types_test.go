package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"testing"
)

// TestWriteVarint 测试 WriteVarint 函数
func TestWriteVarint(t *testing.T) {
	// 表驱动测试：定义测试用例表
	tests := []struct {
		name     string // 测试用例名称
		input    int32  // 输入值
		expected []byte // 期望的字节输出
	}{
		{
			name:     "零值",
			input:    0,
			expected: []byte{0x00},
		},
		{
			name:     "小正数",
			input:    1,
			expected: []byte{0x01},
		},
		{
			name:     "127 (单字节最大值)",
			input:    127,
			expected: []byte{0x7F},
		},
		{
			name:     "128 (需要两字节)",
			input:    128,
			expected: []byte{0x80, 0x01},
		},
		{
			name:     "255",
			input:    255,
			expected: []byte{0xFF, 0x01},
		},
		{
			name:     "大数值",
			input:    300,
			expected: []byte{0xAC, 0x02},
		},
		{
			name:     "更大数值",
			input:    2097151,
			expected: []byte{0xFF, 0xFF, 0x7F},
		},
	}

	// 遍历每个测试用例
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建一个字节缓冲区
			buf := &bytes.Buffer{}

			// 调用被测函数
			err := WriteVarint(buf, tt.input)

			// 检查是否有错误
			if err != nil {
				t.Fatalf("WriteVarint() 返回错误: %v", err)
			}

			// 获取实际输出
			got := buf.Bytes()

			// 比较实际输出和期望输出
			if !bytes.Equal(got, tt.expected) {
				t.Errorf("WriteVarint(%d) = %v, 期望 %v", tt.input, got, tt.expected)
			}
		})
	}
}

// TestReadVarint 测试 ReadVarint 函数
func TestReadVarint(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected int32
		wantErr  bool // 是否期望出现错误
	}{
		{
			name:     "零值",
			input:    []byte{0x00},
			expected: 0,
			wantErr:  false,
		},
		{
			name:     "小正数",
			input:    []byte{0x01},
			expected: 1,
			wantErr:  false,
		},
		{
			name:     "127",
			input:    []byte{0x7F},
			expected: 127,
			wantErr:  false,
		},
		{
			name:     "128",
			input:    []byte{0x80, 0x01},
			expected: 128,
			wantErr:  false,
		},
		{
			name:     "255",
			input:    []byte{0xFF, 0x01},
			expected: 255,
			wantErr:  false,
		},
		{
			name:     "300",
			input:    []byte{0xAC, 0x02},
			expected: 300,
			wantErr:  false,
		},
		{
			name:     "空输入-应该报错",
			input:    []byte{},
			expected: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 从字节数组创建 Reader
			reader := bytes.NewReader(tt.input)

			// 调用被测函数
			got, err := ReadVarint(reader)

			// 检查错误情况
			if tt.wantErr {
				if err == nil {
					t.Errorf("ReadVarint() 应该返回错误，但没有")
				}
				return
			}

			if err != nil {
				t.Fatalf("ReadVarint() 返回错误: %v", err)
			}

			// 比较结果
			if got != tt.expected {
				t.Errorf("ReadVarint() = %d, 期望 %d", got, tt.expected)
			}
		})
	}
}

// TestVarintRoundTrip 测试写入和读取的往返一致性
func TestVarintRoundTrip(t *testing.T) {
	testValues := []int32{0, 1, 127, 128, 255, 256, 300, 2097151, 2147483647}

	for _, value := range testValues {
		t.Run(string(rune(value)), func(t *testing.T) {
			buf := &bytes.Buffer{}

			// 写入
			err := WriteVarint(buf, value)
			if err != nil {
				t.Fatalf("WriteVarint(%d) 错误: %v", value, err)
			}

			// 读取
			got, err := ReadVarint(buf)
			if err != nil {
				t.Fatalf("ReadVarint() 错误: %v", err)
			}

			// 验证读取的值是否等于原始值
			if got != value {
				t.Errorf("往返测试失败: 写入 %d, 读取 %d", value, got)
			}
		})
	}
}

// TestWriteVarLong 测试 WriteVarLong 函数
func TestWriteVarLong(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected []byte
	}{
		{
			name:     "零值",
			input:    0,
			expected: []byte{0x00},
		},
		{
			name:     "小正数",
			input:    1,
			expected: []byte{0x01},
		},
		{
			name:     "127",
			input:    127,
			expected: []byte{0x7F},
		},
		{
			name:     "128",
			input:    128,
			expected: []byte{0x80, 0x01},
		},
		{
			name:     "大数值",
			input:    9223372036854775807, // int64 最大值
			expected: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x7F},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			err := WriteVarLong(buf, tt.input)

			if err != nil {
				t.Fatalf("WriteVarLong() 返回错误: %v", err)
			}

			got := buf.Bytes()
			if !bytes.Equal(got, tt.expected) {
				t.Errorf("WriteVarLong(%d) = %v, 期望 %v", tt.input, got, tt.expected)
			}
		})
	}
}

// TestReadVarLong 测试 ReadVarLong 函数
func TestReadVarLong(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected int64
		wantErr  bool
	}{
		{
			name:     "零值",
			input:    []byte{0x00},
			expected: 0,
			wantErr:  false,
		},
		{
			name:     "小正数",
			input:    []byte{0x01},
			expected: 1,
			wantErr:  false,
		},
		{
			name:     "127",
			input:    []byte{0x7F},
			expected: 127,
			wantErr:  false,
		},
		{
			name:     "128",
			input:    []byte{0x80, 0x01},
			expected: 128,
			wantErr:  false,
		},
		{
			name:     "int64最大值",
			input:    []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x7F},
			expected: 9223372036854775807,
			wantErr:  false,
		},
		{
			name:     "空输入-应该报错",
			input:    []byte{},
			expected: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytes.NewReader(tt.input)
			got, err := ReadVarLong(reader)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ReadVarLong() 应该返回错误，但没有")
				}
				return
			}

			if err != nil {
				t.Fatalf("ReadVarLong() 返回错误: %v", err)
			}

			if got != tt.expected {
				t.Errorf("ReadVarLong() = %d, 期望 %d", got, tt.expected)
			}
		})
	}
}

// TestVarLongRoundTrip 测试 VarLong 的往返一致性
func TestVarLongRoundTrip(t *testing.T) {
	testValues := []int64{
		0,
		1,
		127,
		128,
		255,
		256,
		300,
		2097151,
		9223372036854775807, // int64 最大值
	}

	for _, value := range testValues {
		t.Run(string(rune(value)), func(t *testing.T) {
			buf := &bytes.Buffer{}

			err := WriteVarLong(buf, value)
			if err != nil {
				t.Fatalf("WriteVarLong(%d) 错误: %v", value, err)
			}

			got, err := ReadVarLong(buf)
			if err != nil {
				t.Fatalf("ReadVarLong() 错误: %v", err)
			}

			if got != value {
				t.Errorf("往返测试失败: 写入 %d, 读取 %d", value, got)
			}
		})
	}
}

// TestReadVarintEOF 测试读取时遇到 EOF 的情况
func TestReadVarintEOF(t *testing.T) {
	// 创建一个不完整的 varint（有 CONTINUE_BIT 但数据不完整）
	reader := bytes.NewReader([]byte{0x80}) // 有继续位但没有后续字节

	_, err := ReadVarint(reader)
	if err != io.EOF {
		t.Errorf("ReadVarint() 应该返回 EOF，实际返回: %v", err)
	}
}

// TestReadVarLongEOF 测试 VarLong 读取时遇到 EOF 的情况
func TestReadVarLongEOF(t *testing.T) {
	reader := bytes.NewReader([]byte{0x80})

	_, err := ReadVarLong(reader)
	if err != io.EOF {
		t.Errorf("ReadVarLong() 应该返回 EOF，实际返回: %v", err)
	}
}

// TestReadString 测试 ReadString 函数
func TestReadString(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
		wantErr  bool
	}{
		{
			name:     "空字符串",
			input:    []byte{0x00},
			expected: "",
			wantErr:  false,
		},
		{
			name:     "简单ASCII字符串",
			input:    append([]byte{0x05}, []byte("hello")...),
			expected: "hello",
			wantErr:  false,
		},
		{
			name:     "中文字符串",
			input:    append([]byte{0x06}, []byte("你好")...),
			expected: "你好",
			wantErr:  false,
		},
		{
			name:     "长字符串-超过127字节",
			input:    append([]byte{0x80, 0x01}, make([]byte, 128)...), // 128 字节的空内容
			expected: string(make([]byte, 128)),
			wantErr:  false,
		},
		{
			name:     "空输入-应该报错",
			input:    []byte{},
			expected: "",
			wantErr:  true,
		},
		{
			name:     "长度不足-应该报错",
			input:    []byte{0x05, 'h', 'i'}, // 声明5字节但只有2字节
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytes.NewReader(tt.input)
			got, err := ReadString(reader)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ReadString() 应该返回错误，但没有")
				}
				return
			}

			if err != nil {
				t.Fatalf("ReadString() 返回错误: %v", err)
			}

			if got != tt.expected {
				t.Errorf("ReadString() = %q, 期望 %q", got, tt.expected)
			}
		})
	}
}

// TestReadUnsignedShort 测试 ReadUnsignedShort 函数
func TestReadUnsignedShort(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected uint16
		wantErr  bool
	}{
		{
			name:     "零值",
			input:    []byte{0x00, 0x00},
			expected: 0,
			wantErr:  false,
		},
		{
			name:     "小数值",
			input:    []byte{0x00, 0x01},
			expected: 1,
			wantErr:  false,
		},
		{
			name:     "256",
			input:    []byte{0x01, 0x00},
			expected: 256,
			wantErr:  false,
		},
		{
			name:     "最大值",
			input:    []byte{0xFF, 0xFF},
			expected: 65535,
			wantErr:  false,
		},
		{
			name:     "典型端口号25565",
			input:    []byte{0x63, 0xDD},
			expected: 25565,
			wantErr:  false,
		},
		{
			name:     "空输入-应该报错",
			input:    []byte{},
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "只有一个字节-应该报错",
			input:    []byte{0x01},
			expected: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytes.NewReader(tt.input)
			got, err := ReadUnsignedShort(reader)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ReadUnsignedShort() 应该返回错误，但没有")
				}
				return
			}

			if err != nil {
				t.Fatalf("ReadUnsignedShort() 返回错误: %v", err)
			}

			if got != tt.expected {
				t.Errorf("ReadUnsignedShort() = %d, 期望 %d", got, tt.expected)
			}
		})
	}
}

// TestWriteVarintNegative 测试负数编码
func TestWriteVarintNegative(t *testing.T) {
	tests := []struct {
		name     string
		input    int32
		expected []byte
	}{
		{
			name:     "-1",
			input:    -1,
			expected: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x0F},
		},
		{
			name:     "-2",
			input:    -2,
			expected: []byte{0xFE, 0xFF, 0xFF, 0xFF, 0x0F},
		},
		{
			name:     "int32最小值",
			input:    -2147483648,
			expected: []byte{0x80, 0x80, 0x80, 0x80, 0x08},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			err := WriteVarint(buf, tt.input)

			if err != nil {
				t.Fatalf("WriteVarint() 返回错误: %v", err)
			}

			got := buf.Bytes()
			if !bytes.Equal(got, tt.expected) {
				t.Errorf("WriteVarint(%d) = %v, 期望 %v", tt.input, got, tt.expected)
			}
		})
	}
}

// TestReadVarintNegative 测试负数解码
func TestReadVarintNegative(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected int32
	}{
		{
			name:     "-1",
			input:    []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x0F},
			expected: -1,
		},
		{
			name:     "-2",
			input:    []byte{0xFE, 0xFF, 0xFF, 0xFF, 0x0F},
			expected: -2,
		},
		{
			name:     "int32最小值",
			input:    []byte{0x80, 0x80, 0x80, 0x80, 0x08},
			expected: -2147483648,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytes.NewReader(tt.input)
			got, err := ReadVarint(reader)

			if err != nil {
				t.Fatalf("ReadVarint() 返回错误: %v", err)
			}

			if got != tt.expected {
				t.Errorf("ReadVarint() = %d, 期望 %d", got, tt.expected)
			}
		})
	}
}

// TestWriteVarLongNegative 测试 VarLong 负数编码
func TestWriteVarLongNegative(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected []byte
	}{
		{
			name:     "-1",
			input:    -1,
			expected: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x01},
		},
		{
			name:     "-2",
			input:    -2,
			expected: []byte{0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x01},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			err := WriteVarLong(buf, tt.input)

			if err != nil {
				t.Fatalf("WriteVarLong() 返回错误: %v", err)
			}

			got := buf.Bytes()
			if !bytes.Equal(got, tt.expected) {
				t.Errorf("WriteVarLong(%d) = %v, 期望 %v", tt.input, got, tt.expected)
			}
		})
	}
}

// TestReadVarLongNegative 测试 VarLong 负数解码
func TestReadVarLongNegative(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected int64
	}{
		{
			name:     "-1",
			input:    []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x01},
			expected: -1,
		},
		{
			name:     "-2",
			input:    []byte{0xFE, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x01},
			expected: -2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytes.NewReader(tt.input)
			got, err := ReadVarLong(reader)

			if err != nil {
				t.Fatalf("ReadVarLong() 返回错误: %v", err)
			}

			if got != tt.expected {
				t.Errorf("ReadVarLong() = %d, 期望 %d", got, tt.expected)
			}
		})
	}
}

// TestVarintNegativeRoundTrip 测试负数往返一致性
func TestVarintNegativeRoundTrip(t *testing.T) {
	testValues := []int32{-1, -2, -127, -128, -255, -256, -2147483648}

	for _, value := range testValues {
		buf := &bytes.Buffer{}

		err := WriteVarint(buf, value)
		if err != nil {
			t.Fatalf("WriteVarint(%d) 错误: %v", value, err)
		}

		got, err := ReadVarint(buf)
		if err != nil {
			t.Fatalf("ReadVarint() 错误: %v", err)
		}

		if got != value {
			t.Errorf("往返测试失败: 写入 %d, 读取 %d", value, got)
		}
	}
}

// TestVarLongNegativeRoundTrip 测试 VarLong 负数往返一致性
func TestVarLongNegativeRoundTrip(t *testing.T) {
	testValues := []int64{-1, -2, -127, -128, -255, -256, -9223372036854775808}

	for _, value := range testValues {
		buf := &bytes.Buffer{}

		err := WriteVarLong(buf, value)
		if err != nil {
			t.Fatalf("WriteVarLong(%d) 错误: %v", value, err)
		}

		got, err := ReadVarLong(buf)
		if err != nil {
			t.Fatalf("ReadVarLong() 错误: %v", err)
		}

		if got != value {
			t.Errorf("往返测试失败: 写入 %d, 读取 %d", value, got)
		}
	}
}

// TestReadVarintTooLong 测试超出位数限制的错误
func TestReadVarintTooLong(t *testing.T) {
	// 6 个字节都有 CONTINUE_BIT，超过 32 位限制
	input := []byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x01}
	reader := bytes.NewReader(input)

	_, err := ReadVarint(reader)
	if !errors.Is(err, ErrVarIntTooLong) {
		t.Errorf("ReadVarint() 应该返回 ErrVarIntTooLong，实际返回: %v", err)
	}
}

// TestReadVarLongTooLong 测试 VarLong 超出位数限制的错误
func TestReadVarLongTooLong(t *testing.T) {
	// 11 个字节都有 CONTINUE_BIT，超过 64 位限制
	input := []byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x01}
	reader := bytes.NewReader(input)

	_, err := ReadVarLong(reader)
	if !errors.Is(err, ErrVarLongTooLong) {
		t.Errorf("ReadVarLong() 应该返回 ErrVarLongTooLong，实际返回: %v", err)
	}
}

// TestReadUUID 测试 ReadUUID 函数
func TestReadUUID(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected UUID
		wantErr  bool
	}{
		{
			name:     "全零UUID",
			input:    make([]byte, 16),
			expected: UUID{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			wantErr:  false,
		},
		{
			name: "标准UUID",
			input: []byte{
				0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
				0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10,
			},
			expected: UUID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10},
			wantErr:  false,
		},
		{
			name: "全FF的UUID",
			input: []byte{
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
				0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
			},
			expected: UUID{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			wantErr:  false,
		},
		{
			name:     "空输入-应该报错",
			input:    []byte{},
			expected: UUID{},
			wantErr:  true,
		},
		{
			name:     "不足16字节-应该报错",
			input:    []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
			expected: UUID{},
			wantErr:  true,
		},
		{
			name:     "只有1字节-应该报错",
			input:    []byte{0x01},
			expected: UUID{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytes.NewReader(tt.input)
			got, err := ReadUUID(reader)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ReadUUID() 应该返回错误，但没有")
				}
				return
			}

			if err != nil {
				t.Fatalf("ReadUUID() 返回错误: %v", err)
			}

			if got != tt.expected {
				t.Errorf("ReadUUID() = %v, 期望 %v", got, tt.expected)
			}
		})
	}
}

// TestUUIDString 测试 UUID 的字符串格式化
func TestUUIDString(t *testing.T) {
	tests := []struct {
		name     string
		uuid     UUID
		expected string
	}{
		{
			name:     "全零UUID",
			uuid:     UUID{},
			expected: "00000000-0000-0000-0000-000000000000",
		},
		{
			name:     "标准UUID",
			uuid:     UUID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10},
			expected: "01020304-0506-0708-090a-0b0c0d0e0f10",
		},
		{
			name:     "全FF的UUID",
			uuid:     UUID{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			expected: "ffffffff-ffff-ffff-ffff-ffffffffffff",
		},
		{
			name:     "Notch的UUID示例",
			uuid:     UUID{0x06, 0x9a, 0x79, 0xf4, 0x44, 0xe9, 0x4b, 0x2c, 0x98, 0x30, 0xa5, 0x75, 0x26, 0x2d, 0x8c, 0x85},
			expected: "069a79f4-44e9-4b2c-9830-a575262d8c85",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.uuid.String()
			if got != tt.expected {
				t.Errorf("UUID.String() = %q, 期望 %q", got, tt.expected)
			}
		})
	}
}

// TestReadBool 测试 ReadBool 函数
func TestReadBool(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected bool
		wantErr  bool
	}{
		{
			name:     "false (0x00)",
			input:    []byte{0x00},
			expected: false,
			wantErr:  false,
		},
		{
			name:     "true (0x01)",
			input:    []byte{0x01},
			expected: true,
			wantErr:  false,
		},
		{
			name:     "非零值也为true (0xFF)",
			input:    []byte{0xFF},
			expected: true,
			wantErr:  false,
		},
		{
			name:     "非零值也为true (0x42)",
			input:    []byte{0x42},
			expected: true,
			wantErr:  false,
		},
		{
			name:     "空输入-应该报错",
			input:    []byte{},
			expected: false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytes.NewReader(tt.input)
			got, err := ReadBool(reader)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ReadBool() 应该返回错误，但没有")
				}
				return
			}

			if err != nil {
				t.Fatalf("ReadBool() 返回错误: %v", err)
			}

			if got != tt.expected {
				t.Errorf("ReadBool() = %v, 期望 %v", got, tt.expected)
			}
		})
	}
}

// TestReadUUIDWithExtraData 测试读取UUID后是否正确消费了16字节
func TestReadUUIDWithExtraData(t *testing.T) {
	// 16字节UUID + 额外数据
	input := []byte{
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10,
		0xAA, 0xBB, // 额外数据
	}
	reader := bytes.NewReader(input)

	uuid, err := ReadUUID(reader)
	if err != nil {
		t.Fatalf("ReadUUID() 返回错误: %v", err)
	}

	expected := UUID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10}
	if uuid != expected {
		t.Errorf("ReadUUID() = %v, 期望 %v", uuid, expected)
	}

	// 检查剩余数据
	remaining := make([]byte, 2)
	n, _ := reader.Read(remaining)
	if n != 2 || remaining[0] != 0xAA || remaining[1] != 0xBB {
		t.Errorf("ReadUUID() 没有正确消费16字节，剩余数据不对")
	}
}

// ============================================================
// T031: Write 辅助函数 + GenerateOfflineUUID 测试
// ============================================================

// TestWriteUUID 测试 WriteUUID 函数
func TestWriteUUID(t *testing.T) {
	tests := []struct {
		name     string
		uuid     UUID
		expected []byte
	}{
		{
			name:     "全零UUID",
			uuid:     UUID{},
			expected: make([]byte, 16),
		},
		{
			name:     "标准UUID",
			uuid:     UUID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10},
			expected: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10},
		},
		{
			name:     "全FF的UUID",
			uuid:     UUID{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			expected: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			err := WriteUUID(buf, tt.uuid)
			if err != nil {
				t.Fatalf("WriteUUID() 返回错误: %v", err)
			}
			got := buf.Bytes()
			if !bytes.Equal(got, tt.expected) {
				t.Errorf("WriteUUID() = %v, 期望 %v", got, tt.expected)
			}
		})
	}
}

// TestUUIDRoundTrip 测试 UUID 写入→读取往返一致性
func TestUUIDRoundTrip(t *testing.T) {
	testUUIDs := []UUID{
		{},
		{0x06, 0x9a, 0x79, 0xf4, 0x44, 0xe9, 0x4b, 0x2c, 0x98, 0x30, 0xa5, 0x75, 0x26, 0x2d, 0x8c, 0x85},
		{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
	}

	for _, uuid := range testUUIDs {
		t.Run(uuid.String(), func(t *testing.T) {
			buf := &bytes.Buffer{}
			if err := WriteUUID(buf, uuid); err != nil {
				t.Fatalf("WriteUUID() 错误: %v", err)
			}
			got, err := ReadUUID(buf)
			if err != nil {
				t.Fatalf("ReadUUID() 错误: %v", err)
			}
			if got != uuid {
				t.Errorf("往返测试失败: 写入 %v, 读取 %v", uuid, got)
			}
		})
	}
}

// TestWriteUnsignedShort 测试 WriteUnsignedShort 函数
func TestWriteUnsignedShort(t *testing.T) {
	tests := []struct {
		name     string
		input    uint16
		expected []byte
	}{
		{
			name:     "零值",
			input:    0,
			expected: []byte{0x00, 0x00},
		},
		{
			name:     "小数值",
			input:    1,
			expected: []byte{0x00, 0x01},
		},
		{
			name:     "256",
			input:    256,
			expected: []byte{0x01, 0x00},
		},
		{
			name:     "最大值",
			input:    65535,
			expected: []byte{0xFF, 0xFF},
		},
		{
			name:     "典型端口号25565",
			input:    25565,
			expected: []byte{0x63, 0xDD},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			err := WriteUnsignedShort(buf, tt.input)
			if err != nil {
				t.Fatalf("WriteUnsignedShort() 返回错误: %v", err)
			}
			got := buf.Bytes()
			if !bytes.Equal(got, tt.expected) {
				t.Errorf("WriteUnsignedShort(%d) = %v, 期望 %v", tt.input, got, tt.expected)
			}
		})
	}
}

// TestUnsignedShortRoundTrip 测试 UnsignedShort 往返一致性
func TestUnsignedShortRoundTrip(t *testing.T) {
	testValues := []uint16{0, 1, 255, 256, 25565, 65535}

	for _, value := range testValues {
		buf := &bytes.Buffer{}
		if err := WriteUnsignedShort(buf, value); err != nil {
			t.Fatalf("WriteUnsignedShort(%d) 错误: %v", value, err)
		}
		got, err := ReadUnsignedShort(buf)
		if err != nil {
			t.Fatalf("ReadUnsignedShort() 错误: %v", err)
		}
		if got != value {
			t.Errorf("往返测试失败: 写入 %d, 读取 %d", value, got)
		}
	}
}

// TestWriteBool 测试 WriteBool 函数
func TestWriteBool(t *testing.T) {
	tests := []struct {
		name     string
		input    bool
		expected []byte
	}{
		{
			name:     "false",
			input:    false,
			expected: []byte{0x00},
		},
		{
			name:     "true",
			input:    true,
			expected: []byte{0x01},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			err := WriteBool(buf, tt.input)
			if err != nil {
				t.Fatalf("WriteBool() 返回错误: %v", err)
			}
			got := buf.Bytes()
			if !bytes.Equal(got, tt.expected) {
				t.Errorf("WriteBool(%v) = %v, 期望 %v", tt.input, got, tt.expected)
			}
		})
	}
}

// TestBoolRoundTrip 测试 Bool 往返一致性
func TestBoolRoundTrip(t *testing.T) {
	for _, value := range []bool{true, false} {
		buf := &bytes.Buffer{}
		if err := WriteBool(buf, value); err != nil {
			t.Fatalf("WriteBool(%v) 错误: %v", value, err)
		}
		got, err := ReadBool(buf)
		if err != nil {
			t.Fatalf("ReadBool() 错误: %v", err)
		}
		if got != value {
			t.Errorf("往返测试失败: 写入 %v, 读取 %v", value, got)
		}
	}
}

// TestWriteInt64 测试 WriteInt64 函数
func TestWriteInt64(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected []byte
	}{
		{
			name:     "零值",
			input:    0,
			expected: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
		{
			name:     "正数1",
			input:    1,
			expected: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		},
		{
			name:     "负数-1",
			input:    -1,
			expected: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
		},
		{
			name:     "int64最大值",
			input:    9223372036854775807,
			expected: []byte{0x7F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			err := WriteInt64(buf, tt.input)
			if err != nil {
				t.Fatalf("WriteInt64() 返回错误: %v", err)
			}
			got := buf.Bytes()
			if !bytes.Equal(got, tt.expected) {
				t.Errorf("WriteInt64(%d) = %v, 期望 %v", tt.input, got, tt.expected)
			}
		})
	}
}

// TestInt64RoundTrip 测试 Int64 写入→读取往返一致性
func TestInt64RoundTrip(t *testing.T) {
	testValues := []int64{0, 1, -1, 127, -128, 9223372036854775807, -9223372036854775808}

	for _, value := range testValues {
		buf := &bytes.Buffer{}
		if err := WriteInt64(buf, value); err != nil {
			t.Fatalf("WriteInt64(%d) 错误: %v", value, err)
		}
		// 使用 binary.BigEndian 读取来验证（与 nbt.go 中 ReadInt64 一致）
		var raw [8]byte
		copy(raw[:], buf.Bytes())
		got := int64(binary.BigEndian.Uint64(raw[:]))
		if got != value {
			t.Errorf("往返测试失败: 写入 %d, 读取 %d", value, got)
		}
	}
}

// TestWriteFloat 测试 WriteFloat 函数
func TestWriteFloat(t *testing.T) {
	tests := []struct {
		name  string
		input float32
	}{
		{"零值", 0.0},
		{"正数", 3.14},
		{"负数", -1.5},
		{"最大值", math.MaxFloat32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			err := WriteFloat(buf, tt.input)
			if err != nil {
				t.Fatalf("WriteFloat() 返回错误: %v", err)
			}
			got := buf.Bytes()
			if len(got) != 4 {
				t.Fatalf("WriteFloat() 输出长度 = %d, 期望 4", len(got))
			}
			// 验证字节内容
			bits := binary.BigEndian.Uint32(got)
			decoded := math.Float32frombits(bits)
			if decoded != tt.input {
				t.Errorf("WriteFloat(%v) 解码后 = %v", tt.input, decoded)
			}
		})
	}
}

// TestWriteDouble 测试 WriteDouble 函数
func TestWriteDouble(t *testing.T) {
	tests := []struct {
		name  string
		input float64
	}{
		{"零值", 0.0},
		{"正数", 3.141592653589793},
		{"负数", -1.5},
		{"最大值", math.MaxFloat64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			err := WriteDouble(buf, tt.input)
			if err != nil {
				t.Fatalf("WriteDouble() 返回错误: %v", err)
			}
			got := buf.Bytes()
			if len(got) != 8 {
				t.Fatalf("WriteDouble() 输出长度 = %d, 期望 8", len(got))
			}
			bits := binary.BigEndian.Uint64(got)
			decoded := math.Float64frombits(bits)
			if decoded != tt.input {
				t.Errorf("WriteDouble(%v) 解码后 = %v", tt.input, decoded)
			}
		})
	}
}

// TestGenerateOfflineUUID 测试离线模式 UUID 生成
func TestGenerateOfflineUUID(t *testing.T) {
	tests := []struct {
		name     string
		username string
		expected string
	}{
		{
			name:     "Locus",
			username: "Locus",
			expected: "a]replace_with_runtime",
		},
		{
			name:     "Notch",
			username: "Notch",
			expected: "b]replace_with_runtime",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uuid := GenerateOfflineUUID(tt.username)

			// 验证 version = 3（byte 6 的高 4 位为 0011）
			version := (uuid[6] >> 4) & 0x0F
			if version != 3 {
				t.Errorf("GenerateOfflineUUID(%q) version = %d, 期望 3", tt.username, version)
			}

			// 验证 variant = RFC 4122（byte 8 的高 2 位为 10）
			variant := (uuid[8] >> 6) & 0x03
			if variant != 2 { // 0b10 = 2
				t.Errorf("GenerateOfflineUUID(%q) variant = %d, 期望 2 (RFC 4122)", tt.username, variant)
			}
		})
	}
}

// TestGenerateOfflineUUID_Deterministic 测试相同用户名生成相同 UUID
func TestGenerateOfflineUUID_Deterministic(t *testing.T) {
	uuid1 := GenerateOfflineUUID("Locus")
	uuid2 := GenerateOfflineUUID("Locus")
	if uuid1 != uuid2 {
		t.Errorf("相同用户名应生成相同UUID: %v != %v", uuid1, uuid2)
	}
}

// TestGenerateOfflineUUID_DifferentUsers 测试不同用户名生成不同 UUID
func TestGenerateOfflineUUID_DifferentUsers(t *testing.T) {
	uuid1 := GenerateOfflineUUID("Alice")
	uuid2 := GenerateOfflineUUID("Bob")
	if uuid1 == uuid2 {
		t.Errorf("不同用户名不应生成相同UUID: %v == %v", uuid1, uuid2)
	}
}

// TestGenerateOfflineUUID_KnownValue 测试已知的离线UUID值
// "OfflinePlayer:Notch" 的 MD5 → 设置 version=3 和 variant=RFC4122 后的已知结果
func TestGenerateOfflineUUID_KnownValue(t *testing.T) {
	uuid := GenerateOfflineUUID("Notch")
	uuidStr := uuid.String()
	// Notch 的离线 UUID: MD5("OfflinePlayer:Notch") + version=3 + variant=RFC4122
	expected := "b50ad385-829d-3141-a216-7e7d7539ba7f"
	if uuidStr != expected {
		t.Errorf("GenerateOfflineUUID(\"Notch\") = %q, 期望 %q", uuidStr, expected)
	}
}
