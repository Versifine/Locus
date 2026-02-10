package protocol

import (
	"bytes"
	"testing"
)

func TestUpdateHealth(t *testing.T) {
	health := float32(20.0)
	food := int32(20)
	saturation := float32(5.0)

	buf := new(bytes.Buffer)
	_ = WriteFloat(buf, health)
	_ = WriteVarint(buf, food)
	_ = WriteFloat(buf, saturation)

	parsed, err := ParseUpdateHealth(buf)
	if err != nil {
		t.Fatalf("ParseUpdateHealth failed: %v", err)
	}

	if parsed.Health != health || parsed.Food != food || parsed.FoodSaturation != saturation {
		t.Errorf("UpdateHealth mismatch")
	}
}

func TestUpdateTime(t *testing.T) {
	age := int64(1000)
	worldTime := int64(24000)
	tickDayTime := true

	buf := new(bytes.Buffer)
	_ = WriteInt64(buf, age)
	_ = WriteInt64(buf, worldTime)
	_ = WriteBool(buf, tickDayTime)

	parsed, err := ParseUpdateTime(buf)
	if err != nil {
		t.Fatalf("ParseUpdateTime failed: %v", err)
	}

	if parsed.Age != age || parsed.WorldTime != worldTime || parsed.TickDayTime != tickDayTime {
		t.Errorf("UpdateTime mismatch")
	}
}
