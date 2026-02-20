package body

import (
	"bytes"
	"testing"

	"github.com/Versifine/locus/internal/physics"
	"github.com/Versifine/locus/internal/protocol"
	"github.com/Versifine/locus/internal/world"
)

type mockPacketSender struct {
	packets   []*protocol.Packet
	entityID  int32
	hasEntity bool
}

func (m *mockPacketSender) SendPacket(packet *protocol.Packet) error {
	m.packets = append(m.packets, packet)
	return nil
}

func (m *mockPacketSender) SelfEntityID() (int32, bool) {
	return m.entityID, m.hasEntity
}

type mockStateUpdater struct {
	positions []world.Position
}

func (m *mockStateUpdater) UpdatePosition(pos world.Position) {
	m.positions = append(m.positions, pos)
}

type mockBlockStore struct {
	solid map[[3]int]bool
}

func newMockBlockStore() *mockBlockStore {
	return &mockBlockStore{solid: make(map[[3]int]bool)}
}

func (m *mockBlockStore) IsSolid(x, y, z int) bool {
	return m.solid[[3]int{x, y, z}]
}

func (m *mockBlockStore) setSolid(x, y, z int) {
	m.solid[[3]int{x, y, z}] = true
}

func addFloor(store *mockBlockStore, minX, maxX, minZ, maxZ, y int) {
	for x := minX; x <= maxX; x++ {
		for z := minZ; z <= maxZ; z++ {
			store.setSolid(x, y, z)
		}
	}
}

func parsePositionLookPayload(t *testing.T, packet *protocol.Packet) (float64, float64, float64, float32, float32, byte) {
	t.Helper()
	r := bytes.NewReader(packet.Payload)
	x, err := protocol.ReadDouble(r)
	if err != nil {
		t.Fatalf("ReadDouble(x): %v", err)
	}
	y, err := protocol.ReadDouble(r)
	if err != nil {
		t.Fatalf("ReadDouble(y): %v", err)
	}
	z, err := protocol.ReadDouble(r)
	if err != nil {
		t.Fatalf("ReadDouble(z): %v", err)
	}
	yaw, err := protocol.ReadFloat(r)
	if err != nil {
		t.Fatalf("ReadFloat(yaw): %v", err)
	}
	pitch, err := protocol.ReadFloat(r)
	if err != nil {
		t.Fatalf("ReadFloat(pitch): %v", err)
	}
	flags, err := protocol.ReadByte(r)
	if err != nil {
		t.Fatalf("ReadByte(flags): %v", err)
	}
	return x, y, z, yaw, pitch, flags
}

func parseUseEntityPayload(t *testing.T, packet *protocol.Packet) (int32, int32, *int32, bool) {
	t.Helper()
	r := bytes.NewReader(packet.Payload)
	target, err := protocol.ReadVarint(r)
	if err != nil {
		t.Fatalf("ReadVarint(target) failed: %v", err)
	}
	action, err := protocol.ReadVarint(r)
	if err != nil {
		t.Fatalf("ReadVarint(action) failed: %v", err)
	}
	var hand *int32
	if action == protocol.UseEntityActionInteract || action == protocol.UseEntityActionInteractAt {
		value, err := protocol.ReadVarint(r)
		if err != nil {
			t.Fatalf("ReadVarint(hand) failed: %v", err)
		}
		hand = &value
	}
	sneaking, err := protocol.ReadBool(r)
	if err != nil {
		t.Fatalf("ReadBool(sneaking) failed: %v", err)
	}
	return target, action, hand, sneaking
}

func parseBlockDigPayload(t *testing.T, packet *protocol.Packet) (int32, int32, int32, int32, int8, int32) {
	t.Helper()
	r := bytes.NewReader(packet.Payload)
	status, err := protocol.ReadVarint(r)
	if err != nil {
		t.Fatalf("ReadVarint(status) failed: %v", err)
	}
	rawPos, err := protocol.ReadInt64(r)
	if err != nil {
		t.Fatalf("ReadInt64(position) failed: %v", err)
	}
	face, err := protocol.ReadByte(r)
	if err != nil {
		t.Fatalf("ReadByte(face) failed: %v", err)
	}
	sequence, err := protocol.ReadVarint(r)
	if err != nil {
		t.Fatalf("ReadVarint(sequence) failed: %v", err)
	}
	x, y, z := decodePackedPosition(rawPos)
	return status, x, y, z, int8(face), sequence
}

