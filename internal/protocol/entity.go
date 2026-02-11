package protocol

import (
	"bytes"
	"io"
)

// SpawnEntity represents the S2C Spawn Entity packet (0x01).
// We only extract fields needed for entity tracking.
type SpawnEntity struct {
	EntityID   int32
	ObjectUUID UUID
	Type       int32
	X          float64
	Y          float64
	Z          float64
}

func ParseSpawnEntity(r io.Reader) (*SpawnEntity, error) {
	entityID, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	objectUUID, err := ReadUUID(r)
	if err != nil {
		return nil, err
	}
	entityType, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	x, err := ReadDouble(r)
	if err != nil {
		return nil, err
	}
	y, err := ReadDouble(r)
	if err != nil {
		return nil, err
	}
	z, err := ReadDouble(r)
	if err != nil {
		return nil, err
	}
	// Skip remaining fields (velocity, pitch, yaw, headPitch, objectData)
	// We only need entityId, UUID, type, and position for tracking.
	return &SpawnEntity{
		EntityID:   entityID,
		ObjectUUID: objectUUID,
		Type:       entityType,
		X:          x,
		Y:          y,
		Z:          z,
	}, nil
}

// EntityDestroy represents the S2C Remove Entities packet (0x4b).
type EntityDestroy struct {
	EntityIDs []int32
}

func ParseEntityDestroy(r io.Reader) (*EntityDestroy, error) {
	count, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	ids := make([]int32, count)
	for i := int32(0); i < count; i++ {
		id, err := ReadVarint(r)
		if err != nil {
			return nil, err
		}
		ids[i] = id
	}
	return &EntityDestroy{EntityIDs: ids}, nil
}

// RelEntityMove represents the S2C Entity Relative Move packet (0x33).
// Delta values are fixed-point: actual offset = delta / 4096.0
type RelEntityMove struct {
	EntityID int32
	DX       int16
	DY       int16
	DZ       int16
	OnGround bool
}

func ParseRelEntityMove(r io.Reader) (*RelEntityMove, error) {
	entityID, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	dx, err := ReadInt16(r)
	if err != nil {
		return nil, err
	}
	dy, err := ReadInt16(r)
	if err != nil {
		return nil, err
	}
	dz, err := ReadInt16(r)
	if err != nil {
		return nil, err
	}
	onGround, err := ReadBool(r)
	if err != nil {
		return nil, err
	}
	return &RelEntityMove{
		EntityID: entityID,
		DX:       dx,
		DY:       dy,
		DZ:       dz,
		OnGround: onGround,
	}, nil
}

// DeltaX returns the actual X offset in blocks.
func (m *RelEntityMove) DeltaX() float64 { return float64(m.DX) / 4096.0 }

// DeltaY returns the actual Y offset in blocks.
func (m *RelEntityMove) DeltaY() float64 { return float64(m.DY) / 4096.0 }

// DeltaZ returns the actual Z offset in blocks.
func (m *RelEntityMove) DeltaZ() float64 { return float64(m.DZ) / 4096.0 }

// EntityMoveLook represents the S2C Entity Position and Rotation packet (0x34).
// Delta values are fixed-point: actual offset = delta / 4096.0
type EntityMoveLook struct {
	EntityID int32
	DX       int16
	DY       int16
	DZ       int16
	Yaw      int8
	Pitch    int8
	OnGround bool
}

func ParseEntityMoveLook(r io.Reader) (*EntityMoveLook, error) {
	entityID, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	dx, err := ReadInt16(r)
	if err != nil {
		return nil, err
	}
	dy, err := ReadInt16(r)
	if err != nil {
		return nil, err
	}
	dz, err := ReadInt16(r)
	if err != nil {
		return nil, err
	}
	yawByte, err := ReadByte(r)
	if err != nil {
		return nil, err
	}
	pitchByte, err := ReadByte(r)
	if err != nil {
		return nil, err
	}
	onGround, err := ReadBool(r)
	if err != nil {
		return nil, err
	}
	return &EntityMoveLook{
		EntityID: entityID,
		DX:       dx,
		DY:       dy,
		DZ:       dz,
		Yaw:      int8(yawByte),
		Pitch:    int8(pitchByte),
		OnGround: onGround,
	}, nil
}

