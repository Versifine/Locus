package skill

import (
	"math"
	"testing"

	"github.com/Versifine/locus/internal/world"
)

func TestCalcLookAt(t *testing.T) {
	self := world.Position{X: 0, Y: 64, Z: 0}
	target := Vec3{X: 0, Y: 64, Z: 10}

	yaw, pitch := CalcLookAt(self, target)
	if math.Abs(float64(yaw)) > 0.001 {
		t.Fatalf("yaw = %.3f, want 0", yaw)
	}
	if math.Abs(float64(pitch)) > 0.001 {
		t.Fatalf("pitch = %.3f, want 0", pitch)
	}
}

func TestIsNearBoundary(t *testing.T) {
	self := world.Position{X: 0, Y: 0, Z: 0}
	target := Vec3{X: 3, Y: 4, Z: 0}
	if !IsNear(self, target, 5) {
		t.Fatal("expected IsNear true on boundary")
	}
	if IsNear(self, target, 4.99) {
		t.Fatal("expected IsNear false below boundary")
	}
}

func TestFindEntity(t *testing.T) {
	snap := world.Snapshot{Entities: []world.Entity{{EntityID: 1}, {EntityID: 2, X: 3}}}
	e := FindEntity(snap, 2)
	if e == nil || e.EntityID != 2 || e.X != 3 {
		t.Fatalf("unexpected entity: %+v", e)
	}
	if FindEntity(snap, 99) != nil {
		t.Fatal("expected nil for missing entity")
	}
}

func TestAngleDiffWrap(t *testing.T) {
	if got := AngleDiff(179, -179); math.Abs(float64(got-2)) > 0.001 {
		t.Fatalf("AngleDiff(179,-179)=%.3f, want 2", got)
	}
	if got := AngleDiff(-179, 179); math.Abs(float64(got+2)) > 0.001 {
		t.Fatalf("AngleDiff(-179,179)=%.3f, want -2", got)
	}
}