func decodePackedPosition(raw int64) (int32, int32, int32) {
	v := uint64(raw)
	x := signExtendInt32(int64((v>>38)&0x3FFFFFF), 26)
	z := signExtendInt32(int64((v>>12)&0x3FFFFFF), 26)
	y := signExtendInt32(int64(v&0xFFF), 12)
	return x, y, z
}

func signExtendInt32(value int64, bits uint) int32 {
	shift := 64 - bits
	return int32((value << shift) >> shift)
}

func lastPacketByID(packets []*protocol.Packet, id int32) *protocol.Packet {
	for i := len(packets) - 1; i >= 0; i-- {
		if packets[i].ID == id {
			return packets[i]
		}
	}
	return nil
}

func TestBodyTickStandingForwardMovesAndSendsPacket(t *testing.T) {
	store := newMockBlockStore()
	addFloor(store, -4, 4, -4, 4, -1)
	sender := &mockPacketSender{}
	updater := &mockStateUpdater{}

	b := New(world.Position{X: 0.5, Y: 0.0, Z: 0.5, Yaw: 0, Pitch: 0}, true, sender, store, updater)
	if err := b.Tick(InputState{Forward: true, Yaw: 0, Pitch: 0}); err != nil {
		t.Fatalf("Tick failed: %v", err)
	}

	if len(sender.packets) != 2 {
		t.Fatalf("sent packets = %d, want 2", len(sender.packets))
	}
	packet := lastPacketByID(sender.packets, protocol.C2SPlayerPositionLook)
	if packet == nil {
		t.Fatalf("missing packet ID %d", protocol.C2SPlayerPositionLook)
	}

	x, y, z, yaw, pitch, flags := parsePositionLookPayload(t, packet)
	if x != 0.5 {
		t.Fatalf("x = %.6f, want 0.5", x)
	}
	if y != 0.0 {
		t.Fatalf("y = %.6f, want 0.0", y)
	}
	if z <= 0.5 {
		t.Fatalf("z = %.6f, want > 0.5", z)
	}
	if yaw != 0 || pitch != 0 {
		t.Fatalf("yaw/pitch = %.2f/%.2f, want 0/0", yaw, pitch)
	}
	if flags&0x01 == 0 {
		t.Fatalf("flags = 0x%02x, expected onGround bit set", flags)
	}

	if len(updater.positions) != 1 {
		t.Fatalf("updated positions = %d, want 1", len(updater.positions))
	}
	if updater.positions[0].Z <= 0.5 {
		t.Fatalf("updated z = %.6f, want > 0.5", updater.positions[0].Z)
	}
}

func TestBodyTickAirborneFallsAndClearsOnGround(t *testing.T) {
	store := newMockBlockStore()
	sender := &mockPacketSender{}
	updater := &mockStateUpdater{}

	b := New(world.Position{X: 0, Y: 10, Z: 0, Yaw: 0, Pitch: 0}, false, sender, store, updater)
	if err := b.Tick(InputState{}); err != nil {
		t.Fatalf("Tick failed: %v", err)
	}
	if err := b.Tick(InputState{}); err != nil {
		t.Fatalf("second Tick failed: %v", err)
	}

	if len(sender.packets) != 4 {
		t.Fatalf("sent packets = %d, want 4", len(sender.packets))
	}
	posPacket := lastPacketByID(sender.packets, protocol.C2SPlayerPositionLook)
	if posPacket == nil {
		t.Fatalf("missing packet ID %d", protocol.C2SPlayerPositionLook)
	}
	_, y, _, _, _, flags := parsePositionLookPayload(t, posPacket)
	if y >= 10 {
		t.Fatalf("y = %.6f, want < 10", y)
	}
	if flags&0x01 != 0 {
		t.Fatalf("flags = 0x%02x, expected onGround bit cleared", flags)
	}

	state := b.PhysicsState()
	if state.OnGround {
		t.Fatalf("physics onGround = true, want false")
	}
	if state.Position.Y >= 10 {
		t.Fatalf("physics y = %.6f, want < 10", state.Position.Y)
	}
}

