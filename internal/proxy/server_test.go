package proxy

import (
	"bytes"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/Versifine/locus/internal/protocol"
)

func TestProxyForwardsDataToBackend(t *testing.T) {
	backendListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("启动后端 mock 服务器失败: %v", err)
	}
	t.Cleanup(func() {
		_ = backendListener.Close()
	})

	backendPacketCh := make(chan *protocol.Packet, 1)
	backendErrCh := make(chan error, 1)
	go func() {
		conn, err := backendListener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			backendErrCh <- err
			return
		}
		defer conn.Close()

		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
		packet, err := protocol.ReadPacket(conn, -1)
		if err != nil {
			backendErrCh <- err
			return
		}
		backendPacketCh <- packet
	}()

	proxyAddr, proxyDone := startProxyOnceForTest(t, backendListener.Addr().String())

	clientConn, err := net.DialTimeout("tcp", proxyAddr, 2*time.Second)
	if err != nil {
		t.Fatalf("客户端连接 proxy 失败: %v", err)
	}
	t.Cleanup(func() {
		_ = clientConn.Close()
	})

	_ = clientConn.SetDeadline(time.Now().Add(2 * time.Second))
	want := &protocol.Packet{
		ID:      0x01, // 避免触发 Handshake 特殊解析逻辑
		Payload: []byte("hello-backend"),
	}
	if err := protocol.WritePacket(clientConn, want, -1); err != nil {
		t.Fatalf("客户端写入数据到 proxy 失败: %v", err)
	}

	select {
	case err := <-backendErrCh:
		t.Fatalf("后端读取失败: %v", err)
	case got := <-backendPacketCh:
		if got.ID != want.ID {
			t.Errorf("后端收到 Packet.ID = %d, 期望 %d", got.ID, want.ID)
		}
		if !bytes.Equal(got.Payload, want.Payload) {
			t.Errorf("后端收到 Payload = %q, 期望 %q", got.Payload, want.Payload)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("等待后端收到转发数据超时")
	}

	_ = clientConn.Close()

	select {
	case <-proxyDone:
	case <-time.After(2 * time.Second):
		t.Fatal("等待 proxy 清理连接超时")
	}
}

func startProxyOnceForTest(t *testing.T, backendAddr string) (string, <-chan struct{}) {
	t.Helper()

	proxyListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("启动 proxy 监听失败: %v", err)
	}
	t.Cleanup(func() {
		_ = proxyListener.Close()
	})

	server := NewServer(proxyListener.Addr().String(), backendAddr)
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := proxyListener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		server.handleConnection(conn)
	}()

	return proxyListener.Addr().String(), done
}
