package opencode

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/proxy"
)

// Free-tier traffic to the Zen /inference gateway is fingerprinted: the gate
// rejects requests whose TLS ClientHello does not look like the Bun runtime the
// official OpenCode client runs on. crypto/tls always sends Go's own hello, so
// the free path dials through uTLS with a captured Bun hello instead.
const bunHandshakeTimeout = 10 * time.Second

// bunClientHello is Bun's fetch ClientHello (Bun 1.4.x, JA4 t13d1713h1): no
// GREASE, no padding, http/1.1 as the only ALPN protocol and X25519MLKEM768 as
// the first key share. uTLS keeps per-handshake state in the extensions, so
// every connection builds a fresh spec.
func bunClientHello() *utls.ClientHelloSpec {
	return &utls.ClientHelloSpec{
		TLSVersMin: utls.VersionTLS12,
		TLSVersMax: utls.VersionTLS13,
		CipherSuites: []uint16{
			utls.TLS_AES_128_GCM_SHA256,
			utls.TLS_AES_256_GCM_SHA384,
			utls.TLS_CHACHA20_POLY1305_SHA256,
			utls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			utls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			utls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			utls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			utls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
			utls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
			utls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
			utls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			utls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			utls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			utls.TLS_RSA_WITH_AES_128_GCM_SHA256,
			utls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			utls.TLS_RSA_WITH_AES_128_CBC_SHA,
			utls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
		CompressionMethods: []uint8{0},
		Extensions: []utls.TLSExtension{
			&utls.SNIExtension{},
			&utls.ExtendedMasterSecretExtension{},
			&utls.RenegotiationInfoExtension{Renegotiation: utls.RenegotiateOnceAsClient},
			&utls.SupportedCurvesExtension{Curves: []utls.CurveID{utls.X25519MLKEM768, utls.X25519, utls.CurveP256, utls.CurveP384}},
			&utls.SupportedPointsExtension{SupportedPoints: []uint8{0}},
			&utls.SessionTicketExtension{},
			&utls.ALPNExtension{AlpnProtocols: []string{"http/1.1"}},
			&utls.StatusRequestExtension{},
			&utls.SignatureAlgorithmsExtension{SupportedSignatureAlgorithms: []utls.SignatureScheme{
				utls.ECDSAWithP256AndSHA256,
				utls.PSSWithSHA256,
				utls.PKCS1WithSHA256,
				utls.ECDSAWithP384AndSHA384,
				utls.PSSWithSHA384,
				utls.PKCS1WithSHA384,
				utls.PSSWithSHA512,
				utls.PKCS1WithSHA512,
				utls.PKCS1WithSHA1,
			}},
			&utls.SCTExtension{},
			&utls.KeyShareExtension{KeyShares: []utls.KeyShare{{Group: utls.X25519MLKEM768}, {Group: utls.X25519}}},
			&utls.PSKKeyExchangeModesExtension{Modes: []uint8{utls.PskModeDHE}},
			&utls.SupportedVersionsExtension{Versions: []uint16{utls.VersionTLS13, utls.VersionTLS12}},
		},
	}
}

// freeClients caches one Bun-fingerprinted client per configured proxy, so a
// channel's free traffic keeps its exit and its TLS sessions are never reused
// across exits.
var freeClients sync.Map

// freeTransportClient returns the client used for free-tier traffic on the Zen
// gateway. An empty proxy setting keeps honouring the process proxy
// environment, as every other relay client does.
func freeTransportClient(rawProxyURL string) (*http.Client, error) {
	key := strings.TrimSpace(rawProxyURL)
	if cached, ok := freeClients.Load(key); ok {
		return cached.(*http.Client), nil
	}
	proxyFn := http.ProxyFromEnvironment
	if key != "" {
		parsed, err := parseFreeProxyURL(key)
		if err != nil {
			return nil, err
		}
		proxyFn = func(*http.Request) (*url.URL, error) { return parsed, nil }
	}
	client := &http.Client{Transport: &bunRoundTripper{
		proxy:    proxyFn,
		dialer:   &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second},
		insecure: common.TLSInsecureSkipVerify,
	}}
	if cached, loaded := freeClients.LoadOrStore(key, client); loaded {
		return cached.(*http.Client), nil
	}
	return client, nil
}

// parseFreeProxyURL accepts the schemes the relay proxy setting supports plus
// the OpenCode pool markers, which are irrelevant here because every free
// request already uses a fresh connection.
func parseFreeProxyURL(raw string) (*url.URL, error) {
	value := strings.TrimSpace(raw)
	value = strings.TrimPrefix(value, "pool+")
	value = strings.TrimSuffix(value, "#pool")
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return nil, fmt.Errorf("invalid proxy url: %w", err)
	}
	switch parsed.Scheme {
	case "http", "https", "socks5", "socks5h":
		return parsed, nil
	default:
		return nil, fmt.Errorf("unsupported proxy scheme %q", parsed.Scheme)
	}
}

// bunRoundTripper sends one request over a fresh connection that completed the
// Bun TLS handshake. It delegates HTTP/1.1 framing to net/http: the request is
// handed to a single-use transport whose TLS dialer returns the connection the
// round tripper already established (through the configured proxy).
type bunRoundTripper struct {
	proxy    func(*http.Request) (*url.URL, error)
	dialer   *net.Dialer
	insecure bool
}

