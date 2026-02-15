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

func TestBodyTickStandingForwardMovesAndSendsPacket(t *testing.T) {
	store := newMockBlockStore()
	addFloor(store, -4, 4, -4, 4, -1)
	sender := &mockPacketSender{}
	updater := &mockStateUpdater{}

	b := New(world.Position{X: 0.5, Y: 0.0, Z: 0.5, Yaw: 0, Pitch: 0}, true, sender, store, updater)
	if err := b.Tick(InputState{Forward: true, Yaw: 0, Pitch: 0}); err != nil {
		t.Fatalf("Tick failed: %v", err)
	}

	if len(sender.packets) != 1 {
		t.Fatalf("sent packets = %d, want 1", len(sender.packets))
	}
	packet := sender.packets[0]
	if packet.ID != protocol.C2SPlayerPositionLook {
		t.Fatalf("packet.ID = %d, want %d", packet.ID, protocol.C2SPlayerPositionLook)
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

	if len(sender.packets) != 2 {
		t.Fatalf("sent packets = %d, want 2", len(sender.packets))
	}
	_, y, _, _, _, flags := parsePositionLookPayload(t, sender.packets[1])
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
	if err := b.Tick(InputState{Right: true, Yaw: 0, Pitch: 0}); err != nil {
		t.Fatalf("Tick failed: %v", err)
	}

	if len(sender.packets) != 1 {
		t.Fatalf("sent packets = %d, want 1", len(sender.packets))
	}
	x, y, z, _, _, _ := parsePositionLookPayload(t, sender.packets[0])
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

	if len(actionIDs) != 3 {
		t.Fatalf("entity action packet count = %d, want 3", len(actionIDs))
	}
	if actionIDs[0] != protocol.EntityActionStartSprinting {
		t.Fatalf("first actionID = %d, want %d", actionIDs[0], protocol.EntityActionStartSprinting)
	}
	if actionIDs[1] != protocol.EntityActionStartSneaking {
		t.Fatalf("second actionID = %d, want %d", actionIDs[1], protocol.EntityActionStartSneaking)
	}
	if actionIDs[2] != protocol.EntityActionStopSprinting {
		t.Fatalf("third actionID = %d, want %d", actionIDs[2], protocol.EntityActionStopSprinting)
	}
}
