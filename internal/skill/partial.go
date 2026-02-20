package skill

import "github.com/Versifine/locus/internal/physics"

type PartialInput struct {
	Forward  *bool
	Backward *bool
	Left     *bool
	Right    *bool
	Jump     *bool
	Sneak    *bool
	Sprint   *bool
	Attack   *bool
	Use      *bool

	AttackTarget   *int32
	BreakTarget    *physics.BlockPos
	BreakFinished  *bool
	PlaceTarget    *physics.PlaceAction
	InteractTarget *int32
	HotbarSlot     *int8

	Yaw   *float32
	Pitch *float32

	channels []Channel
}

func (p PartialInput) Channels() []Channel {
	if len(p.channels) == 0 {
		return nil
	}
	out := make([]Channel, len(p.channels))
	copy(out, p.channels)
	return out
}

func (p PartialInput) WithChannels(channels ...Channel) PartialInput {
	p.channels = append([]Channel(nil), channels...)
	return p
}
