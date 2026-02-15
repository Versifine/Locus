package body

import (
	"fmt"
	"log/slog"
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

type EntitySnapshotProvider interface {
	GetState() world.Snapshot
}

type Body struct {
	mu           sync.Mutex
	physics      physics.PhysicsState
	packetSender PacketSender
	blockStore   physics.BlockStore
	stateUpdater StateUpdater
	entitySource EntitySnapshotProvider
	lastInput    InputState
	hasLastInput bool
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

	effectiveInput := normalizeMovementInput(input)
	entityColliders := b.currentEntityColliders()

	b.mu.Lock()
	startSneak := b.hasLastInput && !b.lastInput.Sneak && effectiveInput.Sneak
	stopSneak := b.hasLastInput && b.lastInput.Sneak && !effectiveInput.Sneak
	startSprint := b.hasLastInput && !b.lastInput.Sprint && effectiveInput.Sprint
	stopSprint := b.hasLastInput && b.lastInput.Sprint && !effectiveInput.Sprint
	b.lastInput = effectiveInput
	b.hasLastInput = true

	physics.PhysicsTickWithEntities(&b.physics, physics.InputState(effectiveInput), b.blockStore, entityColliders)
	pos := b.physics.Position
	onGround := b.physics.OnGround
	b.mu.Unlock()

	if err := b.sendEntityActions(startSneak, stopSneak, startSprint, stopSprint); err != nil {
		return err
	}

	packet := protocol.CreatePlayerPositionAndRotationPacket(
		pos.X,
		pos.Y,
		pos.Z,
		effectiveInput.Yaw,
		effectiveInput.Pitch,
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
			Yaw:   effectiveInput.Yaw,
			Pitch: effectiveInput.Pitch,
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

func (b *Body) SetLocalPosition(pos world.Position) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.physics.Position = physics.Vec3{X: pos.X, Y: pos.Y, Z: pos.Z}
	b.mu.Unlock()
	if b.stateUpdater != nil {
		b.stateUpdater.UpdatePosition(pos)
	}
}

func (b *Body) SetEntityProvider(source EntitySnapshotProvider) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.entitySource = source
	b.mu.Unlock()
}

func (b *Body) currentEntityColliders() []physics.EntityCollider {
	b.mu.Lock()
	source := b.entitySource
	b.mu.Unlock()

	if source == nil {
		return nil
	}

	snapshot := source.GetState()
	if len(snapshot.Entities) == 0 {
		return nil
	}

	colliders := make([]physics.EntityCollider, 0, len(snapshot.Entities))
	for _, e := range snapshot.Entities {
		// Item entities are ignored for push resolution.
		if e.Type == 71 {
			continue
		}
		colliders = append(colliders, physics.EntityCollider{
			X:      e.X,
			Y:      e.Y,
			Z:      e.Z,
			Width:  physics.PlayerWidth,
			Height: physics.PlayerHeight,
		})
	}
	return colliders
}

func (b *Body) sendEntityActions(startSneak, stopSneak, startSprint, stopSprint bool) error {
	if b == nil {
		return nil
	}
	if !startSneak && !stopSneak && !startSprint && !stopSprint {
		return nil
	}

	idProvider, ok := b.packetSender.(interface{ SelfEntityID() (int32, bool) })
	if !ok {
		return nil
	}
	entityID, ok := idProvider.SelfEntityID()
	if !ok {
		return nil
	}

	send := func(action int32) error {
		packet := protocol.CreateEntityActionPacket(entityID, action, 0)
		if err := b.packetSender.SendPacket(packet); err != nil {
			return err
		}
		slog.Debug("Sent entity action", "entity_id", entityID, "action", action)
		return nil
	}

	if startSneak {
		if err := send(protocol.EntityActionStartSneaking); err != nil {
			return err
		}
	}
	if stopSneak {
		if err := send(protocol.EntityActionStopSneaking); err != nil {
			return err
		}
	}
	if startSprint {
		if err := send(protocol.EntityActionStartSprinting); err != nil {
			return err
		}
	}
	if stopSprint {
		if err := send(protocol.EntityActionStopSprinting); err != nil {
			return err
		}
	}

	return nil
}

func normalizeMovementInput(input InputState) InputState {
	out := input

	// Vanilla behavior: sneaking and sprinting are mutually exclusive.
	if out.Sneak {
		out.Sprint = false
	}
	// Sprint requires forward movement intent.
	if !out.Forward || out.Backward {
		out.Sprint = false
	}

	return out
}
