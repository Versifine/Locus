package protocol

import (
	"bytes"
	"errors"
	"testing"
)

func TestReadPacket(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    *Packet
		wantErr bool
	}{
		{
			name: "正常数据包",
			input: []byte{
				0x06,                         // Length = 6
				0x01,                         // PacketID = 1
				0x48, 0x65, 0x6c, 0x6c, 0x6f, // "Hello"
			},
			want: &Packet{
				ID:      1,
				Payload: []byte("Hello"),
			},
			wantErr: false,
		},
		{
			name: "空Payload",
			input: []byte{
				0x01, // Length = 1
				0x00, // PacketID = 0
			},
			want: &Packet{
				ID:      0,
				Payload: []byte{},
			},
			wantErr: false,
		},
		{
			name: "大PacketID",
			input: []byte{
				0x03,       // Length = 3
				0x80, 0x01, // PacketID = 128 (VarInt编码)
				0x01, // Payload = [0x01]
			},
			want: &Packet{
				ID:      128,
				Payload: []byte{0x01},
			},
			wantErr: false,
		},
		{
			name: "大Payload",
			input: func() []byte {
				payload := bytes.Repeat([]byte("a"), 100)
				data := []byte{0x65, 0x01} // Length = 101, PacketID = 1
				return append(data, payload...)
			}(),
			want: &Packet{
				ID:      1,
				Payload: bytes.Repeat([]byte("a"), 100),
			},
			wantErr: false,
		},
		{
			name:    "空数据",
			input:   []byte{},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "只有Length不完整",
			input:   []byte{0x05},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Length声明大于实际数据",
			input: []byte{
				0x10, // Length = 16
				0x01, // PacketID = 1
				0x48, // 只有1字节payload
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Length为0",
			input: []byte{
				0x00, // Length = 0
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(tt.input)
			got, err := ReadPacket(r)

			if (err != nil) != tt.wantErr {
				t.Errorf("ReadPacket() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if got.ID != tt.want.ID {
					t.Errorf("ReadPacket() ID = %v, want %v", got.ID, tt.want.ID)
				}
				if !bytes.Equal(got.Payload, tt.want.Payload) {
					t.Errorf("ReadPacket() Payload = %v, want %v", got.Payload, tt.want.Payload)
				}
			}
		})
	}
}

func TestWritePacket(t *testing.T) {
	tests := []struct {
		name    string
		packet  *Packet
		want    []byte
		wantErr bool
	}{
		{
			name: "正常数据包",
			packet: &Packet{
				ID:      1,
				Payload: []byte("Hello"),
			},
			want: []byte{
				0x06,                         // Length = 6
				0x01,                         // PacketID = 1
				0x48, 0x65, 0x6c, 0x6c, 0x6f, // "Hello"
			},
			wantErr: false,
		},
		{
			name: "空Payload",
			packet: &Packet{
				ID:      0,
				Payload: []byte{},
			},
			want: []byte{
				0x01, // Length = 1
				0x00, // PacketID = 0
			},
			wantErr: false,
		},
		{
			name: "大PacketID",
			packet: &Packet{
				ID:      128,
				Payload: []byte{0x01},
			},
			want: []byte{
				0x03,       // Length = 3
				0x80, 0x01, // PacketID = 128 (VarInt编码)
				0x01, // Payload
			},
			wantErr: false,
		},
		{
			name: "负数PacketID",
			packet: &Packet{
				ID:      -1,
				Payload: []byte("test"),
			},
			wantErr: false, // VarInt支持负数
		},
		{
			name: "大Payload",
			packet: &Packet{
				ID:      1,
				Payload: bytes.Repeat([]byte("a"), 100),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := WritePacket(&buf, tt.packet)

			if (err != nil) != tt.wantErr {
				t.Errorf("WritePacket() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.want != nil && !bytes.Equal(buf.Bytes(), tt.want) {
				t.Errorf("WritePacket() = %v, want %v", buf.Bytes(), tt.want)
			}
		})
	}
}

func TestReadWritePacketRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		packet *Packet
	}{
		{
			name: "简单包",
			packet: &Packet{
				ID:      0x01,
				Payload: []byte("Hello, World!"),
			},
		},
		{
			name: "空Payload",
			packet: &Packet{
				ID:      0x00,
				Payload: []byte{},
			},
		},
		{
			name: "大PacketID",
			packet: &Packet{
				ID:      0x7FFFFFFF,
				Payload: []byte{0x01, 0x02, 0x03},
			},
		},
		{
			name: "大Payload",
			packet: &Packet{
				ID:      0x01,
				Payload: bytes.Repeat([]byte("x"), 1000),
			},
		},
		{
			name: "二进制Payload",
			packet: &Packet{
				ID:      0x0A,
				Payload: []byte{0x00, 0xFF, 0x12, 0x34, 0xAB, 0xCD},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 写入
			var buf bytes.Buffer
			if err := WritePacket(&buf, tt.packet); err != nil {
				t.Fatalf("WritePacket() error = %v", err)
			}

			// 读取
			got, err := ReadPacket(&buf)
			if err != nil {
				t.Fatalf("ReadPacket() error = %v", err)
			}

			// 验证
			if got.ID != tt.packet.ID {
				t.Errorf("ID mismatch: got %v, want %v", got.ID, tt.packet.ID)
			}
			if !bytes.Equal(got.Payload, tt.packet.Payload) {
				t.Errorf("Payload mismatch: got %v, want %v", got.Payload, tt.packet.Payload)
			}
		})
	}
}

