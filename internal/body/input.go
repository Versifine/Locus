package body

import "github.com/Versifine/locus/internal/physics"

// InputState is the single action format shared by skills and debug controls.
// It aliases physics.InputState to avoid field divergence.
type InputState = physics.InputState