func TestBodyTickWallCollisionKeepsPosition(t *testing.T) {
	store := newMockBlockStore()
	addFloor(store, -2, 2, -2, 2, -1)
	store.setSolid(1, 0, 0)
	store.setSolid(1, 1, 0)

	sender := &mockPacketSender{}
	updater := &mockStateUpdater{}

	b := New(world.Position{X: 0.7, Y: 0.0, Z: 0.5, Yaw: 0, Pitch: 0}, true, sender, store, updater)
	if err := b.Tick(InputState{Left: true, Yaw: 0, Pitch: 0}); err != nil {
		t.Fatalf("Tick failed: %v", err)
	}

	if len(sender.packets) != 2 {
		t.Fatalf("sent packets = %d, want 2", len(sender.packets))
	}
	posPacket := lastPacketByID(sender.packets, protocol.C2SPlayerPositionLook)
	if posPacket == nil {
		t.Fatalf("missing packet ID %d", protocol.C2SPlayerPositionLook)
	}
	x, y, z, _, _, _ := parsePositionLookPayload(t, posPacket)
	if x != 0.7 || y != 0.0 || z != 0.5 {
		t.Fatalf("position moved through wall: x=%.6f y=%.6f z=%.6f", x, y, z)
	}

	state := b.PhysicsState()
	if state.Position != (physics.Vec3{X: 0.7, Y: 0.0, Z: 0.5}) {
		t.Fatalf("physics state moved through wall: %+v", state.Position)
	}
}

func TestBodyTickSprintAndSneakTransitionsSendEntityAction(t *testing.T) {
	store := newMockBlockStore()
	addFloor(store, -4, 4, -4, 4, -1)
	sender := &mockPacketSender{entityID: 7, hasEntity: true}
	updater := &mockStateUpdater{}

	b := New(world.Position{X: 0.5, Y: 0.0, Z: 0.5}, true, sender, store, updater)

	if err := b.Tick(InputState{Yaw: 0, Pitch: 0}); err != nil {
		t.Fatalf("first Tick failed: %v", err)
	}
	if err := b.Tick(InputState{Forward: true, Sprint: true, Yaw: 0, Pitch: 0}); err != nil {
		t.Fatalf("second Tick failed: %v", err)
	}
	if err := b.Tick(InputState{Sprint: true, Yaw: 0, Pitch: 0}); err != nil {
		t.Fatalf("third Tick failed: %v", err)
	}

	var actionIDs []int32
	for _, packet := range sender.packets {
		if packet.ID != protocol.C2SEntityAction {
			continue
		}
		r := bytes.NewReader(packet.Payload)
		gotEntityID, err := protocol.ReadVarint(r)
		if err != nil {
			t.Fatalf("ReadVarint(entityID) failed: %v", err)
		}
		gotActionID, err := protocol.ReadVarint(r)
		if err != nil {
			t.Fatalf("ReadVarint(actionID) failed: %v", err)
		}
		if gotEntityID != 7 {
			t.Fatalf("entityID = %d, want 7", gotEntityID)
		}
		actionIDs = append(actionIDs, gotActionID)
	}

	if len(actionIDs) != 2 {
		t.Fatalf("entity action packet count = %d, want 2", len(actionIDs))
	}
	if actionIDs[0] != protocol.EntityActionStartSprinting {
		t.Fatalf("first actionID = %d, want %d", actionIDs[0], protocol.EntityActionStartSprinting)
	}
	if actionIDs[1] != protocol.EntityActionStopSprinting {
		t.Fatalf("second actionID = %d, want %d", actionIDs[1], protocol.EntityActionStopSprinting)
	}
}

