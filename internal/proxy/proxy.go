// Package proxy 负责 TCP 连接管理、双向 io.Copy
// 这是核心管道模块
package proxy

import (
	"io"
	"net"
)

// Pipe 在两个连接之间进行双向数据转发
func Pipe(client, server net.Conn) {
	go func() {
		defer server.Close()
		io.Copy(server, client)
	}()
	go func() {
		defer client.Close()
		io.Copy(client, server)
	}()
}
