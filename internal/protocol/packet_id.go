package protocol

const (
	// Handshaking (C→S)
	C2SHandshake = 0x00

	// Login (C→S)
	C2SLoginStart = 0x00

	// Login (S→C)
	S2CLoginSuccess   = 0x02
	S2CSetCompression = 0x03

	// Configuration (S→C)
	S2CFinishConfiguration = 0x03

	// Play (S→C)
	S2CPlayerChatMessage = 0x3f
	S2CSystemChatMessage = 0x77

	// Play (C→S)
	C2SChatCommand       = 0x06
	C2SChatCommandSigned = 0x07
	C2SChatMessage       = 0x08
)
