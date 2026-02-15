package protocol

import (
	"bytes"
	"testing"
)

func TestCreateEntityActionPacket(t *testing.T) {
	packet := CreateEntityActionPacket(42, EntityActionStartSprinting, 0)
	if packet.ID != C2SEntityAction {
		t.Fatalf("packet.ID = %d, want %d", packet.ID, C2SEntityAction)
	}

	r := bytes.NewReader(packet.Payload)
	entityID, err := ReadVarint(r)
	if err != nil {
		t.Fatalf("ReadVarint(entityID) failed: %v", err)
	}
	actionID, err := ReadVarint(r)
	if err != nil {
		t.Fatalf("ReadVarint(actionID) failed: %v", err)
	}
	jumpBoost, err := ReadVarint(r)
	if err != nil {
		t.Fatalf("ReadVarint(jumpBoost) failed: %v", err)
	}

	if entityID != 42 || actionID != EntityActionStartSprinting || jumpBoost != 0 {
		t.Fatalf("unexpected payload values: entityID=%d actionID=%d jumpBoost=%d", entityID, actionID, jumpBoost)
	}
}
