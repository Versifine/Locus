package protocol

import "bytes"

type ClientInformation struct {
	Locale              string
	ViewDistance        int8
	ChatFlags           int32
	ChatColors          bool
	SkinParts           uint8
	MainHand            int32
	EnableTextFiltering bool
	EnableServerListing bool
	ParticleStatus      int32
}

func CreateClientInformationPacket(locale string, viewDistance int8, chatFlags int32, chatColors bool, skinParts uint8, mainHand int32, enableTextFiltering bool, enableServerListing bool, particleStatus int32, packetID int32) *Packet {
	payload := make([]byte, 0)
	writer := bytes.NewBuffer(payload)
	_ = WriteString(writer, locale)
	_ = WriteByte(writer, byte(viewDistance))
	_ = WriteVarint(writer, chatFlags)
	_ = WriteBool(writer, chatColors)
	_ = WriteByte(writer, skinParts)
	_ = WriteVarint(writer, mainHand)
	_ = WriteBool(writer, enableTextFiltering)
	_ = WriteBool(writer, enableServerListing)
	_ = WriteVarint(writer, particleStatus)

	return &Packet{
		ID:      packetID,
		Payload: writer.Bytes(),
	}
}

type CustomPayload struct {
	Channel string
	Data    []byte
}

func CreateCustomPayloadPacket(channel string, data []byte) *Packet {
	buf := new(bytes.Buffer)
	_ = WriteString(buf, channel)
	buf.Write(data)

	return &Packet{
		ID:      C2SCustomPayload,
		Payload: buf.Bytes(),
	}

}

type KnownPack struct {
	NameSpace string
	Id        string
	Version   string
}

type SelectKnown struct {
	PacketLength int32
	Packs        []KnownPack
}

func CreateSelectKnownPacket(packs []KnownPack, packetID int32) *Packet {
	buf := new(bytes.Buffer)
	_ = WriteVarint(buf, int32(len(packs)))
	for _, pack := range packs {
		_ = WriteString(buf, pack.NameSpace)
		_ = WriteString(buf, pack.Id)
		_ = WriteString(buf, pack.Version)
	}

	return &Packet{
		ID:      packetID,
		Payload: buf.Bytes(),
	}
}

func CreateFinishConfigurationPacket(packetID int32) *Packet {
	return &Packet{
		ID:      packetID,
		Payload: []byte{},
	}
}
