package protocol

import "io"

type SystemChat struct {
	Content     NBTNode
	IsActionBar bool
}

func ParseSystemChat(r io.Reader) (*SystemChat, error) {
	var chat SystemChat
	content, err := ReadAnonymousNBT(r)
	if err != nil {
		return nil, err
	}
	chat.Content = *content

	isActionBarByte, err := ReadByte(r)
	if err != nil {
		return nil, err
	}
	chat.IsActionBar = isActionBarByte != 0
	return &chat, nil
}