func TestReadPacketError(t *testing.T) {
	t.Run("读取时出错", func(t *testing.T) {
		r := &errorReader{err: errors.New("read error")}
		_, err := ReadPacket(r)
		if err == nil {
			t.Error("ReadPacket() expected error, got nil")
		}
	})
}

func TestWritePacketError(t *testing.T) {
	t.Run("写入时出错", func(t *testing.T) {
		w := &errorWriter{err: errors.New("write error")}
		packet := &Packet{ID: 1, Payload: []byte("test")}
		err := WritePacket(w, packet)
		if err == nil {
			t.Error("WritePacket() expected error, got nil")
		}
	})
}

// 辅助类型：模拟读取错误
type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}

// 辅助类型：模拟写入错误
type errorWriter struct {
	err error
}

func (e *errorWriter) Write(p []byte) (n int, err error) {
	return 0, e.err
}

func TestMultiplePackets(t *testing.T) {
	// 测试连续读写多个包
	packets := []*Packet{
		{ID: 1, Payload: []byte("first")},
		{ID: 2, Payload: []byte("second")},
		{ID: 3, Payload: []byte("third")},
	}

	var buf bytes.Buffer

	// 写入所有包
	for _, p := range packets {
		if err := WritePacket(&buf, p); err != nil {
			t.Fatalf("WritePacket() error = %v", err)
		}
	}

	// 读取所有包
	for i, want := range packets {
		got, err := ReadPacket(&buf)
		if err != nil {
			t.Fatalf("ReadPacket() packet %d error = %v", i, err)
		}
		if got.ID != want.ID {
			t.Errorf("packet %d ID mismatch: got %v, want %v", i, got.ID, want.ID)
		}
		if !bytes.Equal(got.Payload, want.Payload) {
			t.Errorf("packet %d Payload mismatch", i)
		}
	}

	// 确保没有剩余数据
	if buf.Len() != 0 {
		t.Errorf("unexpected remaining data: %d bytes", buf.Len())
	}
}

func BenchmarkReadPacket(b *testing.B) {
	data := []byte{
		0x10, // Length = 16
		0x01, // PacketID = 1
	}
	data = append(data, bytes.Repeat([]byte("x"), 15)...)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := bytes.NewReader(data)
		ReadPacket(r)
	}
}

func BenchmarkWritePacket(b *testing.B) {
	packet := &Packet{
		ID:      1,
		Payload: bytes.Repeat([]byte("x"), 100),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		WritePacket(&buf, packet)
	}
}
