package proxy

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/jnmproxy/jnmproxy/internal/cache"
	"github.com/jnmproxy/jnmproxy/internal/outbound"
	"github.com/jnmproxy/jnmproxy/internal/stats"
)

type SOCKS5Server struct {
	Cache  *cache.Store
	Dialer *outbound.Dialer
	Stats  *stats.Collector
}

func NewSOCKS5Server(store *cache.Store, dialer *outbound.Dialer) *SOCKS5Server {
	return &SOCKS5Server{Cache: store, Dialer: dialer}
}

func (server *SOCKS5Server) Serve(listener net.Listener) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		go server.handleConn(conn)
	}
}

func (server *SOCKS5Server) handleConn(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))

	username, password, err := readSOCKS5Auth(conn)
	if err != nil {
		return
	}
	if !verifyCachedCredential(server.Cache, username, password) {
		_, _ = conn.Write([]byte{0x01, 0x01})
		return
	}
	if _, err := conn.Write([]byte{0x01, 0x00}); err != nil {
		return
	}

	targetAddress, err := readSOCKS5ConnectRequest(conn)
	if err != nil {
		_ = writeSOCKS5Reply(conn, 0x01)
		return
	}

	selection, err := server.Cache.Select(username)
	if err != nil {
		_ = writeSOCKS5Reply(conn, 0x04)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	outConn, err := server.Dialer.DialContext(ctx, selection.Node, targetAddress)
	if err != nil {
		server.Cache.ReportNodeFailure(selection.Node.ID)
		recordStats(server.Stats, selection, 0, 0, false)
		_ = writeSOCKS5Reply(conn, 0x05)
		return
	}
	defer outConn.Close()

	_ = conn.SetDeadline(time.Time{})
	if err := writeSOCKS5Reply(conn, 0x00); err != nil {
		server.Cache.ReportNodeFailure(selection.Node.ID)
		recordStats(server.Stats, selection, 0, 0, false)
		return
	}
	uploadBytes, downloadBytes := pipeConnections(conn, outConn)
	server.Cache.ReportNodeSuccess(selection.Node.ID)
	recordStats(server.Stats, selection, uploadBytes, downloadBytes, true)
}

func readSOCKS5Auth(conn net.Conn) (string, string, error) {
	header := []byte{0, 0}
	if _, err := io.ReadFull(conn, header); err != nil {
		return "", "", fmt.Errorf("read socks5 greeting: %w", err)
	}
	if header[0] != 0x05 {
		return "", "", errors.New("invalid socks5 version")
	}
	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(conn, methods); err != nil {
		return "", "", fmt.Errorf("read socks5 methods: %w", err)
	}
	if !containsByte(methods, 0x02) {
		_, _ = conn.Write([]byte{0x05, 0xff})
		return "", "", errors.New("socks5 username/password auth required")
	}
	if _, err := conn.Write([]byte{0x05, 0x02}); err != nil {
		return "", "", fmt.Errorf("write socks5 auth method: %w", err)
	}

	authHeader := []byte{0, 0}
	if _, err := io.ReadFull(conn, authHeader); err != nil {
		return "", "", fmt.Errorf("read socks5 auth header: %w", err)
	}
	if authHeader[0] != 0x01 {
		_, _ = conn.Write([]byte{0x01, 0x01})
		return "", "", errors.New("invalid socks5 auth version")
	}
	username := make([]byte, int(authHeader[1]))
	if _, err := io.ReadFull(conn, username); err != nil {
		return "", "", fmt.Errorf("read socks5 username: %w", err)
	}
	passwordLength := []byte{0}
	if _, err := io.ReadFull(conn, passwordLength); err != nil {
		return "", "", fmt.Errorf("read socks5 password length: %w", err)
	}
	password := make([]byte, int(passwordLength[0]))
	if _, err := io.ReadFull(conn, password); err != nil {
		return "", "", fmt.Errorf("read socks5 password: %w", err)
	}
	return string(username), string(password), nil
}

func readSOCKS5ConnectRequest(conn net.Conn) (string, error) {
	header := []byte{0, 0, 0, 0}
	if _, err := io.ReadFull(conn, header); err != nil {
		return "", fmt.Errorf("read socks5 request header: %w", err)
	}
	if header[0] != 0x05 {
		return "", errors.New("invalid socks5 request version")
	}
	if header[1] != 0x01 {
		return "", errors.New("only socks5 connect command is supported")
	}

	host, err := readSOCKS5Address(conn, header[3])
	if err != nil {
		return "", err
	}
	portBytes := []byte{0, 0}
	if _, err := io.ReadFull(conn, portBytes); err != nil {
		return "", fmt.Errorf("read socks5 target port: %w", err)
	}
	port := binary.BigEndian.Uint16(portBytes)
	return net.JoinHostPort(host, strconv.Itoa(int(port))), nil
}

func readSOCKS5Address(conn net.Conn, addressType byte) (string, error) {
	switch addressType {
	case 0x01:
		ip := make([]byte, net.IPv4len)
		if _, err := io.ReadFull(conn, ip); err != nil {
			return "", fmt.Errorf("read socks5 ipv4 address: %w", err)
		}
		return net.IP(ip).String(), nil
	case 0x03:
		length := []byte{0}
		if _, err := io.ReadFull(conn, length); err != nil {
			return "", fmt.Errorf("read socks5 domain length: %w", err)
		}
		host := make([]byte, int(length[0]))
		if _, err := io.ReadFull(conn, host); err != nil {
			return "", fmt.Errorf("read socks5 domain: %w", err)
		}
		return string(host), nil
	case 0x04:
		ip := make([]byte, net.IPv6len)
		if _, err := io.ReadFull(conn, ip); err != nil {
			return "", fmt.Errorf("read socks5 ipv6 address: %w", err)
		}
		return net.IP(ip).String(), nil
	default:
		return "", errors.New("invalid socks5 address type")
	}
}

func writeSOCKS5Reply(conn net.Conn, reply byte) error {
	_, err := conn.Write([]byte{0x05, reply, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	return err
}

func containsByte(values []byte, target byte) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
