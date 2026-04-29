package outbound

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jnmproxy/jnmproxy/internal/cache"
)

type Dialer struct {
	Timeout time.Duration
	SingBox SingBoxDialer
}

type SingBoxDialer interface {
	DialContext(ctx context.Context, node cache.NodeSnapshot, targetAddress string) (net.Conn, error)
	Supports(protocol string) bool
	CloseNode(nodeID int64) error
}

func NewDialer(timeout time.Duration) *Dialer {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Dialer{Timeout: timeout}
}

func (dialer *Dialer) DialContext(ctx context.Context, node cache.NodeSnapshot, targetAddress string) (net.Conn, error) {
	switch strings.ToLower(node.Protocol) {
	case "direct":
		return dialer.DialDirectContext(ctx, targetAddress)
	case "http", "https":
		return dialer.dialHTTPProxy(ctx, node, targetAddress)
	case "socks5", "socks":
		return dialer.dialSOCKS5Proxy(ctx, node, targetAddress)
	default:
		if dialer.SingBox != nil && node.SingBoxStatus == "supported" && dialer.SingBox.Supports(node.Protocol) {
			return dialer.SingBox.DialContext(ctx, node, targetAddress)
		}
		return nil, fmt.Errorf("unsupported outbound protocol %q", node.Protocol)
	}
}

func (dialer *Dialer) DialDirectContext(ctx context.Context, targetAddress string) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, dialer.Timeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", targetAddress)
	if err != nil {
		return nil, fmt.Errorf("dial direct %s: %w", targetAddress, err)
	}
	return conn, nil
}

func (dialer *Dialer) dialHTTPProxy(ctx context.Context, node cache.NodeSnapshot, targetAddress string) (net.Conn, error) {
	proxyAddress := net.JoinHostPort(node.Server, strconv.Itoa(node.Port))
	ctx, cancel := context.WithTimeout(ctx, dialer.Timeout)
	defer cancel()

	var conn net.Conn
	var err error
	netDialer := &net.Dialer{}
	if strings.EqualFold(node.Protocol, "https") {
		tlsDialer := &tls.Dialer{NetDialer: netDialer}
		conn, err = tlsDialer.DialContext(ctx, "tcp", proxyAddress)
	} else {
		conn, err = netDialer.DialContext(ctx, "tcp", proxyAddress)
	}
	if err != nil {
		return nil, fmt.Errorf("dial http proxy %s: %w", proxyAddress, err)
	}

	if err := writeHTTPConnect(conn, targetAddress, nodeCredentials(node.RawConfigJSON)); err != nil {
		_ = conn.Close()
		return nil, err
	}

	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("read http proxy connect response: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = conn.Close()
		return nil, fmt.Errorf("http proxy connect failed: %s", resp.Status)
	}
	if reader.Buffered() > 0 {
		return &bufferedConn{Conn: conn, reader: reader}, nil
	}
	return conn, nil
}

func writeHTTPConnect(conn net.Conn, targetAddress string, credential proxyCredential) error {
	var builder strings.Builder
	builder.WriteString("CONNECT ")
	builder.WriteString(targetAddress)
	builder.WriteString(" HTTP/1.1\r\nHost: ")
	builder.WriteString(targetAddress)
	builder.WriteString("\r\nProxy-Connection: Keep-Alive\r\n")
	if credential.Username != "" || credential.Password != "" {
		token := base64.StdEncoding.EncodeToString([]byte(credential.Username + ":" + credential.Password))
		builder.WriteString("Proxy-Authorization: Basic ")
		builder.WriteString(token)
		builder.WriteString("\r\n")
	}
	builder.WriteString("\r\n")

	if _, err := io.WriteString(conn, builder.String()); err != nil {
		return fmt.Errorf("write http proxy connect: %w", err)
	}
	return nil
}

