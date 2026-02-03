package protocol

import (
	"bytes"
	"io"
)

type Packet struct {
	ID      int32
	Payload []byte
}

func ReadPacket(r io.Reader) (*Packet, error) {
	length, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}

	if length <= 0 {
		return nil, io.ErrUnexpectedEOF
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}

	buf := bytes.NewReader(data)
	id, err := ReadVarint(buf)
	if err != nil {
		return nil, err
	}
	payload, _ := io.ReadAll(buf)
	return &Packet{
		ID:      id,
		Payload: payload,
	}, nil
}

func WritePacket(w io.Writer, packet *Packet) error {
	idBytes := make([]byte, 5)
	buf := bytes.NewBuffer(idBytes[:0])
	if err := WriteVarint(buf, packet.ID); err != nil {
		return err
	}

	length := int32(len(packet.Payload)) + int32(buf.Len())

	if err := WriteVarint(w, length); err != nil {
		return err
	}
	if _, err := w.Write(buf.Bytes()); err != nil {
		return err
	}
	if _, err := w.Write(packet.Payload); err != nil {
		return err
	}
	return nil
}
