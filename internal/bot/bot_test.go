package bot

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"

	"github.com/Versifine/locus/internal/protocol"
)

func TestBotLoginAndConfig(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	bot := NewBot("localhost:25565", "TestBot")
	bot.conn = client
	bot.connState = protocol.NewConnState()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Use a channel to report completion
	done := make(chan struct{})
	var botErr error

	go func() {
		defer close(done)
		// We can't call Start because it calls Dial.
		// Instead we manually call login and handleConfiguration.
		if err := bot.login(); err != nil {
			botErr = err
			return
		}
		if err := bot.handleConfiguration(); err != nil {
			botErr = err
			return
		}
	}()

	// Server-side simulation
	go func() {
		threshold := -1
		// 1. Read Handshake
		p, err := protocol.ReadPacket(server, threshold)
		if err != nil {
			t.Errorf("Server: failed to read handshake: %v", err)
			return
		}
		if p.ID != protocol.C2SHandshake {
			t.Errorf("Server: expected handshake, got %x", p.ID)
			return
		}

		// 2. Read Login Start
		p, err = protocol.ReadPacket(server, threshold)
		if err != nil {
			t.Errorf("Server: failed to read login start: %v", err)
			return
		}
		if p.ID != protocol.C2SLoginStart {
			t.Errorf("Server: expected login start, got %x", p.ID)
			return
		}

		// 3. Send Login Success
		successPacket := &protocol.Packet{
			ID: protocol.S2CLoginSuccess,
			Payload: func() []byte {
				buf := new(bytes.Buffer)
				_ = protocol.WriteUUID(buf, protocol.GenerateOfflineUUID("TestBot"))
				_ = protocol.WriteString(buf, "TestBot")
				_ = protocol.WriteVarint(buf, 0) // properties
				return buf.Bytes()
			}(),
		}
		if err := protocol.WritePacket(server, successPacket, threshold); err != nil {
			t.Errorf("Server: failed to send login success: %v", err)
			return
		}

		// 4. Read Login Acknowledged
		p, err = protocol.ReadPacket(server, threshold)
		if err != nil {
			t.Errorf("Server: failed to read login ack: %v", err)
			return
		}
		if p.ID != protocol.C2SLoginAcknowledged {
			t.Errorf("Server: expected login ack, got %x", p.ID)
			return
		}

		// 5. Read Client Information (Config start)
		p, err = protocol.ReadPacket(server, threshold)
		if err != nil {
			t.Errorf("Server: failed to read client info: %v", err)
			return
		}
		if p.ID != protocol.C2SConfigClientInformation {
			t.Errorf("Server: expected client info, got %x", p.ID)
			return
		}

		// 6. Send Finish Configuration
		finishPacket := &protocol.Packet{
			ID:      protocol.S2CFinishConfiguration,
			Payload: []byte{},
		}
		if err := protocol.WritePacket(server, finishPacket, threshold); err != nil {
			t.Errorf("Server: failed to send finish config: %v", err)
			return
		}

		// 7. Read Finish Configuration Ack
		p, err = protocol.ReadPacket(server, threshold)
		if err != nil {
			t.Errorf("Server: failed to read finish config ack: %v", err)
			return
		}
		if p.ID != protocol.C2SFinishConfiguration {
			t.Errorf("Server: expected finish config ack, got %x", p.ID)
			return
		}
	}()

	select {
	case <-done:
		if botErr != nil {
			t.Fatalf("Bot failed: %v", botErr)
		}
	case <-ctx.Done():
		t.Fatal("Test timed out")
	}
}
