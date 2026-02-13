package world

import "testing"

func TestItemName(t *testing.T) {
	tests := []struct {
		name   string
		itemID int32
		want   string
	}{
		{name: "egg", itemID: 1031, want: "Egg"},
		{name: "air", itemID: 0, want: "Air"},
		{name: "out of range", itemID: 9999, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ItemName(tt.itemID)
			if got != tt.want {
				t.Fatalf("ItemName(%d) = %q, want %q", tt.itemID, got, tt.want)
			}
		})
	}
}