func TestBodyTickSneakCancelsSprint(t *testing.T) {
	store := newMockBlockStore()
	addFloor(store, -4, 4, -4, 4, -1)
	sender := &mockPacketSender{entityID: 7, hasEntity: true}
	updater := &mockStateUpdater{}

	b := New(world.Position{X: 0.5, Y: 0.0, Z: 0.5}, true, sender, store, updater)
	if err := b.Tick(InputState{Yaw: 0, Pitch: 0}); err != nil {
		t.Fatalf("first Tick failed: %v", err)
	}
	if err := b.Tick(InputState{Forward: true, Sprint: true, Yaw: 0, Pitch: 0}); err != nil {
		t.Fatalf("second Tick failed: %v", err)
	}
	if err := b.Tick(InputState{Forward: true, Sprint: true, Sneak: true, Yaw: 0, Pitch: 0}); err != nil {
		t.Fatalf("third Tick failed: %v", err)
	}

	var actionIDs []int32
	for _, packet := range sender.packets {
		if packet.ID != protocol.C2SEntityAction {
			continue
		}
		r := bytes.NewReader(packet.Payload)
		_, _ = protocol.ReadVarint(r)
		actionID, err := protocol.ReadVarint(r)
		if err != nil {
			t.Fatalf("ReadVarint(actionID) failed: %v", err)
		}
		actionIDs = append(actionIDs, actionID)
	}

	if len(actionIDs) != 2 {
		t.Fatalf("entity action packet count = %d, want 2", len(actionIDs))
	}
	if actionIDs[0] != protocol.EntityActionStartSprinting {
		t.Fatalf("first actionID = %d, want %d", actionIDs[0], protocol.EntityActionStartSprinting)
	}
	if actionIDs[1] != protocol.EntityActionStopSprinting {
		t.Fatalf("second actionID = %d, want %d", actionIDs[1], protocol.EntityActionStopSprinting)
	}
}

func TestBodyTickSendsSneakViaPlayerInput(t *testing.T) {
	store := newMockBlockStore()
	addFloor(store, -4, 4, -4, 4, -1)
	sender := &mockPacketSender{entityID: 7, hasEntity: true}
	updater := &mockStateUpdater{}

	b := New(world.Position{X: 0.5, Y: 0.0, Z: 0.5}, true, sender, store, updater)
	if err := b.Tick(InputState{Sneak: true, Yaw: 0, Pitch: 0}); err != nil {
		t.Fatalf("Tick failed: %v", err)
	}

	packet := lastPacketByID(sender.packets, protocol.C2SPlayerInput)
	if packet == nil {
		t.Fatalf("missing packet ID %d", protocol.C2SPlayerInput)
	}

	r := bytes.NewReader(packet.Payload)
	flags, err := protocol.ReadByte(r)
	if err != nil {
		t.Fatalf("ReadByte(flags) failed: %v", err)
	}
	if flags&(1<<5) == 0 {
		t.Fatalf("player_input flags = 0x%02x, expected shift bit set", flags)
	}
}

func TestBodyTickHotbarSlotSentOnlyOnChange(t *testing.T) {
	store := newMockBlockStore()
	addFloor(store, -4, 4, -4, 4, -1)
	sender := &mockPacketSender{}
	b := New(world.Position{X: 0.5, Y: 0.0, Z: 0.5}, true, sender, store, nil)

	slot2 := int8(2)
	if err := b.Tick(InputState{HotbarSlot: &slot2}); err != nil {
		t.Fatalf("first Tick failed: %v", err)
	}
	if err := b.Tick(InputState{HotbarSlot: &slot2}); err != nil {
		t.Fatalf("second Tick failed: %v", err)
	}
	slot3 := int8(3)
	if err := b.Tick(InputState{HotbarSlot: &slot3}); err != nil {
		t.Fatalf("third Tick failed: %v", err)
	}

	var slots []int16
	for _, packet := range sender.packets {
		if packet.ID != protocol.C2SHeldItemSlot {
			continue
		}
		r := bytes.NewReader(packet.Payload)
		slot, err := protocol.ReadInt16(r)
		if err != nil {
			t.Fatalf("ReadInt16(slot) failed: %v", err)
		}
		slots = append(slots, slot)
	}

	if len(slots) != 2 {
		t.Fatalf("held_item_slot count = %d, want 2", len(slots))
	}
	if slots[0] != 2 || slots[1] != 3 {
		t.Fatalf("held_item_slot payloads = %v, want [2 3]", slots)
	}
}

