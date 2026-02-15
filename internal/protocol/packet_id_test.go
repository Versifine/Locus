package protocol

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type ProtocolMapping struct {
	Handshaking struct {
		ToClient struct {
			Types struct {
				Packet []any `json:"packet"`
			} `json:"types"`
		} `json:"toClient"`
		ToServer struct {
			Types struct {
				Packet []any `json:"packet"`
			} `json:"types"`
		} `json:"toServer"`
	} `json:"handshaking"`
	Login struct {
		ToClient struct {
			Types struct {
				Packet []any `json:"packet"`
			} `json:"types"`
		} `json:"toClient"`
		ToServer struct {
			Types struct {
				Packet []any `json:"packet"`
			} `json:"types"`
		} `json:"toServer"`
	} `json:"login"`
	Configuration struct {
		ToClient struct {
			Types struct {
				Packet []any `json:"packet"`
			} `json:"types"`
		} `json:"toClient"`
		ToServer struct {
			Types struct {
				Packet []any `json:"packet"`
			} `json:"types"`
		} `json:"toServer"`
	} `json:"configuration"`
	Play struct {
		ToClient struct {
			Types struct {
				Packet []any `json:"packet"`
			} `json:"types"`
		} `json:"toClient"`
		ToServer struct {
			Types struct {
				Packet []any `json:"packet"`
			} `json:"types"`
		} `json:"toServer"`
	} `json:"play"`
}

func getMappings(t *testing.T, packetType []any) map[string]int {
	if len(packetType) < 2 {
		return nil
	}
	container := packetType[1].([]any)
	for _, field := range container {
		fMap := field.(map[string]any)
		if fMap["name"] == "name" {
			typeDef := fMap["type"].([]any)
			mapper := typeDef[1].(map[string]any)
			mappings := mapper["mappings"].(map[string]any)
			result := make(map[string]int)
			for idStr, name := range mappings {
				var id int
				if len(idStr) > 2 && idStr[:2] == "0x" {
					_, err := json.Marshal(idStr) // Just to use json
					_ = err
					// Parse hex
					var hexVal int
					for i := 2; i < len(idStr); i++ {
						c := idStr[i]
						hexVal *= 16
						if c >= '0' && c <= '9' {
							hexVal += int(c - '0')
						} else if c >= 'a' && c <= 'f' {
							hexVal += int(c - 'a' + 10)
						} else if c >= 'A' && c <= 'F' {
							hexVal += int(c - 'A' + 10)
						}
					}
					id = hexVal
				} else {
					// Should not happen based on protocol.json structure
					continue
				}
				result[name.(string)] = id
			}
			return result
		}
	}
	return nil
}

func TestPacketIDsConsistency(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "1.21.11", "protocol.json"))
	if err != nil {
		t.Fatalf("Failed to read protocol.json: %v", err)
	}

	var protocol ProtocolMapping
	if err := json.Unmarshal(data, &protocol); err != nil {
		t.Fatalf("Failed to unmarshal protocol.json: %v", err)
	}

	t.Run("Handshaking ToServer", func(t *testing.T) {
		m := getMappings(t, protocol.Handshaking.ToServer.Types.Packet)
		checkID(t, m, "set_protocol", C2SHandshake)
	})

	t.Run("Login ToClient", func(t *testing.T) {
		m := getMappings(t, protocol.Login.ToClient.Types.Packet)
		checkID(t, m, "success", S2CLoginSuccess)
		checkID(t, m, "compress", S2CSetCompression)
	})

	t.Run("Login ToServer", func(t *testing.T) {
		m := getMappings(t, protocol.Login.ToServer.Types.Packet)
		checkID(t, m, "login_start", C2SLoginStart)
		checkID(t, m, "login_acknowledged", C2SLoginAcknowledged)
	})

	t.Run("Configuration ToClient", func(t *testing.T) {
		m := getMappings(t, protocol.Configuration.ToClient.Types.Packet)
		checkID(t, m, "finish_configuration", S2CFinishConfiguration)
		checkID(t, m, "keep_alive", S2CConfigKeepAlive)
		checkID(t, m, "select_known_packs", S2CSelectKnown)
	})

	t.Run("Configuration ToServer", func(t *testing.T) {
		m := getMappings(t, protocol.Configuration.ToServer.Types.Packet)
		checkID(t, m, "settings", C2SConfigClientInformation)
		checkID(t, m, "custom_payload", C2SCustomPayload)
		checkID(t, m, "select_known_packs", C2SSelectKnown)
		checkID(t, m, "finish_configuration", C2SFinishConfiguration)
		checkID(t, m, "keep_alive", C2SConfigKeepAlive)
	})

	t.Run("Play ToClient", func(t *testing.T) {
		m := getMappings(t, protocol.Play.ToClient.Types.Packet)
		checkID(t, m, "acknowledge_player_digging", S2CAcknowledgePlayerDigging)
		checkID(t, m, "tile_entity_data", S2CTileEntityData)
		checkID(t, m, "block_action", S2CBlockAction)
		checkID(t, m, "block_change", S2CBlockChange)
		checkID(t, m, "chunk_batch_finished", S2CChunkBatchFinished)
		checkID(t, m, "chunk_batch_start", S2CChunkBatchStart)
		checkID(t, m, "player_chat", S2CPlayerChatMessage)
		checkID(t, m, "system_chat", S2CSystemChatMessage)
		checkID(t, m, "keep_alive", S2CPlayKeepAlive)
		checkID(t, m, "unload_chunk", S2CUnloadChunk)
		checkID(t, m, "map_chunk", S2CLevelChunkWithLight)
		checkID(t, m, "login", S2CLogin)
		checkID(t, m, "multi_block_change", S2CMultiBlockChange)
		checkID(t, m, "position", S2CPlayerPosition)
		checkID(t, m, "respawn", S2CRespawn)
		checkID(t, m, "update_view_position", S2CUpdateViewPosition)
		checkID(t, m, "entity_metadata", S2CEntityMetadata)
	})

	t.Run("Play ToServer", func(t *testing.T) {
		m := getMappings(t, protocol.Play.ToServer.Types.Packet)
		checkID(t, m, "chat_command", C2SChatCommand)
		checkID(t, m, "chat_command_signed", C2SChatCommandSigned)
		checkID(t, m, "chat_message", C2SChatMessage)
		checkID(t, m, "chunk_batch_received", C2SChunkBatchReceived)
		checkID(t, m, "keep_alive", C2SPlayKeepAlive)
		checkID(t, m, "teleport_confirm", C2STeleportConfirm)
		checkID(t, m, "settings", C2SPlayClientInformation)
		checkID(t, m, "position", C2SPlayerPosition)
		checkID(t, m, "position_look", C2SPlayerPositionLook)
		checkID(t, m, "look", C2SPlayerRotation)
		checkID(t, m, "block_dig", C2SBlockDig)
		checkID(t, m, "entity_action", C2SEntityAction)
		checkID(t, m, "player_input", C2SPlayerInput)
	})
}

func checkID(t *testing.T, m map[string]int, name string, expected int) {
	if m == nil {
		t.Errorf("Mapping for %s is nil", name)
		return
	}
	id, ok := m[name]
	if !ok {
		t.Errorf("Packet %s not found in JSON mappings", name)
		return
	}
	if id != expected {
		t.Errorf("Packet ID mismatch for %s: JSON has 0x%02X, Go has 0x%02X", name, id, expected)
	}
}