func (t *bunRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL == nil || req.URL.Host == "" {
		return nil, errors.New("bun transport: request has no URL")
	}
	proxyURL := (*url.URL)(nil)
	if t.proxy != nil {
		resolved, err := t.proxy(req)
		if err != nil {
			return nil, err
		}
		proxyURL = resolved
	}
	addr := req.URL.Host
	if req.URL.Port() == "" {
		port := "80"
		if req.URL.Scheme == "https" {
			port = "443"
		}
		addr = net.JoinHostPort(req.URL.Hostname(), port)
	}
	useTLS := req.URL.Scheme == "https"
	var conn net.Conn
	var err error
	if useTLS {
		conn, err = dialBunConn(req.Context(), t.dialer, proxyURL, addr, req.URL.Hostname(), t.insecure)
	} else {
		conn, err = dialFreeExit(req.Context(), t.dialer, proxyURL, addr)
	}
	if err != nil {
		return nil, err
	}
	var once sync.Once
	oneShot := func(context.Context, string, string) (net.Conn, error) {
		var handed net.Conn
		once.Do(func() { handed = conn })
		if handed == nil {
			return nil, errors.New("bun transport: connection already used")
		}
		return handed, nil
	}
	transport := &http.Transport{
		DisableKeepAlives: true,
		ForceAttemptHTTP2: false,
		// An empty (non-nil) map disables the HTTP/2 upgrade, matching the
		// http/1.1-only ALPN Bun offers.
		TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{},
	}
	if useTLS {
		transport.DialTLSContext = oneShot
	} else {
		transport.DialContext = oneShot
	}
	defer transport.CloseIdleConnections()
	resp, err := transport.RoundTrip(req)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return resp, nil
}

// dialBunConn opens a TCP connection (directly or through proxyURL) and
// completes the Bun TLS handshake when addr is https.
func dialBunConn(ctx context.Context, dialer *net.Dialer, proxyURL *url.URL, addr, serverName string, insecure bool) (net.Conn, error) {
	if dialer == nil {
		dialer = &net.Dialer{Timeout: 10 * time.Second}
	}
	conn, err := dialFreeExit(ctx, dialer, proxyURL, addr)
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	_ = conn.SetDeadline(time.Now().Add(bunHandshakeTimeout))
	tlsConn := utls.UClient(conn, &utls.Config{ServerName: serverName, InsecureSkipVerify: insecure}, utls.HelloCustom)
	if err := tlsConn.ApplyPreset(bunClientHello()); err != nil {
		stop()
		_ = conn.Close()
		return nil, err
	}
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		stop()
		_ = conn.Close()
		return nil, err
	}
	stop()
	_ = conn.SetDeadline(time.Time{})
	return tlsConn, nil
}

// dialFreeExit opens a raw connection to addr, tunnelling through proxyURL
// when one is configured.
func dialFreeExit(ctx context.Context, dialer *net.Dialer, proxyURL *url.URL, addr string) (net.Conn, error) {
	if proxyURL == nil {
		return dialer.DialContext(ctx, "tcp", addr)
	}
	switch proxyURL.Scheme {
	case "socks5", "socks5h":
		var auth *proxy.Auth
		if proxyURL.User != nil {
			password, _ := proxyURL.User.Password()
			auth = &proxy.Auth{User: proxyURL.User.Username(), Password: password}
		}
		dialer, err := proxy.SOCKS5("tcp", proxyHostPort(proxyURL), auth, dialer)
		if err != nil {
			return nil, err
		}
		if contextDialer, ok := dialer.(proxy.ContextDialer); ok {
			return contextDialer.DialContext(ctx, "tcp", addr)
		}
		return dialer.Dial("tcp", addr)
	default:
		conn, err := dialer.DialContext(ctx, "tcp", proxyHostPort(proxyURL))
		if err != nil {
			return nil, &net.OpError{Op: "proxyconnect", Net: "tcp", Err: err}
		}
		if proxyURL.Scheme == "https" {
			tlsConn := tls.Client(conn, &tls.Config{ServerName: proxyURL.Hostname()})
			if err = tlsConn.HandshakeContext(ctx); err == nil {
				conn = tlsConn
			}
		}
		if err == nil {
			conn, err = proxyConnect(conn, proxyURL.User, addr)
		}
		if err != nil {
			_ = conn.Close()
			return nil, &net.OpError{Op: "proxyconnect", Net: "tcp", Err: err}
		}
		return conn, nil
	}
}

func proxyHostPort(proxyURL *url.URL) string {
	port := proxyURL.Port()
	if port == "" {
		switch proxyURL.Scheme {
		case "https":
			port = "443"
		case "socks5", "socks5h":
			port = "1080"
		default:
			port = "80"
		}
	}
	return net.JoinHostPort(proxyURL.Hostname(), port)
}

// proxyConnect asks an HTTP proxy for a CONNECT tunnel to addr.
func proxyConnect(conn net.Conn, user *url.Userinfo, addr string) (net.Conn, error) {
	request := "CONNECT " + addr + " HTTP/1.1\r\nHost: " + addr + "\r\n"
	if user != nil {
		password, _ := user.Password()
		request += "Proxy-Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte(user.Username()+":"+password)) + "\r\n"
	}
	if _, err := io.WriteString(conn, request+"\r\n"); err != nil {
		return conn, err
	}
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	if err != nil {
		return conn, err
	}
	if resp.StatusCode/100 != 2 {
		return conn, fmt.Errorf("proxy refused CONNECT: %s", resp.Status)
	}
	if reader.Buffered() > 0 {
		return &bufferedConn{Conn: conn, reader: reader}, nil
	}
	return conn, nil
}

// bufferedConn is a net.Conn whose reads drain reader first, for a tunnel the
// proxy already wrote into while answering CONNECT.
type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) { return c.reader.Read(p) }