func TestBodyTickAttackTargetSendsUseEntityAttack(t *testing.T) {
	store := newMockBlockStore()
	addFloor(store, -4, 4, -4, 4, -1)
	sender := &mockPacketSender{}
	b := New(world.Position{X: 0.5, Y: 0.0, Z: 0.5}, true, sender, store, nil)

	target := int32(99)
	if err := b.Tick(InputState{Attack: true, AttackTarget: &target}); err != nil {
		t.Fatalf("Tick failed: %v", err)
	}

	packet := lastPacketByID(sender.packets, protocol.C2SUseEntity)
	if packet == nil {
		t.Fatalf("missing packet ID %d", protocol.C2SUseEntity)
	}
	gotTarget, action, hand, sneaking := parseUseEntityPayload(t, packet)
	if gotTarget != 99 || action != protocol.UseEntityActionAttack || hand != nil || sneaking {
		t.Fatalf("unexpected use_entity payload target=%d action=%d hand=%v sneaking=%v", gotTarget, action, hand, sneaking)
	}
}

func TestBodyTickUseWithoutTargetSendsUseItem(t *testing.T) {
	store := newMockBlockStore()
	addFloor(store, -4, 4, -4, 4, -1)
	sender := &mockPacketSender{}
	b := New(world.Position{X: 0.5, Y: 0.0, Z: 0.5}, true, sender, store, nil)

	if err := b.Tick(InputState{Use: true}); err != nil {
		t.Fatalf("Tick failed: %v", err)
	}

	packet := lastPacketByID(sender.packets, protocol.C2SUseItem)
	if packet == nil {
		t.Fatalf("missing packet ID %d", protocol.C2SUseItem)
	}
	r := bytes.NewReader(packet.Payload)
	hand, _ := protocol.ReadVarint(r)
	sequence, _ := protocol.ReadVarint(r)
	if hand != 0 || sequence != 0 {
		t.Fatalf("use_item payload hand=%d sequence=%d, want 0/0", hand, sequence)
	}
}

func TestBodyTickUsePlaceTargetSendsBlockPlace(t *testing.T) {
	store := newMockBlockStore()
	addFloor(store, -4, 4, -4, 4, -1)
	sender := &mockPacketSender{}
	b := New(world.Position{X: 0.5, Y: 0.0, Z: 0.5}, true, sender, store, nil)

	place := physics.PlaceAction{Pos: physics.BlockPos{X: 1, Y: 64, Z: 2}, Face: 1}
	if err := b.Tick(InputState{Use: true, PlaceTarget: &place}); err != nil {
		t.Fatalf("Tick failed: %v", err)
	}

	packet := lastPacketByID(sender.packets, protocol.C2SBlockPlace)
	if packet == nil {
		t.Fatalf("missing packet ID %d", protocol.C2SBlockPlace)
	}
	r := bytes.NewReader(packet.Payload)
	_, _ = protocol.ReadVarint(r) // hand
	rawPos, _ := protocol.ReadInt64(r)
	face, _ := protocol.ReadVarint(r)
	x, y, z := decodePackedPosition(rawPos)
	if x != 1 || y != 64 || z != 2 || face != 1 {
		t.Fatalf("block_place payload pos=(%d,%d,%d) face=%d", x, y, z, face)
	}
}

