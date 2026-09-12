package core

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kingsley-ChenChen/MockCharles2/internal/certificates"
)

func (s *Service) CertificateInfo() (certificates.Info, error) {
	if s.caError != nil {
		return certificates.Info{}, s.caError
	}
	return s.ca.Info(), nil
}
func (s *Service) GenerateCertificate() (certificates.Info, error) {
	if s.caError != nil {
		return certificates.Info{}, s.caError
	}
	return s.ca.Generate()
}
func (s *Service) PublicCertificate() ([]byte, error) {
	if s.caError != nil {
		return nil, s.caError
	}
	return s.ca.PublicDER()
}

// Only this public certificate resource is exposed on the proxy port.
func (s *Service) servePublicCertificate(w http.ResponseWriter, r *http.Request) bool {
	short := r.URL.Hostname() == "mc.invalid" && (r.URL.Path == "/" || r.URL.Path == "" || r.URL.Path == "/ca.crt")
	legacy := r.URL.Hostname() == "mockcharles.invalid" && r.URL.Path == "/ca.crt"
	if (!short && !legacy) || r.URL.Scheme != "http" {
		return false
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", 405)
		return true
	}
	der, err := s.PublicCertificate()
	if err != nil {
		http.Error(w, "CA is not available; generate it in the desktop application", 404)
		return true
	}
	filename, err := certificates.PublicFilename(der)
	if err != nil {
		http.Error(w, "Invalid public certificate", 500)
		return true
	}
	w.Header().Set("Content-Type", "application/x-x509-ca-cert")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len(der)))
	if r.Method == "GET" {
		_, _ = w.Write(der)
	}
	return true
}

func normalizeTLSHost(host string) (string, error) {
	if host == "" || host != strings.TrimSpace(host) {
		return "", errors.New("HTTPS 域名不能为空或含空白")
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		return ip.Unmap().String(), nil
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if len(host) > 253 {
		return "", errors.New("HTTPS 域名过长")
	}
	for _, part := range strings.Split(host, ".") {
		if len(part) == 0 || len(part) > 63 || part[0] == '-' || part[len(part)-1] == '-' {
			return "", fmt.Errorf("无效 HTTPS 域名: %s", host)
		}
		for _, c := range part {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return "", fmt.Errorf("请填写单独域名或 IP，不含协议、端口或通配符: %s", host)
			}
		}
	}
	return host, nil
}
func connectTarget(authority string) (string, string, error) {
	host, port, err := net.SplitHostPort(authority)
	if err != nil {
		return "", "", errors.New("CONNECT target must include host and port")
	}
	host, err = normalizeTLSHost(host)
	if err != nil {
		return "", "", err
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", "", errors.New("invalid CONNECT port")
	}
	return host, net.JoinHostPort(host, strconv.Itoa(n)), nil
}
func intercepts(settings TLSSettings, host string) bool {
	if !settings.Enabled {
		return false
	}
	for _, candidate := range settings.Hosts {
		normalized, err := normalizeTLSHost(candidate)
		if err == nil && normalized == host {
			return true
		}
	}
	return false
}

