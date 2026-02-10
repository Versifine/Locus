package protocol

const (
	CurrentProtocolVersion = 774
	NextStateLogin         = 2

	// Handshaking (C→S)
	C2SHandshake = 0x00

	// Login (C→S)
	C2SLoginStart        = 0x00
	C2SLoginAcknowledged = 0x03

	// Login (S→C)
	S2CLoginSuccess   = 0x02
	S2CSetCompression = 0x03

	// Configuration (S→C)
	S2CFinishConfiguration = 0x03
	S2CSelectKnown         = 0x0E
	S2CConfigKeepAlive     = 0x04

	// Configuration (C→S)
	C2SConfigClientInformation = 0x00
	C2SCustomPayload           = 0x02
	C2SSelectKnown             = 0x07
	C2SFinishConfiguration     = 0x03
	C2SConfigKeepAlive         = 0x04

	// Play (S→C)
	S2CPlayerChatMessage = 0x3f
	S2CSystemChatMessage = 0x77
	S2CPlayKeepAlive     = 0x2b
	S2CPlayerPosition    = 0x46

	// Play (C→S)
	C2SChatCommand           = 0x06
	C2SChatCommandSigned     = 0x07
	C2SChatMessage           = 0x08
	C2SPlayKeepAlive         = 0x1b
	C2STeleportConfirm       = 0x00
	C2SPlayClientInformation = 0x0d
)