func TestBodyTickUseInteractTargetSendsUseEntityInteract(t *testing.T) {
	store := newMockBlockStore()
	addFloor(store, -4, 4, -4, 4, -1)
	sender := &mockPacketSender{}
	b := New(world.Position{X: 0.5, Y: 0.0, Z: 0.5}, true, sender, store, nil)

	target := int32(11)
	if err := b.Tick(InputState{Use: true, InteractTarget: &target, Sneak: true}); err != nil {
		t.Fatalf("Tick failed: %v", err)
	}

	packet := lastPacketByID(sender.packets, protocol.C2SUseEntity)
	if packet == nil {
		t.Fatalf("missing packet ID %d", protocol.C2SUseEntity)
	}
	gotTarget, action, hand, sneaking := parseUseEntityPayload(t, packet)
	if hand == nil {
		t.Fatalf("hand is nil, want 0")
	}
	if gotTarget != 11 || action != protocol.UseEntityActionInteract || *hand != 0 || !sneaking {
		t.Fatalf("unexpected use_entity payload target=%d action=%d hand=%d sneaking=%v", gotTarget, action, *hand, sneaking)
	}
}

func TestBodyTickBreakStateMachine(t *testing.T) {
	store := newMockBlockStore()
	addFloor(store, -4, 4, -4, 4, -1)
	sender := &mockPacketSender{}
	b := New(world.Position{X: 0.5, Y: 0.0, Z: 0.5}, true, sender, store, nil)

	a := physics.BlockPos{X: 1, Y: 64, Z: 2}
	bPos := physics.BlockPos{X: 2, Y: 64, Z: 2}

	if err := b.Tick(InputState{Attack: true, BreakTarget: &a}); err != nil {
		t.Fatalf("first Tick failed: %v", err)
	}
	if err := b.Tick(InputState{Attack: true, BreakTarget: &a}); err != nil {
		t.Fatalf("second Tick failed: %v", err)
	}
	if err := b.Tick(InputState{Attack: true, BreakTarget: &bPos}); err != nil {
		t.Fatalf("third Tick failed: %v", err)
	}
	if err := b.Tick(InputState{}); err != nil {
		t.Fatalf("fourth Tick failed: %v", err)
	}

	type digEvent struct {
		status int32
		x      int32
		y      int32
		z      int32
		face   int8
	}
	var events []digEvent
	for _, packet := range sender.packets {
		if packet.ID != protocol.C2SBlockDig {
			continue
		}
		status, x, y, z, face, _ := parseBlockDigPayload(t, packet)
		events = append(events, digEvent{status: status, x: x, y: y, z: z, face: face})
	}

	if len(events) != 4 {
		t.Fatalf("block_dig count = %d, want 4", len(events))
	}
	if events[0].status != protocol.BlockDigStatusStarted || events[0].x != 1 || events[0].z != 2 {
		t.Fatalf("event[0] = %+v", events[0])
	}
	if events[1].status != protocol.BlockDigStatusCancelled || events[1].x != 1 || events[1].z != 2 {
		t.Fatalf("event[1] = %+v", events[1])
	}
	if events[2].status != protocol.BlockDigStatusStarted || events[2].x != 2 || events[2].z != 2 {
		t.Fatalf("event[2] = %+v", events[2])
	}
	if events[3].status != protocol.BlockDigStatusCancelled || events[3].x != 2 || events[3].z != 2 {
		t.Fatalf("event[3] = %+v", events[3])
	}
}

func TestBodyTickBreakHoldDoesNotResendStart(t *testing.T) {
	store := newMockBlockStore()
	addFloor(store, -4, 4, -4, 4, -1)
	sender := &mockPacketSender{}
	b := New(world.Position{X: 0.5, Y: 0.0, Z: 0.5}, true, sender, store, nil)

	target := physics.BlockPos{X: 3, Y: 64, Z: 3}
	if err := b.Tick(InputState{Attack: true, BreakTarget: &target}); err != nil {
		t.Fatalf("first tick failed: %v", err)
	}
	if err := b.Tick(InputState{Attack: true, BreakTarget: &target}); err != nil {
		t.Fatalf("second tick failed: %v", err)
	}
	if err := b.Tick(InputState{}); err != nil {
		t.Fatalf("third tick failed: %v", err)
	}

	var statuses []int32
	for _, packet := range sender.packets {
		if packet.ID != protocol.C2SBlockDig {
			continue
		}
		status, _, _, _, _, _ := parseBlockDigPayload(t, packet)
		statuses = append(statuses, status)
	}

	if len(statuses) != 2 {
		t.Fatalf("block_dig count = %d, want 2", len(statuses))
	}
	if statuses[0] != protocol.BlockDigStatusStarted {
		t.Fatalf("first status = %d, want %d", statuses[0], protocol.BlockDigStatusStarted)
	}
	if statuses[1] != protocol.BlockDigStatusCancelled {
		t.Fatalf("second status = %d, want %d", statuses[1], protocol.BlockDigStatusCancelled)
	}
}

