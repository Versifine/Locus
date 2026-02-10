package protocol

import (
	"bytes"
	"testing"
)

func TestExperience(t *testing.T) {
	expected := &Experience{
		ExperienceBar:   0.5,
		Level:           10,
		TotalExperience: 100,
	}

	buf := new(bytes.Buffer)
	_ = WriteFloat(buf, expected.ExperienceBar)
	_ = WriteVarint(buf, expected.Level)
	_ = WriteVarint(buf, expected.TotalExperience)

	parsed, err := ParseExperience(buf)
	if err != nil {
		t.Fatalf("ParseExperience failed: %v", err)
	}

	if parsed.ExperienceBar != expected.ExperienceBar {
		t.Errorf("ExperienceBar mismatch: expected %f, got %f", expected.ExperienceBar, parsed.ExperienceBar)
	}
	if parsed.Level != expected.Level {
		t.Errorf("Level mismatch: expected %d, got %d", expected.Level, parsed.Level)
	}
	if parsed.TotalExperience != expected.TotalExperience {
		t.Errorf("TotalExperience mismatch: expected %d, got %d", expected.TotalExperience, parsed.TotalExperience)
	}
}
