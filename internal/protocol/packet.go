package protocol

import (
	"bytes"
	"compress/zlib"
	"errors"
	"io"
)

const MaxPacketSize = 2097152 // 2MB

type Packet struct {
	ID      int32
	Payload []byte
}

func ReadPacket(r io.Reader, threshold int) (*Packet, error) {
	// 1. Read Packet Length
	packetLen, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}

	if packetLen <= 0 {
		return nil, ErrInvalidPacket
	}
	if packetLen > MaxPacketSize {
		return nil, ErrPacketTooLarge
	}

	// 2. Read entire packet data
	data := make([]byte, packetLen)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, errors.Join(ErrInvalidPacket, err)
	}

	var rawDataReader io.Reader = bytes.NewReader(data)

	// 3. Handle Compression
	if threshold >= 0 {
		dataLen, err := ReadVarint(rawDataReader)
		if err != nil {
			return nil, err
		}

		if dataLen != 0 {
			// Compressed
			// Verify dataLen makes sense (optional)
			// Decompress remaining data
			z, err := zlib.NewReader(rawDataReader)
			if err != nil {
				return nil, err
			}
			defer z.Close()

			// Safety limit for decompressed data
			decompressed := make([]byte, dataLen)
			if _, err := io.ReadFull(z, decompressed); err != nil {
				return nil, err
			}
			rawDataReader = bytes.NewReader(decompressed)
		}
		// If dataLen == 0, the remaining data is uncompressed [ID] [Payload]
		// rawDataReader is already pointing to ID
	}

	// 4. Parse ID and Payload
	id, err := ReadVarint(rawDataReader)
	if err != nil {
		return nil, err
	}
	payload, _ := io.ReadAll(rawDataReader)
	return &Packet{
		ID:      id,
		Payload: payload,
	}, nil
}

func WritePacket(w io.Writer, packet *Packet, threshold int) error {
	// 1. Prepare raw [ID] [Payload]
	idBytes := make([]byte, 5)
	idBuf := bytes.NewBuffer(idBytes[:0])
	if err := WriteVarint(idBuf, packet.ID); err != nil {
		return err
	}

	// Calculate size of uncompressed payload
	uncompressedLen := idBuf.Len() + len(packet.Payload)

	var packetData []byte
	var dataLength int32 = 0 // 0 means uncompressed

	if threshold >= 0 && uncompressedLen >= threshold {
		// Need to compress
		var buf bytes.Buffer
		z := zlib.NewWriter(&buf)
		if _, err := z.Write(idBuf.Bytes()); err != nil {
			return err
		}
		if _, err := z.Write(packet.Payload); err != nil {
			return err
		}
		if err := z.Close(); err != nil {
			return err
		}
		packetData = buf.Bytes()
		dataLength = int32(uncompressedLen)
	} else {
		// No compression needed (or threshold < 0)
		packetData = append(idBuf.Bytes(), packet.Payload...)
		// dataLength remains 0 if threshold >= 0
		// If threshold < 0, we don't write dataLength at all
	}

	// 2. Write Packet Header + Data

	if threshold >= 0 {
		// Format: [Packet Length] [Data Length] [Data]
		dataLenBytes := make([]byte, 5)
		dataLenBuf := bytes.NewBuffer(dataLenBytes[:0])
		if err := WriteVarint(dataLenBuf, dataLength); err != nil {
			return err
		}

		totalLen := dataLenBuf.Len() + len(packetData)
		if err := WriteVarint(w, int32(totalLen)); err != nil {
			return err
		}
		if _, err := w.Write(dataLenBuf.Bytes()); err != nil {
			return err
		}
		if _, err := w.Write(packetData); err != nil {
			return err
		}
	} else {
		// Format: [Length] [ID] [Payload]
		// packetData already contains [ID] [Payload]
		if err := WriteVarint(w, int32(len(packetData))); err != nil {
			return err
		}
		if _, err := w.Write(packetData); err != nil {
			return err
		}
	}

	return nil
}