func (dialer *Dialer) dialSOCKS5Proxy(ctx context.Context, node cache.NodeSnapshot, targetAddress string) (net.Conn, error) {
	proxyAddress := net.JoinHostPort(node.Server, strconv.Itoa(node.Port))
	ctx, cancel := context.WithTimeout(ctx, dialer.Timeout)
	defer cancel()

	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", proxyAddress)
	if err != nil {
		return nil, fmt.Errorf("dial socks5 proxy %s: %w", proxyAddress, err)
	}
	if err := socks5Handshake(conn, targetAddress, nodeCredentials(node.RawConfigJSON)); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func socks5Handshake(conn net.Conn, targetAddress string, credential proxyCredential) error {
	methods := []byte{0x00}
	if credential.Username != "" || credential.Password != "" {
		methods = []byte{0x02}
	}
	if _, err := conn.Write([]byte{0x05, byte(len(methods))}); err != nil {
		return fmt.Errorf("write socks5 greeting: %w", err)
	}
	if _, err := conn.Write(methods); err != nil {
		return fmt.Errorf("write socks5 methods: %w", err)
	}
	response := []byte{0, 0}
	if _, err := io.ReadFull(conn, response); err != nil {
		return fmt.Errorf("read socks5 greeting: %w", err)
	}
	if response[0] != 0x05 {
		return errors.New("invalid socks5 version")
	}
	if response[1] == 0xff {
		return errors.New("socks5 proxy rejected authentication methods")
	}
	if response[1] == 0x02 {
		if err := socks5UsernamePasswordAuth(conn, credential); err != nil {
			return err
		}
	}

	host, portText, err := net.SplitHostPort(targetAddress)
	if err != nil {
		return fmt.Errorf("split target address: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return fmt.Errorf("parse target port: %w", err)
	}
	request := []byte{0x05, 0x01, 0x00}
	if ip := net.ParseIP(host); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			request = append(request, 0x01)
			request = append(request, v4...)
		} else {
			request = append(request, 0x04)
			request = append(request, ip.To16()...)
		}
	} else {
		if len(host) > 255 {
			return errors.New("socks5 target host is too long")
		}
		request = append(request, 0x03, byte(len(host)))
		request = append(request, []byte(host)...)
	}
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(port))
	request = append(request, portBytes...)

	if _, err := conn.Write(request); err != nil {
		return fmt.Errorf("write socks5 connect request: %w", err)
	}
	if err := readSOCKS5ConnectResponse(conn); err != nil {
		return err
	}
	return nil
}

func socks5UsernamePasswordAuth(conn net.Conn, credential proxyCredential) error {
	if len(credential.Username) > 255 || len(credential.Password) > 255 {
		return errors.New("socks5 credential is too long")
	}
	request := []byte{0x01, byte(len(credential.Username))}
	request = append(request, []byte(credential.Username)...)
	request = append(request, byte(len(credential.Password)))
	request = append(request, []byte(credential.Password)...)
	if _, err := conn.Write(request); err != nil {
		return fmt.Errorf("write socks5 auth: %w", err)
	}
	response := []byte{0, 0}
	if _, err := io.ReadFull(conn, response); err != nil {
		return fmt.Errorf("read socks5 auth: %w", err)
	}
	if response[1] != 0x00 {
		return errors.New("socks5 username/password authentication failed")
	}
	return nil
}

func readSOCKS5ConnectResponse(conn net.Conn) error {
	header := []byte{0, 0, 0, 0}
	if _, err := io.ReadFull(conn, header); err != nil {
		return fmt.Errorf("read socks5 connect response: %w", err)
	}
	if header[0] != 0x05 {
		return errors.New("invalid socks5 connect response version")
	}
	if header[1] != 0x00 {
		return fmt.Errorf("socks5 connect failed with reply %d", header[1])
	}
	var addressLength int
	switch header[3] {
	case 0x01:
		addressLength = net.IPv4len
	case 0x03:
		length := []byte{0}
		if _, err := io.ReadFull(conn, length); err != nil {
			return fmt.Errorf("read socks5 bind domain length: %w", err)
		}
		addressLength = int(length[0])
	case 0x04:
		addressLength = net.IPv6len
	default:
		return errors.New("invalid socks5 bind address type")
	}
	if addressLength > 0 {
		if _, err := io.CopyN(io.Discard, conn, int64(addressLength)); err != nil {
			return fmt.Errorf("read socks5 bind address: %w", err)
		}
	}
	if _, err := io.CopyN(io.Discard, conn, 2); err != nil {
		return fmt.Errorf("read socks5 bind port: %w", err)
	}
	return nil
}

type proxyCredential struct {
	Username string
	Password string
}

func nodeCredentials(rawConfigJSON string) proxyCredential {
	var config map[string]any
	if err := json.Unmarshal([]byte(rawConfigJSON), &config); err != nil {
		return proxyCredential{}
	}
	return proxyCredential{
		Username: stringValue(config["username"]),
		Password: stringValue(config["password"]),
	}
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (conn *bufferedConn) Read(p []byte) (int, error) {
	if conn.reader.Buffered() > 0 {
		return conn.reader.Read(p)
	}
	return conn.Conn.Read(p)
}
