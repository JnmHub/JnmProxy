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
	Cache                 *cache.Store
	Dialer                *outbound.Dialer
	Stats                 *stats.Collector
	MaxAttemptsPerRequest int
	RequestLogger         *RequestLogger
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
	startedAt := time.Now()
	credential, _ := server.Cache.Credential(username)

	targetAddress, err := readSOCKS5ConnectRequest(conn)
	if err != nil {
		_ = writeSOCKS5Reply(conn, 0x01)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	selection, outConn, attempts, err := server.dialWithRetries(ctx, username, targetAddress)
	if err != nil {
		server.recordRequestLog(username, credential.ID, targetAddress, "failed", selection, attempts, err, startedAt)
		_ = writeSOCKS5Reply(conn, 0x05)
		return
	}
	defer outConn.Close()

	_ = conn.SetDeadline(time.Time{})
	if err := writeSOCKS5Reply(conn, 0x00); err != nil {
		reportSelectionFailure(server.Cache, selection, err.Error())
		recordStats(server.Stats, selection, 0, 0, false)
		server.recordRequestLog(username, credential.ID, targetAddress, "failed", selection, markLastAttemptFailed(attempts, selection.Node.ID, err), err, startedAt)
		return
	}
	uploadBytes, downloadBytes := pipeConnections(conn, outConn)
	reportSelectionSuccess(server.Cache, selection)
	recordStats(server.Stats, selection, uploadBytes, downloadBytes, true)
}

func (server *SOCKS5Server) dialWithRetries(ctx context.Context, username string, targetAddress string) (cache.Selection, net.Conn, []RequestAttempt, error) {
	excluded := make(map[int64]struct{})
	attempts := make([]RequestAttempt, 0, normalizeMaxAttempts(server.MaxAttemptsPerRequest))
	var lastErr error
	for attempt := 0; attempt < normalizeMaxAttempts(server.MaxAttemptsPerRequest); attempt++ {
		selection, err := server.Cache.SelectExcluding(username, excluded)
		if err != nil {
			if shouldFallbackDirect(err) && len(attempts) == 0 {
				return server.dialDirect(ctx, username, targetAddress, attempts)
			}
			lastErr = err
			break
		}
		excluded[selection.Node.ID] = struct{}{}
		outConn, err := server.Dialer.DialContext(ctx, selection.Node, targetAddress)
		if err == nil {
			attempts = append(attempts, RequestAttempt{NodeID: selection.Node.ID, NodeName: selection.Node.Name, Success: true})
			return selection, outConn, attempts, nil
		}
		lastErr = err
		attempts = append(attempts, RequestAttempt{NodeID: selection.Node.ID, NodeName: selection.Node.Name, Success: false, Error: err.Error()})
		reportSelectionFailure(server.Cache, selection, err.Error())
		recordStats(server.Stats, selection, 0, 0, false)
	}
	if lastErr == nil {
		lastErr = cache.ErrNoCandidateNodes
	}
	return cache.Selection{}, nil, attempts, lastErr
}

func (server *SOCKS5Server) dialDirect(ctx context.Context, username string, targetAddress string, attempts []RequestAttempt) (cache.Selection, net.Conn, []RequestAttempt, error) {
	selection, err := directSelection(server.Cache, username)
	if err != nil {
		return cache.Selection{}, nil, attempts, err
	}
	outConn, err := server.Dialer.DialDirectContext(ctx, targetAddress)
	if err != nil {
		attempts = append(attempts, directAttempt(false, err))
		recordStats(server.Stats, selection, 0, 0, false)
		return selection, nil, attempts, err
	}
	attempts = append(attempts, directAttempt(true, nil))
	return selection, outConn, attempts, nil
}

func (server *SOCKS5Server) recordRequestLog(username string, credentialID int64, targetAddress string, status string, selection cache.Selection, attempts []RequestAttempt, err error, startedAt time.Time) {
	if server.RequestLogger == nil {
		return
	}
	event := RequestLogEvent{
		EntryProtocol: "SOCKS5",
		CredentialID:  credentialID,
		Username:      username,
		TargetAddress: targetAddress,
		Status:        status,
		Attempts:      attempts,
		Duration:      time.Since(startedAt),
	}
	if selection.Direct {
		event.SelectedNodeName = directNodeName
	} else if selection.Node.ID != 0 {
		event.SelectedNodeID = selection.Node.ID
		event.SelectedNodeName = selection.Node.Name
	}
	if err != nil {
		event.Error = err.Error()
	}
	server.RequestLogger.Record(event)
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
