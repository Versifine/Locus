package protocol

import (
	"bytes"
	"testing"
)

func TestCreateHeldItemSlotPacket(t *testing.T) {
	packet := CreateHeldItemSlotPacket(4)
	if packet.ID != C2SHeldItemSlot {
		t.Fatalf("packet.ID = %d, want %d", packet.ID, C2SHeldItemSlot)
	}

	r := bytes.NewReader(packet.Payload)
	slot, err := ReadInt16(r)
	if err != nil {
		t.Fatalf("ReadInt16(slot) failed: %v", err)
	}
	if slot != 4 {
		t.Fatalf("slot = %d, want 4", slot)
	}
}

func TestCreateUseEntityPacketAttack(t *testing.T) {
	packet := CreateUseEntityPacket(42, UseEntityActionAttack, nil, nil, nil, nil, false)
	if packet.ID != C2SUseEntity {
		t.Fatalf("packet.ID = %d, want %d", packet.ID, C2SUseEntity)
	}

	r := bytes.NewReader(packet.Payload)
	target, err := ReadVarint(r)
	if err != nil {
		t.Fatalf("ReadVarint(target) failed: %v", err)
	}
	action, err := ReadVarint(r)
	if err != nil {
		t.Fatalf("ReadVarint(action) failed: %v", err)
	}
	sneaking, err := ReadBool(r)
	if err != nil {
		t.Fatalf("ReadBool(sneaking) failed: %v", err)
	}

	if target != 42 || action != UseEntityActionAttack || sneaking {
		t.Fatalf("unexpected payload target=%d action=%d sneaking=%v", target, action, sneaking)
	}
}

func TestCreateUseEntityPacketInteract(t *testing.T) {
	hand := int32(1)
	packet := CreateUseEntityPacket(7, UseEntityActionInteract, nil, nil, nil, &hand, true)

	r := bytes.NewReader(packet.Payload)
	target, _ := ReadVarint(r)
	action, _ := ReadVarint(r)
	gotHand, err := ReadVarint(r)
	if err != nil {
		t.Fatalf("ReadVarint(hand) failed: %v", err)
	}
	sneaking, err := ReadBool(r)
	if err != nil {
		t.Fatalf("ReadBool(sneaking) failed: %v", err)
	}

	if target != 7 || action != UseEntityActionInteract || gotHand != 1 || !sneaking {
		t.Fatalf("unexpected payload target=%d action=%d hand=%d sneaking=%v", target, action, gotHand, sneaking)
	}
}

func TestCreateUseItemPacket(t *testing.T) {
	packet := CreateUseItemPacket(0, 9)
	if packet.ID != C2SUseItem {
		t.Fatalf("packet.ID = %d, want %d", packet.ID, C2SUseItem)
	}

	r := bytes.NewReader(packet.Payload)
	hand, _ := ReadVarint(r)
	sequence, _ := ReadVarint(r)
	rotX, err := ReadFloat(r)
	if err != nil {
		t.Fatalf("ReadFloat(rotX) failed: %v", err)
	}
	rotY, err := ReadFloat(r)
	if err != nil {
		t.Fatalf("ReadFloat(rotY) failed: %v", err)
	}

	if hand != 0 || sequence != 9 || rotX != 0 || rotY != 0 {
		t.Fatalf("unexpected payload hand=%d sequence=%d rot=(%f,%f)", hand, sequence, rotX, rotY)
	}
}

func TestCreateBlockPlacePacket(t *testing.T) {
	packet := CreateBlockPlacePacket(
		BlockPos{X: -3, Y: 64, Z: 9},
		1,
		0,
		0.5,
		0.25,
		0.75,
		true,
		false,
		77,
	)
	if packet.ID != C2SBlockPlace {
		t.Fatalf("packet.ID = %d, want %d", packet.ID, C2SBlockPlace)
	}

	r := bytes.NewReader(packet.Payload)
	hand, _ := ReadVarint(r)
	rawPos, err := ReadInt64(r)
	if err != nil {
		t.Fatalf("ReadInt64(position) failed: %v", err)
	}
	face, _ := ReadVarint(r)
	cursorX, _ := ReadFloat(r)
	cursorY, _ := ReadFloat(r)
	cursorZ, _ := ReadFloat(r)
	inside, _ := ReadBool(r)
	border, _ := ReadBool(r)
	sequence, _ := ReadVarint(r)

	x, y, z := decodePackedPosition(rawPos)
	if hand != 0 || x != -3 || y != 64 || z != 9 || face != 1 {
		t.Fatalf("unexpected block_place base fields hand=%d pos=(%d,%d,%d) face=%d", hand, x, y, z, face)
	}
	if cursorX != 0.5 || cursorY != 0.25 || cursorZ != 0.75 || !inside || border || sequence != 77 {
		t.Fatalf(
			"unexpected block_place payload cursor=(%f,%f,%f) inside=%v border=%v sequence=%d",
			cursorX,
			cursorY,
			cursorZ,
			inside,
			border,
			sequence,
		)
	}
}
