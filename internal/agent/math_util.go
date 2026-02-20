package agent

func lerpAngle(current, target, maxStep float32) float32 {
	if maxStep <= 0 {
		return normalizeAngle(target)
	}
	delta := signedAngleDelta(current, target)
	if delta > maxStep {
		delta = maxStep
	} else if delta < -maxStep {
		delta = -maxStep
	}
	return normalizeAngle(current + delta)
}

func angleDiff(a, b float32) float32 {
	d := signedAngleDelta(a, b)
	if d < 0 {
		return -d
	}
	return d
}

func signedAngleDelta(from, to float32) float32 {
	return normalizeAngle(to - from)
}

func normalizeAngle(v float32) float32 {
	for v <= -180 {
		v += 360
	}
	for v > 180 {
		v -= 360
	}
	return v
}
