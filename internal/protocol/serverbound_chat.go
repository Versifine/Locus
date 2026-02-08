package protocol

import (
	"bytes"
	"io"
)

type ChatMessage struct {
	Message   string
	Timestamp int64
	Salt      int64
	Offset    int32
	Checksum  byte
}

func ParseChatMessage(r io.Reader) (*ChatMessage, error) {
	var chat ChatMessage
	chatMessage, err := ReadString(r)
	if err != nil {
		return nil, err
	}
	chat.Message = chatMessage

	timestamp, err := NBTReadInt64(r)
	if err != nil {
		return nil, err
	}
	chat.Timestamp = timestamp
	salt, err := ReadInt64(r)
	if err != nil {
		return nil, err
	}
	chat.Salt = salt
	offset, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	chat.Offset = offset
	checksum, err := ReadByte(r)
	if err != nil {
		return nil, err
	}
	chat.Checksum = checksum
	return &chat, nil
}

type ChatCommand struct {
	Command string
}

func CreateSayChatCommandPacket(msg string) *Packet {
	command := "say " + msg
	payload := make([]byte, 0)
	writer := bytes.NewBuffer(payload)
	_ = WriteString(writer, command)
	return &Packet{
		ID:      C2SChatCommand,
		Payload: writer.Bytes(),
	}
}

func ParseChatCommand(r io.Reader) (*ChatCommand, error) {
	var chatCmd ChatCommand
	command, err := ReadString(r)
	if err != nil {
		return nil, err
	}
	chatCmd.Command = command
	return &chatCmd, nil
}

type ChatCommandSigned struct {
	Command                  string
	Timestamp                int64
	Salt                     int64
	ArgumentSignaturesLength int32
	ArgumentSignatures       []ArgumentSignature
	MessageCount             int32
	Checksum                 byte
}

type ArgumentSignature struct {
	Name      string
	Signature [256]byte
}

func ParseChatCommandSigned(r io.Reader) (*ChatCommandSigned, error) {
	var chatCmdSigned ChatCommandSigned
	command, err := ReadString(r)
	if err != nil {
		return nil, err
	}
	chatCmdSigned.Command = command
	timestamp, err := ReadInt64(r)
	if err != nil {
		return nil, err
	}
	chatCmdSigned.Timestamp = timestamp
	salt, err := ReadInt64(r)
	if err != nil {
		return nil, err
	}
	chatCmdSigned.Salt = salt
	argSigLength, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	chatCmdSigned.ArgumentSignaturesLength = argSigLength
	chatCmdSigned.ArgumentSignatures = make([]ArgumentSignature, argSigLength)
	for i := int32(0); i < argSigLength; i++ {
		name, err := ReadString(r)
		if err != nil {
			return nil, err
		}
		var signature [256]byte
		_, err = io.ReadFull(r, signature[:])
		if err != nil {
			return nil, err
		}
		chatCmdSigned.ArgumentSignatures[i] = ArgumentSignature{
			Name:      name,
			Signature: signature,
		}
	}
	messageCount, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	chatCmdSigned.MessageCount = messageCount
	checksum, err := ReadByte(r)
	if err != nil {
		return nil, err
	}
	chatCmdSigned.Checksum = checksum
	return &chatCmdSigned, nil
}
