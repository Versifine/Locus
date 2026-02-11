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
	S2CSpawnEntity       = 0x01
	S2CSyncEntityPosition = 0x23
	S2CPlayKeepAlive     = 0x2b
	S2CRelEntityMove     = 0x33
	S2CEntityMoveLook    = 0x34
	S2CPlayerChatMessage = 0x3f
	S2CPlayerRemove      = 0x43
	S2CPlayerInfo        = 0x44
	S2CPlayerPosition    = 0x46
	S2CEntityDestroy     = 0x4b
	S2CExperience        = 0x65
	S2CUpdateHealth      = 0x66
	S2CUpdateTime        = 0x6f
	S2CSystemChatMessage = 0x77
	S2CEntityTeleport    = 0x7b

	// Play (C→S)
	C2STeleportConfirm       = 0x00
	C2SChatCommand           = 0x06
	C2SChatCommandSigned     = 0x07
	C2SChatMessage           = 0x08
	C2SClientCommand         = 0x0b
	C2SPlayClientInformation = 0x0d
	C2SPlayKeepAlive         = 0x1b
)
