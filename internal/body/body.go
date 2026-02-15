package body

import (
	"fmt"
	"sync"

	"github.com/Versifine/locus/internal/physics"
	"github.com/Versifine/locus/internal/protocol"
	"github.com/Versifine/locus/internal/world"
)

type PacketSender interface {
	SendPacket(packet *protocol.Packet) error
}

type StateUpdater interface {
	UpdatePosition(pos world.Position)
}

type Body struct {
	mu           sync.Mutex
	physics      physics.PhysicsState
	packetSender PacketSender
	blockStore   physics.BlockStore
	stateUpdater StateUpdater
}

func New(
	initial world.Position,
	onGround bool,
	packetSender PacketSender,
	blockStore physics.BlockStore,
	stateUpdater StateUpdater,
) *Body {
	return &Body{
		physics: physics.PhysicsState{
			Position: physics.Vec3{
				X: initial.X,
				Y: initial.Y,
				Z: initial.Z,
			},
			OnGround: onGround,
		},
		packetSender: packetSender,
		blockStore:   blockStore,
		stateUpdater: stateUpdater,
	}
}

func (b *Body) Tick(input InputState) error {
	if b == nil {
		return fmt.Errorf("body is nil")
	}
	if b.packetSender == nil {
		return fmt.Errorf("packet sender is nil")
	}

	b.mu.Lock()
	physics.PhysicsTick(&b.physics, physics.InputState(input), b.blockStore)
	pos := b.physics.Position
	onGround := b.physics.OnGround
	b.mu.Unlock()

	packet := protocol.CreatePlayerPositionAndRotationPacket(
		pos.X,
		pos.Y,
		pos.Z,
		input.Yaw,
		input.Pitch,
		onGround,
	)
	if err := b.packetSender.SendPacket(packet); err != nil {
		return err
	}

	if b.stateUpdater != nil {
		b.stateUpdater.UpdatePosition(world.Position{
			X:     pos.X,
			Y:     pos.Y,
			Z:     pos.Z,
			Yaw:   input.Yaw,
			Pitch: input.Pitch,
		})
	}

	return nil
}

func (b *Body) PhysicsState() physics.PhysicsState {
	if b == nil {
		return physics.PhysicsState{}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.physics
}