// DeltaX returns the actual X offset in blocks.
func (m *EntityMoveLook) DeltaX() float64 { return float64(m.DX) / 4096.0 }

// DeltaY returns the actual Y offset in blocks.
func (m *EntityMoveLook) DeltaY() float64 { return float64(m.DY) / 4096.0 }

// DeltaZ returns the actual Z offset in blocks.
func (m *EntityMoveLook) DeltaZ() float64 { return float64(m.DZ) / 4096.0 }

// EntityTeleport represents the S2C Entity Teleport packet (0x7b).
type EntityTeleport struct {
	EntityID int32
	X        float64
	Y        float64
	Z        float64
	Yaw      int8
	Pitch    int8
	OnGround bool
}

func ParseEntityTeleport(r io.Reader) (*EntityTeleport, error) {
	entityID, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	x, err := ReadDouble(r)
	if err != nil {
		return nil, err
	}
	y, err := ReadDouble(r)
	if err != nil {
		return nil, err
	}
	z, err := ReadDouble(r)
	if err != nil {
		return nil, err
	}
	yawByte, err := ReadByte(r)
	if err != nil {
		return nil, err
	}
	pitchByte, err := ReadByte(r)
	if err != nil {
		return nil, err
	}
	onGround, err := ReadBool(r)
	if err != nil {
		return nil, err
	}
	return &EntityTeleport{
		EntityID: entityID,
		X:        x,
		Y:        y,
		Z:        z,
		Yaw:      int8(yawByte),
		Pitch:    int8(pitchByte),
		OnGround: onGround,
	}, nil
}

// SyncEntityPosition represents the S2C Synchronize Entity Position packet (0x23).
type SyncEntityPosition struct {
	EntityID int32
	X        float64
	Y        float64
	Z        float64
	DX       float64
	DY       float64
	DZ       float64
	Yaw      float32
	Pitch    float32
	OnGround bool
}

func ParseSyncEntityPosition(r io.Reader) (*SyncEntityPosition, error) {
	entityID, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	x, err := ReadDouble(r)
	if err != nil {
		return nil, err
	}
	y, err := ReadDouble(r)
	if err != nil {
		return nil, err
	}
	z, err := ReadDouble(r)
	if err != nil {
		return nil, err
	}
	dx, err := ReadDouble(r)
	if err != nil {
		return nil, err
	}
	dy, err := ReadDouble(r)
	if err != nil {
		return nil, err
	}
	dz, err := ReadDouble(r)
	if err != nil {
		return nil, err
	}
	yaw, err := ReadFloat(r)
	if err != nil {
		return nil, err
	}
	pitch, err := ReadFloat(r)
	if err != nil {
		return nil, err
	}
	onGround, err := ReadBool(r)
	if err != nil {
		return nil, err
	}
	return &SyncEntityPosition{
		EntityID: entityID,
		X:        x,
		Y:        y,
		Z:        z,
		DX:       dx,
		DY:       dy,
		DZ:       dz,
		Yaw:      yaw,
		Pitch:    pitch,
		OnGround: onGround,
	}, nil
}

// ParseEntityMetadataItemSlot extracts an item ID from Item entity metadata (index 8, type 7).
// It returns found=false when metadata cannot be parsed safely or does not contain a usable item slot.
func ParseEntityMetadataItemSlot(r io.Reader) (entityID int32, itemID int32, found bool, err error) {
	entityID, err = ReadVarint(r)
	if err != nil {
		return 0, 0, false, err
	}

	for {
		key, err := ReadByte(r)
		if err != nil {
			return entityID, 0, false, err
		}
		if key == 0xFF {
			return entityID, 0, false, nil
		}

		metaType, err := ReadVarint(r)
		if err != nil {
			return entityID, 0, false, err
		}

		if key == 8 && metaType == 7 {
			itemCount, err := ReadVarint(r)
			if err != nil {
				return entityID, 0, false, err
			}
			if itemCount <= 0 {
				return entityID, 0, false, nil
			}
			itemID, err := ReadVarint(r)
			if err != nil {
				return entityID, 0, false, err
			}
			// Validate Slot header shape, component payload is intentionally ignored.
			if _, err := ReadVarint(r); err != nil {
				return entityID, 0, false, err
			}
			if _, err := ReadVarint(r); err != nil {
				return entityID, 0, false, err
			}
			return entityID, itemID, true, nil
		}

		skipped, err := skipEntityMetadataValue(r, metaType)
		if err != nil {
			return entityID, 0, false, err
		}
		if !skipped {
			// Unknown/unsupported metadata type: stop safely without crashing.
			return entityID, 0, false, nil
		}
	}
}