func (s *Service) handleConnect(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	ip := clientIP(r)
	s.observeIP(ip)
	flow := Flow{ID: fmt.Sprint(flowSequence.Add(1)), IP: ip, Method: "CONNECT", URL: "https://" + r.Host, Start: started, Source: "tunnel", RequestHeaders: r.Header.Clone()}
	record := true
	defer func() {
		if record {
			flow.Duration = time.Since(started)
			s.addFlow(flow)
		}
	}()
	host, target, err := connectTarget(r.Host)
	if err != nil {
		flow.Status = 400
		flow.Error = err.Error()
		http.Error(w, err.Error(), 400)
		return
	}
	settings := s.Snapshot().Config.TLS
	decrypt := intercepts(settings, host)
	var cert *tls.Certificate
	if decrypt {
		if s.caError != nil {
			err = s.caError
		} else {
			cert, err = s.ca.Certificate(host)
		}
		if err != nil {
			flow.Source = "tls_error"
			flow.Status = 503
			flow.Error = err.Error()
			http.Error(w, "CA unavailable", 503)
			return
		}
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		flow.Status = 500
		flow.Error = "hijacking unavailable"
		http.Error(w, flow.Error, 500)
		return
	}
	var upstream net.Conn
	if !decrypt {
		upstream, err = (&net.Dialer{Timeout: 10 * time.Second}).DialContext(r.Context(), "tcp", target)
		if err != nil {
			flow.Status = 502
			flow.Error = err.Error()
			http.Error(w, "CONNECT upstream failed", 502)
			return
		}
		defer upstream.Close()
	}
	conn, rw, err := hijacker.Hijack()
	if err != nil {
		flow.Status = 500
		flow.Error = err.Error()
		return
	}
	defer conn.Close()
	s.mu.Lock()
	if s.server == nil || s.closed {
		s.mu.Unlock()
		flow.Status = 503
		flow.Error = "proxy stopped"
		return
	}
	ctx := s.connectContext
	s.connects[conn] = struct{}{}
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.connects, conn); s.mu.Unlock() }()
	if _, err = rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err == nil {
		err = rw.Flush()
	}
	if err != nil {
		flow.Status = 502
		flow.Error = err.Error()
		return
	}
	flow.Status = 200
	buffered := &bufferedConn{Conn: conn, reader: rw.Reader}
	if !decrypt {
		s.addFlow(flow)
		// Activity in either direction refreshes the idle timeout. Preserve TCP half-close.
		touch := func() {
			deadline := time.Now().Add(30 * time.Second)
			_ = conn.SetDeadline(deadline)
			_ = upstream.SetDeadline(deadline)
		}
		left, right := &activityConn{Conn: buffered, touch: touch}, &activityConn{Conn: upstream, touch: touch}
		done := make(chan error, 2)
		go func() { _, e := io.Copy(right, left); closeWrite(upstream); done <- e }()
		go func() { _, e := io.Copy(left, right); closeWrite(conn); done <- e }()
		for range 2 {
			select {
			case e := <-done:
				if e != nil {
					err = e
					conn.Close()
					upstream.Close()
				}
			case <-ctx.Done():
				err = ctx.Err()
				conn.Close()
				upstream.Close()
				<-done
			}
		}
		conn.Close()
		upstream.Close()
		if err != nil && !errors.Is(err, net.ErrClosed) {
			flow.Error = err.Error()
		}
		return
	}
	flow.Source = "tls_error"
	tlsConfig := &tls.Config{Certificates: []tls.Certificate{*cert}, MinVersion: tls.VersionTLS12, NextProtos: []string{"http/1.1"}, GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
		if hello.ServerName != "" {
			sni, e := normalizeTLSHost(hello.ServerName)
			if e != nil || sni != host {
				return nil, errors.New("TLS SNI differs from CONNECT target")
			}
		}
		return nil, nil
	}}
	secure := tls.Server(buffered, tlsConfig)
	handshakeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	err = secure.HandshakeContext(handshakeCtx)
	cancel()
	if err != nil {
		flow.Status = 502
		flow.Error = "设备 TLS 握手失败（请检查 CA 信任、域名或证书固定）: " + err.Error()
		return
	}
	record = false
	listener := newSingleListener(secure)
	inner := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		authority := req.Host
		if strings.HasPrefix(authority, "[") && strings.HasSuffix(authority, "]") {
			authority += ":443"
		} else if !strings.Contains(authority, ":") {
			authority = net.JoinHostPort(authority, "443")
		}
		_, innerTarget, e := connectTarget(authority)
		if e != nil || innerTarget != target || req.URL.IsAbs() || req.Method == "CONNECT" {
			http.Error(w, "HTTPS request authority differs from CONNECT target", 400)
			return
		}
		req.URL.Scheme = "https"
		req.URL.Host = req.Host
		req.RemoteAddr = r.RemoteAddr
		s.handleProxy(w, req)
	}), BaseContext: func(net.Listener) context.Context { return ctx }, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 30 * time.Second}
	defer inner.Close()
	_ = inner.Serve(listener)
}

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func closeWrite(conn net.Conn) {
	if c, ok := conn.(interface{ CloseWrite() error }); ok {
		_ = c.CloseWrite()
	}
}

type activityConn struct {
	net.Conn
	touch func()
}

func (c *activityConn) Read(p []byte) (int, error)  { c.touch(); return c.Conn.Read(p) }
func (c *activityConn) Write(p []byte) (int, error) { c.touch(); return c.Conn.Write(p) }

func (c *bufferedConn) Read(p []byte) (int, error) { return c.reader.Read(p) }

type singleListener struct {
	conn     net.Conn
	done     chan struct{}
	once     sync.Once
	accepted bool
}
type signalConn struct {
	net.Conn
	closeSignal func()
}

func (c *signalConn) Close() error { err := c.Conn.Close(); c.closeSignal(); return err }
func newSingleListener(conn net.Conn) *singleListener {
	l := &singleListener{done: make(chan struct{})}
	l.conn = &signalConn{Conn: conn, closeSignal: func() { l.once.Do(func() { close(l.done) }) }}
	return l
}
func (l *singleListener) Accept() (net.Conn, error) {
	if !l.accepted {
		l.accepted = true
		return l.conn, nil
	}
	<-l.done
	return nil, net.ErrClosed
}
func (l *singleListener) Close() error   { return l.conn.Close() }
func (l *singleListener) Addr() net.Addr { return l.conn.LocalAddr() }
