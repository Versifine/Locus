package protocol

import (
	"bytes"
	"io"
	"testing"
)

// TestWriteVarint 测试 WriteVarint 函数
func TestWriteVarint(t *testing.T) {
	// 表驱动测试：定义测试用例表
	tests := []struct {
		name     string  // 测试用例名称
		input    int32   // 输入值
		expected []byte  // 期望的字节输出
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
		wantErr  bool  // 是否期望出现错误
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