func skipEntityMetadataValue(r io.Reader, metaType int32) (bool, error) {
	switch metaType {
	case 0: // Byte
		return true, discardBytes(r, 1)
	case 1: // VarInt
		_, err := ReadVarint(r)
		return true, err
	case 2: // VarLong
		_, err := ReadVarLong(r)
		return true, err
	case 3: // Float
		return true, discardBytes(r, 4)
	case 4: // String
		_, err := ReadString(r)
		return true, err
	case 5: // Component (anonymousNbt)
		_, err := ReadAnonymousNBT(r)
		return true, err
	case 6: // Optional Component
		hasValue, err := ReadBool(r)
		if err != nil {
			return true, err
		}
		if !hasValue {
			return true, nil
		}
		_, err = ReadAnonymousNBT(r)
		return true, err
	case 7: // Slot
		// Generic Slot skipping requires full slot-component decoding.
		// We only support the target slot (key=8,type=7) in ParseEntityMetadataItemSlot.
		return false, nil
	case 8: // Boolean
		_, err := ReadBool(r)
		return true, err
	case 9: // Rotations
		return true, discardBytes(r, 12)
	case 10: // Block Position (packed i64)
		return true, discardBytes(r, 8)
	case 11: // Optional Block Position
		hasValue, err := ReadBool(r)
		if err != nil {
			return true, err
		}
		if !hasValue {
			return true, nil
		}
		return true, discardBytes(r, 8)
	case 12: // Direction
		_, err := ReadVarint(r)
		return true, err
	case 13: // Optional UUID
		hasValue, err := ReadBool(r)
		if err != nil {
			return true, err
		}
		if !hasValue {
			return true, nil
		}
		return true, discardBytes(r, 16)
	case 14: // Block State
		_, err := ReadVarint(r)
		return true, err
	case 15: // Optional Block State
		_, err := ReadVarint(r)
		return true, err
	case 19: // Optional Unsigned Int (optvarint)
		_, err := ReadVarint(r)
		return true, err
	case 20: // Pose
		_, err := ReadVarint(r)
		return true, err
	case 21: // Cat Variant
		_, err := ReadVarint(r)
		return true, err
	case 22: // Cow Variant
		_, err := ReadVarint(r)
		return true, err
	case 23: // Wolf Variant
		_, err := ReadVarint(r)
		return true, err
	case 24: // Wolf Sound Variant
		_, err := ReadVarint(r)
		return true, err
	case 25: // Frog Variant
		_, err := ReadVarint(r)
		return true, err
	case 26: // Pig Variant
		_, err := ReadVarint(r)
		return true, err
	case 27: // Chicken Variant
		_, err := ReadVarint(r)
		return true, err
	case 28: // Zombie Nautilus Variant
		_, err := ReadVarint(r)
		return true, err
	case 31: // Sniffer State
		_, err := ReadVarint(r)
		return true, err
	case 32: // Armadillo State
		_, err := ReadVarint(r)
		return true, err
	case 33: // Copper Golem State
		_, err := ReadVarint(r)
		return true, err
	case 34: // Weathering Copper Golem State
		_, err := ReadVarint(r)
		return true, err
	case 35: // Vector3 (3 x float32)
		return true, discardBytes(r, 12)
	case 36: // Quaternion (4 x float32)
		return true, discardBytes(r, 16)
	case 38: // Humanoid Arm
		_, err := ReadVarint(r)
		return true, err
	default:
		return false, nil
	}
}

func discardBytes(r io.Reader, n int64) error {
	_, err := io.CopyN(io.Discard, r, n)
	return err
}

// CreateClientCommandPacket creates a C2S Client Command packet.
// actionId 0 = Perform Respawn.
func CreateClientCommandPacket(actionID int32) *Packet {
	buf := new(bytes.Buffer)
	_ = WriteVarint(buf, actionID)
	return &Packet{
		ID:      C2SClientCommand,
		Payload: buf.Bytes(),
	}
}
