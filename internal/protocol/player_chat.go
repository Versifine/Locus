package protocol

import "io"

type PlayerChat struct {
	GlobalIndex         int32
	SenderUUID          UUID
	Index               int32
	PlainMessage        string
	Timestamp           int64
	Salt                int64
	PreviousMessages    *[]PreviousMessage
	UnsignedChatContent *NBTNode
	FilterType          int32
	FilterTypeMask      []int64
	Type                int32
	NetworkName         *NBTNode
	NetworkTargetName   *NBTNode
}

type PreviousMessage struct {
	Id        int32
	Signature *[256]byte
}

func ReadPreviousMessages(r io.Reader) (*[]PreviousMessage, error) {
	length, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	messages := make([]PreviousMessage, length)
	for i := int32(0); i < length; i++ {
		id, err := ReadVarint(r)
		if err != nil {
			return nil, err
		}
		if id < 0 {
			return nil, nil
		}
		var signature [256]byte
		_, err = io.ReadFull(r, signature[:])
		if err != nil {
			return nil, err
		}
		messages[i] = PreviousMessage{
			Id:        id,
			Signature: &signature,
		}
	}
	return &messages, nil
}
func ReadFilterTypeMask(r io.Reader) ([]int64, error) {
	length, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	mask := make([]int64, length)
	for i := int32(0); i < length; i++ {
		val, err := ReadInt64(r) // 协议规定是 i64，不是 VarLong
		if err != nil {
			return nil, err
		}
		mask[i] = val
	}
	return mask, nil
}

func ParsePlayerChat(r io.Reader) (*PlayerChat, error) {
	var chat PlayerChat
	globalIndex, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	chat.GlobalIndex = globalIndex
	senderUUID, err := ReadUUID(r)
	if err != nil {
		return nil, err
	}
	chat.SenderUUID = senderUUID
	index, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	chat.Index = index
	plainMessage, err := ReadString(r)
	if err != nil {
		return nil, err
	}
	chat.PlainMessage = plainMessage
	timestamp, err := ReadInt64(r)
	if err != nil {
		return nil, err
	}
	chat.Timestamp = timestamp
	salt, err := ReadInt64(r)
	if err != nil {
		return nil, err
	}
	chat.Salt = salt
	// PreviousMessages
	previousMessages, err := ReadPreviousMessages(r)
	if err != nil {
		return nil, err
	}
	chat.PreviousMessages = previousMessages
	// UnsignedChatContent (Optional)
	hasUnsignedContent, err := ReadBool(r)
	if err != nil {
		return nil, err
	}
	if hasUnsignedContent {
		unsignedChatContent, err := ReadAnonymousNBT(r)
		if err != nil {
			return nil, err
		}
		chat.UnsignedChatContent = unsignedChatContent
	}
	// FilterType
	filterType, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	chat.FilterType = filterType
	// FilterTypeMask
	if filterType == 2 {
		filterTypeMask, err := ReadFilterTypeMask(r)
		if err != nil {
			return nil, err
		}
		chat.FilterTypeMask = filterTypeMask
	} else {
		chat.FilterTypeMask = nil
	}
	// Type
	chatType, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	chat.Type = chatType
	// NetworkName
	networkName, err := ReadAnonymousNBT(r)
	if err != nil {
		return nil, err
	}
	chat.NetworkName = networkName
	// NetworkTargetName (Optional)
	hasTargetName, err := ReadBool(r)
	if err != nil {
		return nil, err
	}
	if hasTargetName {
		networkTargetName, err := ReadAnonymousNBT(r)
		if err != nil {
			return nil, err
		}
		chat.NetworkTargetName = networkTargetName
	}
	return &chat, nil
}
