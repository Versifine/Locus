package protocol

import (
	"errors"
	"testing"
)

// TestErrorMessages 测试错误消息内容
func TestErrorMessages(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"ErrVarIntTooLong", ErrVarIntTooLong, "varint is too long"},
		{"ErrVarLongTooLong", ErrVarLongTooLong, "varlong is too long"},
		{"ErrPacketTooLarge", ErrPacketTooLarge, "packet size exceeds maximum allowed"},
		{"ErrInvalidPacket", ErrInvalidPacket, "invalid packet structure"},
		{"ErrInvalidNBTType", ErrInvalidNBTType, "invalid NBT type"},
		{"ErrMissingField", ErrMissingField, "missing required field in NBT compound"},
		{"ErrInvalidFieldType", ErrInvalidFieldType, "invalid field type in NBT compound"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("%s.Error() = %q, 期望 %q", tt.name, tt.err.Error(), tt.expected)
			}
		})
	}
}

// TestErrorsAreDistinct 测试每个错误都是独立的
func TestErrorsAreDistinct(t *testing.T) {
	allErrors := []error{
		ErrVarIntTooLong,
		ErrVarLongTooLong,
		ErrPacketTooLarge,
		ErrInvalidPacket,
		ErrInvalidNBTType,
		ErrMissingField,
		ErrInvalidFieldType,
	}

	for i := 0; i < len(allErrors); i++ {
		for j := i + 1; j < len(allErrors); j++ {
			if errors.Is(allErrors[i], allErrors[j]) {
				t.Errorf("错误 %q 和 %q 不应相同", allErrors[i], allErrors[j])
			}
		}
	}
}

// TestErrorsIsComparison 测试 errors.Is 可以正确匹配
func TestErrorsIsComparison(t *testing.T) {
	tests := []error{
		ErrVarIntTooLong,
		ErrVarLongTooLong,
		ErrPacketTooLarge,
		ErrInvalidPacket,
		ErrInvalidNBTType,
		ErrMissingField,
		ErrInvalidFieldType,
	}

	for _, err := range tests {
		if !errors.Is(err, err) {
			t.Errorf("errors.Is(%v, %v) 应为 true", err, err)
		}
	}
}
