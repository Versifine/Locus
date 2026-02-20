package agent

import "testing"

func TestAngleDiffWrapAround(t *testing.T) {
	if diff := angleDiff(179, -179); diff != 2 {
		t.Fatalf("angleDiff(179,-179)=%.2f want 2", diff)
	}
	if diff := angleDiff(-170, 170); diff != 20 {
		t.Fatalf("angleDiff(-170,170)=%.2f want 20", diff)
	}
}

func TestLerpAngleShortestPath(t *testing.T) {
	step := lerpAngle(170, -170, 15)
	if step != -175 {
		t.Fatalf("first lerp step=%.2f want -175", step)
	}
	next := lerpAngle(step, -170, 15)
	if next != -170 {
		t.Fatalf("second lerp step=%.2f want -170", next)
	}
}

func TestLerpAngleZeroStepJumpsToTarget(t *testing.T) {
	got := lerpAngle(10, 35, 0)
	if got != 35 {
		t.Fatalf("lerpAngle with zero maxStep=%.2f want 35", got)
	}
}
