package world

import "testing"

func TestVanillaDimensionBounds(t *testing.T) {
	tests := []struct {
		name      string
		dimension string
		wantMinY  int
		wantH     int
		wantOK    bool
	}{
		{"overworld", DimensionOverworld, -64, 384, true},
		{"nether", DimensionNether, 0, 256, true},
		{"end", DimensionEnd, 0, 256, true},
		{"unknown", "minecraft:custom", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := VanillaDimensionBounds(tt.dimension)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if got.MinY != tt.wantMinY || got.Height != tt.wantH {
				t.Fatalf("bounds = %+v, want minY=%d height=%d", got, tt.wantMinY, tt.wantH)
			}
		})
	}
}
