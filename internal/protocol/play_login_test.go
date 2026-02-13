package protocol

import (
	"bytes"
	"testing"
)

func TestParsePlayLogin(t *testing.T) {
	buf := new(bytes.Buffer)
	_ = WriteInt32(buf, 7)   // entityId
	_ = WriteBool(buf, true) // isHardcore
	_ = WriteVarint(buf, 2)  // worldNames count
	_ = WriteString(buf, "minecraft:overworld")
	_ = WriteString(buf, "minecraft:the_nether")
	_ = WriteVarint(buf, 120) // maxPlayers
	_ = WriteVarint(buf, 10)  // viewDistance
	_ = WriteVarint(buf, 10)  // simulationDistance
	_ = WriteBool(buf, false) // reducedDebugInfo
	_ = WriteBool(buf, true)  // enableRespawnScreen
	_ = WriteBool(buf, false) // doLimitedCrafting
	writeSpawnInfoForTest(buf, true)
	_ = WriteBool(buf, false) // enforcesSecureChat

	got, err := ParsePlayLogin(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ParsePlayLogin failed: %v", err)
	}

	if got.EntityID != 7 || !got.IsHardcore {
		t.Fatalf("unexpected login basics: %+v", got)
	}
	if got.MaxPlayers != 120 || got.ViewDistance != 10 || got.SimulationDistance != 10 {
		t.Fatalf("unexpected login limits: %+v", got)
	}
	if len(got.WorldNames) != 2 || got.WorldNames[0] != "minecraft:overworld" {
		t.Fatalf("unexpected world names: %+v", got.WorldNames)
	}
	if got.WorldState.Name != "minecraft:overworld" || got.WorldState.Dimension != 0 {
		t.Fatalf("unexpected world state: %+v", got.WorldState)
	}
	if got.WorldState.Death == nil || got.WorldState.Death.DimensionName != "minecraft:overworld" {
		t.Fatalf("expected death position to be present: %+v", got.WorldState.Death)
	}
}

func TestParseRespawn(t *testing.T) {
	buf := new(bytes.Buffer)
	writeSpawnInfoForTest(buf, false)
	_ = WriteByte(buf, 0x03) // copyMetadata

	got, err := ParseRespawn(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ParseRespawn failed: %v", err)
	}
	if got.CopyMetadata != 0x03 {
		t.Fatalf("unexpected copy metadata: got %d, want 3", got.CopyMetadata)
	}
	if got.WorldState.Death != nil {
		t.Fatalf("expected nil death position, got %+v", got.WorldState.Death)
	}
}

func TestParsePlayLoginInvalidWorldNameCount(t *testing.T) {
	buf := new(bytes.Buffer)
	_ = WriteInt32(buf, 1)
	_ = WriteBool(buf, false)
	_ = WriteVarint(buf, -1) // invalid worldNames count

	if _, err := ParsePlayLogin(bytes.NewReader(buf.Bytes())); err == nil {
		t.Fatalf("expected error for invalid worldNames count")
	}
}

func writeSpawnInfoForTest(buf *bytes.Buffer, withDeath bool) {
	_ = WriteVarint(buf, 0)                     // dimension
	_ = WriteString(buf, "minecraft:overworld") // name
	_ = WriteInt64(buf, 12345)                  // hashedSeed
	_ = WriteByte(buf, byte(0))                 // gamemode (survival)
	_ = WriteByte(buf, byte(255))               // previous gamemode
	_ = WriteBool(buf, false)                   // isDebug
	_ = WriteBool(buf, true)                    // isFlat

	_ = WriteBool(buf, withDeath) // death present
	if withDeath {
		_ = WriteString(buf, "minecraft:overworld")
		_ = WriteInt64(buf, testEncodeBlockPosition(10, 64, -5))
	}

	_ = WriteVarint(buf, 0)   // portalCooldown
	_ = WriteVarint(buf, -63) // seaLevel
}

func testEncodeBlockPosition(x, y, z int32) int64 {
	ux := uint64(int64(x) & 0x3FFFFFF)
	uy := uint64(int64(y) & 0xFFF)
	uz := uint64(int64(z) & 0x3FFFFFF)
	return int64((ux << 38) | (uz << 12) | uy)
}
