package body

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Versifine/locus/internal/physics"
	"github.com/Versifine/locus/internal/protocol"
	"github.com/Versifine/locus/internal/world"
)

type PacketSender interface {
	SendPacket(packet *protocol.Packet) error
}

type BlockDigSender interface {
	SendBlockDig(status int32, location protocol.BlockPos, face int8) (int32, error)
}

type StateUpdater interface {
	UpdatePosition(pos world.Position)
}

type EntitySnapshotProvider interface {
	GetState() world.Snapshot
}

type Body struct {
	mu                sync.Mutex
	physics           physics.PhysicsState
	packetSender      PacketSender
	blockStore        physics.BlockStore
	stateUpdater      StateUpdater
	entitySource      EntitySnapshotProvider
	serverSprint      bool
	nextDigSequence   int32
	nextUseSequence   int32
	hasActiveHotbar   bool
	activeHotbarSlot  int8
	activeBreakTarget *physics.BlockPos
	lastAttack        bool
	lastUse           bool
	lastSwingAt       time.Time
}

const (
	swingInterval = 120 * time.Millisecond
)

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
	physics.PhysicsTickWithEntities(&b.physics, physics.InputState(effectiveInput), b.blockStore, entityColliders)
	pos := b.physics.Position
	onGround := b.physics.OnGround
	currentServerSprint := b.serverSprint
	b.mu.Unlock()

	newServerSprint, err := b.syncServerSprintAction(
		currentServerSprint,
		effectiveInput.Sprint,
	)
	if err != nil {
		return err
	}
	b.mu.Lock()
	b.serverSprint = newServerSprint
	b.mu.Unlock()

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
	playerInput := protocol.CreatePlayerInputPacket(
		effectiveInput.Forward,
		effectiveInput.Backward,
		effectiveInput.Left,
		effectiveInput.Right,
		effectiveInput.Jump,
		effectiveInput.Sneak,
		effectiveInput.Sprint,
	)
	if err := b.packetSender.SendPacket(playerInput); err != nil {
		return err
	}

	if err := b.syncHandsActions(effectiveInput); err != nil {
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

func (b *Body) syncServerSprintAction(
	currentSprint bool,
	desiredSprint bool,
) (bool, error) {
	if b == nil {
		return currentSprint, nil
	}

	idProvider, ok := b.packetSender.(interface{ SelfEntityID() (int32, bool) })
	if !ok {
		return currentSprint, nil
	}
	entityID, ok := idProvider.SelfEntityID()
	if !ok {
		return currentSprint, nil
	}

	send := func(action int32) error {
		packet := protocol.CreateEntityActionPacket(entityID, action, 0)
		if err := b.packetSender.SendPacket(packet); err != nil {
			return err
		}
		slog.Debug("Sent entity action", "entity_id", entityID, "action", action)
		return nil
	}

	if desiredSprint != currentSprint {
		if desiredSprint {
			if err := send(protocol.EntityActionStartSprinting); err != nil {
				return currentSprint, err
			}
		} else {
			if err := send(protocol.EntityActionStopSprinting); err != nil {
				return currentSprint, err
			}
		}
		currentSprint = desiredSprint
	}

	return currentSprint, nil
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

func (b *Body) syncHandsActions(input InputState) error {
	now := time.Now()
	if err := b.maybeSendArmAnimation(input, now); err != nil {
		return err
	}

	if err := b.syncHotbar(input.HotbarSlot); err != nil {
		return err
	}

	if input.Attack && input.AttackTarget != nil {
		packet := protocol.CreateUseEntityPacket(
			*input.AttackTarget,
			protocol.UseEntityActionAttack,
			nil,
			nil,
			nil,
			nil,
			input.Sneak,
		)
		if err := b.packetSender.SendPacket(packet); err != nil {
			return err
		}
	}

	if err := b.syncBreakTarget(input, now); err != nil {
		return err
	}

	if !input.Use {
		return nil
	}

	if input.PlaceTarget != nil {
		seq := b.nextUseSeq()
		packet := protocol.CreateBlockPlacePacket(
			protocol.BlockPos{
				X: int32(input.PlaceTarget.Pos.X),
				Y: int32(input.PlaceTarget.Pos.Y),
				Z: int32(input.PlaceTarget.Pos.Z),
			},
			input.PlaceTarget.Face,
			0,
			0.5,
			0.5,
			0.5,
			false,
			false,
			seq,
		)
		if err := b.packetSender.SendPacket(packet); err != nil {
			return err
		}
		return nil
	}

	if input.InteractTarget != nil {
		hand := int32(0)
		packet := protocol.CreateUseEntityPacket(
			*input.InteractTarget,
			protocol.UseEntityActionInteract,
			nil,
			nil,
			nil,
			&hand,
			input.Sneak,
		)
		if err := b.packetSender.SendPacket(packet); err != nil {
			return err
		}
		return nil
	}

	packet := protocol.CreateUseItemPacket(0, b.nextUseSeq())
	if err := b.packetSender.SendPacket(packet); err != nil {
		return err
	}

	return nil
}

func (b *Body) maybeSendArmAnimation(input InputState, now time.Time) error {
	b.mu.Lock()
	shouldSwing := false
	if input.Attack && (!b.lastAttack || now.Sub(b.lastSwingAt) >= swingInterval) {
		shouldSwing = true
	}
	if input.Use && (!b.lastUse || now.Sub(b.lastSwingAt) >= swingInterval) {
		shouldSwing = true
	}
	b.lastAttack = input.Attack
	b.lastUse = input.Use
	if shouldSwing {
		b.lastSwingAt = now
	}
	b.mu.Unlock()

	if !shouldSwing {
		return nil
	}
	return b.packetSender.SendPacket(protocol.CreateArmAnimationPacket(0))
}

func (b *Body) syncHotbar(slot *int8) error {
	if slot == nil {
		return nil
	}

	b.mu.Lock()
	same := b.hasActiveHotbar && b.activeHotbarSlot == *slot
	if !same {
		b.activeHotbarSlot = *slot
		b.hasActiveHotbar = true
	}
	b.mu.Unlock()

	if same {
		return nil
	}

	packet := protocol.CreateHeldItemSlotPacket(int16(*slot))
	return b.packetSender.SendPacket(packet)
}

func (b *Body) syncBreakTarget(input InputState, now time.Time) error {
	_ = now
	wantsBreak := input.Attack && input.BreakTarget != nil

	b.mu.Lock()
	active := b.activeBreakTarget
	b.mu.Unlock()

	if input.BreakFinished && input.BreakTarget != nil && active != nil && sameBlockPos(*active, *input.BreakTarget) {
		if err := b.sendBlockDig(protocol.BlockDigStatusFinished, *active, 1); err != nil {
			return err
		}
		b.mu.Lock()
		b.activeBreakTarget = nil
		b.mu.Unlock()
		return nil
	}

	if !wantsBreak {
		if active != nil {
			if err := b.sendBlockDig(protocol.BlockDigStatusCancelled, *active, 1); err != nil {
				return err
			}
			b.mu.Lock()
			b.activeBreakTarget = nil
			b.mu.Unlock()
		}
		return nil
	}

	target := *input.BreakTarget
	if active != nil && sameBlockPos(*active, target) {
		return nil
	}

	if active != nil {
		if err := b.sendBlockDig(protocol.BlockDigStatusCancelled, *active, 1); err != nil {
			return err
		}
	}

	if err := b.sendBlockDig(protocol.BlockDigStatusStarted, target, 1); err != nil {
		return err
	}

	b.mu.Lock()
	b.activeBreakTarget = &target
	b.mu.Unlock()

	return nil
}

func (b *Body) sendBlockDig(status int32, pos physics.BlockPos, face int8) error {
	location := protocol.BlockPos{X: int32(pos.X), Y: int32(pos.Y), Z: int32(pos.Z)}
	if sender, ok := b.packetSender.(BlockDigSender); ok {
		_, err := sender.SendBlockDig(status, location, face)
		return err
	}

	b.mu.Lock()
	sequence := b.nextDigSequence
	b.nextDigSequence++
	b.mu.Unlock()

	packet := protocol.CreateBlockDigPacket(status, location, face, sequence)
	return b.packetSender.SendPacket(packet)
}

func (b *Body) nextUseSeq() int32 {
	b.mu.Lock()
	seq := b.nextUseSequence
	b.nextUseSequence++
	b.mu.Unlock()
	return seq
}

func sameBlockPos(a, b physics.BlockPos) bool {
	return a.X == b.X && a.Y == b.Y && a.Z == b.Z
}