func TestBodyTickBreakFinishedSendsFinishAndClearsActiveTarget(t *testing.T) {
	store := newMockBlockStore()
	addFloor(store, -4, 4, -4, 4, -1)
	sender := &mockPacketSender{}
	b := New(world.Position{X: 0.5, Y: 0.0, Z: 0.5}, true, sender, store, nil)

	target := physics.BlockPos{X: 2, Y: 64, Z: 1}
	if err := b.Tick(InputState{Attack: true, BreakTarget: &target}); err != nil {
		t.Fatalf("first tick failed: %v", err)
	}
	if err := b.Tick(InputState{Attack: true, BreakTarget: &target, BreakFinished: true}); err != nil {
		t.Fatalf("second tick failed: %v", err)
	}
	if err := b.Tick(InputState{Attack: true, BreakTarget: &target}); err != nil {
		t.Fatalf("third tick failed: %v", err)
	}
	if err := b.Tick(InputState{}); err != nil {
		t.Fatalf("fourth tick failed: %v", err)
	}

	var statuses []int32
	for _, packet := range sender.packets {
		if packet.ID != protocol.C2SBlockDig {
			continue
		}
		status, _, _, _, _, _ := parseBlockDigPayload(t, packet)
		statuses = append(statuses, status)
	}

	if len(statuses) != 4 {
		t.Fatalf("block_dig count = %d, want 4", len(statuses))
	}
	if statuses[0] != protocol.BlockDigStatusStarted {
		t.Fatalf("status[0] = %d, want started", statuses[0])
	}
	if statuses[1] != protocol.BlockDigStatusFinished {
		t.Fatalf("status[1] = %d, want finished", statuses[1])
	}
	if statuses[2] != protocol.BlockDigStatusStarted {
		t.Fatalf("status[2] = %d, want started", statuses[2])
	}
	if statuses[3] != protocol.BlockDigStatusCancelled {
		t.Fatalf("status[3] = %d, want cancelled", statuses[3])
	}
}

func TestBodyTickBreakClearedTargetCancels(t *testing.T) {
	store := newMockBlockStore()
	addFloor(store, -4, 4, -4, 4, -1)

	sender := &mockPacketSender{}
	b := New(world.Position{X: 0.5, Y: 0.0, Z: 0.5}, true, sender, store, nil)

	target := physics.BlockPos{X: 0, Y: 64, Z: 1}
	if err := b.Tick(InputState{Attack: true, BreakTarget: &target}); err != nil {
		t.Fatalf("first tick failed: %v", err)
	}
	if err := b.Tick(InputState{Attack: true}); err != nil {
		t.Fatalf("second tick failed: %v", err)
	}

	var statuses []int32
	for _, packet := range sender.packets {
		if packet.ID != protocol.C2SBlockDig {
			continue
		}
		status, _, _, _, _, _ := parseBlockDigPayload(t, packet)
		statuses = append(statuses, status)
	}

	if len(statuses) != 2 {
		t.Fatalf("block_dig count = %d, want 2", len(statuses))
	}
	if statuses[0] != protocol.BlockDigStatusStarted {
		t.Fatalf("first status = %d, want started", statuses[0])
	}
	if statuses[1] != protocol.BlockDigStatusCancelled {
		t.Fatalf("second status = %d, want cancelled", statuses[1])
	}
}
