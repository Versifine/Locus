package protocol

import (
	"bytes"
	"testing"
)

func TestClientInformation(t *testing.T) {
	locale := "en_US"
	viewDistance := int8(10)
	chatFlags := int32(0)
	chatColors := true
	skinParts := uint8(0x7F)
	mainHand := int32(1)
	enableTextFiltering := false
	enableServerListing := true
	particleStatus := int32(0)
	packetID := int32(0x00)

	packet := CreateClientInformationPacket(locale, viewDistance, chatFlags, chatColors, skinParts, mainHand, enableTextFiltering, enableServerListing, particleStatus, packetID)

	if packet.ID != packetID {
		t.Errorf("Expected packet ID %d, got %d", packetID, packet.ID)
	}

	buf := bytes.NewReader(packet.Payload)
	l, _ := ReadString(buf)
	vd, _ := ReadByte(buf)
	cf, _ := ReadVarint(buf)
	cc, _ := ReadBool(buf)
	sp, _ := ReadByte(buf)
	mh, _ := ReadVarint(buf)
	etf, _ := ReadBool(buf)
	esl, _ := ReadBool(buf)
	ps, _ := ReadVarint(buf)

	if l != locale || int8(vd) != viewDistance || cf != chatFlags || cc != chatColors || sp != skinParts || mh != mainHand || etf != enableTextFiltering || esl != enableServerListing || ps != particleStatus {
		t.Errorf("Packet payload mismatch")
	}
}

func TestCustomPayload(t *testing.T) {
	channel := "minecraft:brand"
	data := []byte("locus")
	packet := CreateCustomPayloadPacket(channel, data)

	if packet.ID != C2SCustomPayload {
		t.Errorf("Expected packet ID %d, got %d", C2SCustomPayload, packet.ID)
	}

	buf := bytes.NewReader(packet.Payload)
	c, _ := ReadString(buf)
	d := make([]byte, len(data))
	buf.Read(d)

	if c != channel || !bytes.Equal(d, data) {
		t.Errorf("CustomPayload mismatch")
	}
}

func TestSelectKnown(t *testing.T) {
	packs := []KnownPack{
		{NameSpace: "minecraft", Id: "core", Version: "1.21.1"},
	}
	packetID := int32(0x01)
	packet := CreateSelectKnownPacket(packs, packetID)

	buf := bytes.NewReader(packet.Payload)
	count, _ := ReadVarint(buf)
	if count != 1 {
		t.Errorf("Expected count 1, got %d", count)
	}
	ns, _ := ReadString(buf)
	id, _ := ReadString(buf)
	v, _ := ReadString(buf)

	if ns != packs[0].NameSpace || id != packs[0].Id || v != packs[0].Version {
		t.Errorf("SelectKnown mismatch")
	}
}

func TestFinishConfiguration(t *testing.T) {
	packetID := int32(0x03)
	packet := CreateFinishConfigurationPacket(packetID)
	if packet.ID != packetID || len(packet.Payload) != 0 {
		t.Errorf("FinishConfiguration mismatch")
	}
}
