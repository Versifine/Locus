package protocol

func CreatePlayerLoadedPacket() *Packet {
	return &Packet{
		ID:      C2SPlayerLoaded,
		Payload: []byte{},
	}
}
