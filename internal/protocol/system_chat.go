package protocol

import (
	"io"
	"strings"
)

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

// FormatTextComponent extracts human-readable text from a Minecraft Text Component NBT node.
//
// Output examples:
//
//	"multiplayer.player.joined(Versifine)"
//	"death.attack.mob(Versifine, entity.minecraft.zombie)"
//	"Hello World"
func FormatTextComponent(node *NBTNode) string {
	if node == nil {
		return ""
	}
	switch node.Type {
	case TagString:
		return node.Value.(string)
	case TagCompound:
		return formatCompound(node.Value.(map[string]*NBTNode))
	default:
		return node.String()
	}
}

func formatCompound(c map[string]*NBTNode) string {
	var b strings.Builder

	// "text" field — literal text
	if text, ok := c["text"]; ok && text.Type == TagString {
		b.WriteString(text.Value.(string))
	}

	// "translate" field — translation key like "multiplayer.player.joined"
	if tr, ok := c["translate"]; ok && tr.Type == TagString {
		b.WriteString(tr.Value.(string))
		// "with" — arguments for the translation
		if with, ok := c["with"]; ok && with.Type == TagList {
			args := with.Value.([]*NBTNode)
			if len(args) > 0 {
				b.WriteByte('(')
				for i, arg := range args {
					if i > 0 {
						b.WriteString(", ")
					}
					b.WriteString(FormatTextComponent(arg))
				}
				b.WriteByte(')')
			}
		}
	}

	// "extra" field — appended children
	if extra, ok := c["extra"]; ok && extra.Type == TagList {
		for _, child := range extra.Value.([]*NBTNode) {
			b.WriteString(FormatTextComponent(child))
		}
	}

	return b.String()
}
