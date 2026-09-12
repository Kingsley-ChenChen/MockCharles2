package core

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestHTTPSTunnelWithoutAuthority(t *testing.T) {
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "encrypted-origin") }))
	defer origin.Close()
	s := openTestService(t)
	if err := s.StartProxy("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	target, _ := url.Parse(origin.URL)
	proxy, _ := url.Parse("http://" + s.Snapshot().ProxyAddress)
	roots := x509.NewCertPool()
	roots.AddCert(origin.Certificate())
	tr := &http.Transport{Proxy: http.ProxyURL(proxy), TLSClientConfig: &tls.Config{RootCAs: roots}}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: 5 * time.Second}
	resp, err := client.Get(target.String() + "/hello")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "encrypted-origin" {
		t.Fatalf("body %s", body)
	}
	if s.Snapshot().Config.Devices[0].IP != "127.0.0.1" {
		t.Fatal("lost original device")
	}
}

func TestHTTPSDecryptionUsesFixedRulesAndVerifiesUpstream(t *testing.T) {
	s := openTestService(t)
	if _, err := s.GenerateCertificate(); err != nil {
		t.Fatal(err)
	}
	info, err := s.CertificateInfo()
	if err != nil || !info.Available {
		t.Fatalf("info %v %v", info, err)
	}
	der, err := s.PublicCertificate()
	if err != nil {
		t.Fatal(err)
	}
	root, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(root)
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "origin") }))
	defer origin.Close()
	config := Config{TLS: TLSSettings{Enabled: true, Hosts: []string{"127.0.0.1"}}, Projects: []Project{{ID: "p", Name: "project", ResponseHeaders: map[string]string{"X-Project": "yes"}}}, Devices: []Device{{IP: "127.0.0.1", ProjectID: "p", LinkedRuleSetIDs: []string{"s"}, ActiveRuleSetID: "s"}}, Rules: []Rule{{ID: "r", ProjectID: "p", Name: "fixed", Method: "GET", URL: origin.URL + "/fixed", Status: 200, Body: "secure-mock", Enabled: true}}, RuleSets: []RuleSet{{ID: "s", ProjectID: "p", Name: "set", RuleIDs: []string{"r"}}}}
	if err = s.SaveConfig(config, 0); err != nil {
		t.Fatal(err)
	}
	if err = s.StartProxy("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	proxy, _ := url.Parse("http://" + s.Snapshot().ProxyAddress)
	tr := &http.Transport{Proxy: http.ProxyURL(proxy), TLSClientConfig: &tls.Config{RootCAs: roots}}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: 5 * time.Second}
	for range 2 {
		resp, err := client.Get(origin.URL + "/fixed")
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if string(body) != "secure-mock" || resp.Header.Get("X-Project") != "yes" {
			t.Fatalf("mock mismatch %s %v", body, resp.Header)
		}
	}
	resp, err := client.Get(origin.URL + "/real")
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 502 {
		t.Fatalf("untrusted upstream status %d", resp.StatusCode)
	}
	// Trust only this test origin; production transport keeps system trust and never skips verification.
	originRoots := x509.NewCertPool()
	originRoots.AddCert(origin.Certificate())
	s.transport.TLSClientConfig = &tls.Config{RootCAs: originRoots}
	resp, err = client.Get(origin.URL + "/real")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != "origin" || resp.Header.Get("X-Project") != "yes" {
		t.Fatalf("verified HTTPS forwarding failed: %d %s", resp.StatusCode, body)
	}
	if err = s.StopProxy(); err != nil {
		t.Fatal(err)
	}
	for _, f := range s.Flows() {
		if f.IP != "127.0.0.1" {
			t.Fatal("incorrect HTTPS device identity")
		}
	}
}

func TestHTTPSValidation(t *testing.T) {
	s := openTestService(t)
	for _, host := range []string{"*.example.com", "https://example.com", "example.com:443", "a/b", "", "a b"} {
		c := s.Snapshot().Config
		c.TLS = TLSSettings{Hosts: []string{host}}
		if s.SaveConfig(c, c.Revision) == nil {
			t.Fatalf("invalid host accepted: %q", host)
		}
	}
	c := s.Snapshot().Config
	c.TLS = TLSSettings{Enabled: true, Hosts: []string{"example.com"}}
	if s.SaveConfig(c, c.Revision) == nil {
		t.Fatal("enabled interception without CA")
	}
}

func TestStopClosesIdleConnectTunnel(t *testing.T) {
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer origin.Close()
	s := openTestService(t)
	if err := s.StartProxy("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	conn, err := net.Dial("tcp", s.Snapshot().ProxyAddress)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	host := strings.TrimPrefix(origin.URL, "https://")
	io.WriteString(conn, "CONNECT "+host+" HTTP/1.1\r\nHost: "+host+"\r\n\r\n")
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, &http.Request{Method: "CONNECT"})
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("connect %v %v", resp, err)
	}
	if err := s.StopProxy(); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, err = br.ReadByte(); err == nil {
		t.Fatal("tunnel still open")
	} else if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatal("stop did not close tunnel")
	}
}

func TestHTTPSDefaultPortAndAuthorityValidation(t *testing.T) {
	for _, host := range []string{"example.test", "::1"} {
		t.Run(host, func(t *testing.T) {
			s := openTestService(t)
			if _, err := s.GenerateCertificate(); err != nil {
				t.Fatal(err)
			}
			authority := host
			if strings.Contains(host, ":") {
				authority = "[" + host + "]"
			}
			c := Config{TLS: TLSSettings{Enabled: true, Hosts: []string{host}}, Projects: []Project{{ID: "p", Name: "p"}}, Devices: []Device{{IP: "127.0.0.1", ProjectID: "p", LinkedRuleSetIDs: []string{"s"}, ActiveRuleSetID: "s"}}, Rules: []Rule{{ID: "r", ProjectID: "p", Name: "r", Method: "GET", URL: "https://" + authority + "/fixed", Status: 200, Body: "default-port", Enabled: true}}, RuleSets: []RuleSet{{ID: "s", ProjectID: "p", Name: "s", RuleIDs: []string{"r"}}}}
			if err := s.SaveConfig(c, 0); err != nil {
				t.Fatal(err)
			}
			if err := s.StartProxy("127.0.0.1:0"); err != nil {
				t.Fatal(err)
			}
			der, _ := s.PublicCertificate()
			root, _ := x509.ParseCertificate(der)
			roots := x509.NewCertPool()
			roots.AddCert(root)
			for _, requestHost := range []string{authority, "unrelated.test"} {
				conn, err := net.Dial("tcp", s.Snapshot().ProxyAddress)
				if err != nil {
					t.Fatal(err)
				}
				conn.SetDeadline(time.Now().Add(5 * time.Second))
				target := net.JoinHostPort(host, "443")
				io.WriteString(conn, "CONNECT "+target+" HTTP/1.1\r\nHost: "+target+"\r\n\r\n")
				br := bufio.NewReader(conn)
				resp, err := http.ReadResponse(br, &http.Request{Method: "CONNECT"})
				if err != nil || resp.StatusCode != 200 {
					t.Fatalf("CONNECT: %v %v", resp, err)
				}
				secure := tls.Client(&bufferedConn{Conn: conn, reader: br}, &tls.Config{ServerName: host, RootCAs: roots})
				if err = secure.Handshake(); err != nil {
					t.Fatal(err)
				}
				io.WriteString(secure, "GET /fixed HTTP/1.1\r\nHost: "+requestHost+"\r\nConnection: close\r\n\r\n")
				got, err := http.ReadResponse(bufio.NewReader(secure), &http.Request{Method: "GET"})
				if err != nil {
					t.Fatal(err)
				}
				body, _ := io.ReadAll(got.Body)
				got.Body.Close()
				secure.Close()
				if requestHost == authority {
					if got.StatusCode != 200 || string(body) != "default-port" {
						t.Fatalf("default HTTPS rule failed: %d %s", got.StatusCode, body)
					}
				} else if got.StatusCode != 400 {
					t.Fatal("cross-authority request accepted")
				}
			}
		})
	}
}

func TestPublicCertificateDownloadAndUnavailableDisable(t *testing.T) {
	s := openTestService(t)
	if _, err := s.GenerateCertificate(); err != nil {
		t.Fatal(err)
	}
	if err := s.StartProxy("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	response, err := proxyClient(s).Get("http://mockcharles.invalid/ca.crt")
	if err != nil {
		t.Fatal(err)
	}
	der, _ := io.ReadAll(response.Body)
	response.Body.Close()
	cert, err := x509.ParseCertificate(der)
	if err != nil || !cert.IsCA {
		t.Fatalf("download is not public DER certificate: %v", err)
	}
	if response.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("certificate download cached")
	}
	if response.Header.Get("Content-Disposition") != `attachment; filename="`+cert.Subject.CommonName+`.crt"` {
		t.Fatal("download filename differs from certificate generation time")
	}
	short, err := proxyClient(s).Get("http://mc.invalid/")
	if err != nil {
		t.Fatal(err)
	}
	shortDER, _ := io.ReadAll(short.Body)
	short.Body.Close()
	if string(shortDER) != string(der) {
		t.Fatal("short certificate address differs")
	}
	// A missing/corrupt local CA must not prevent switching interception off.
	config := s.Snapshot().Config
	config.TLS = TLSSettings{Enabled: true, Hosts: []string{"example.test"}}
	if err = s.SaveConfig(config, config.Revision); err != nil {
		t.Fatal(err)
	}
	s.caError = io.ErrUnexpectedEOF
	config = s.Snapshot().Config
	config.TLS.Enabled = false
	if err = s.SaveConfig(config, config.Revision); err != nil {
		t.Fatalf("cannot disable after CA failure: %v", err)
	}
}

func TestConnectHalfClosePreservesReplyAndVisibleTunnel(t *testing.T) {
	origin, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer origin.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, e := origin.Accept()
		if e != nil {
			return
		}
		defer conn.Close()
		body, _ := io.ReadAll(conn)
		io.WriteString(conn, "reply:"+string(body))
	}()
	s := openTestService(t)
	if err = s.StartProxy("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	conn, err := net.Dial("tcp", s.Snapshot().ProxyAddress)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	target := origin.Addr().String()
	io.WriteString(conn, "CONNECT "+target+" HTTP/1.1\r\nHost: "+target+"\r\n\r\n")
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, &http.Request{Method: "CONNECT"})
	if err != nil || resp.StatusCode != 200 {
		t.Fatal("CONNECT failed")
	}
	deadline := time.Now().Add(time.Second)
	for len(s.Flows()) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(s.Flows()) != 1 || s.Flows()[0].Source != "tunnel" {
		t.Error("active tunnel is not visible")
	}
	io.WriteString(conn, "payload")
	conn.(*net.TCPConn).CloseWrite()
	reply, err := io.ReadAll(br)
	if err != nil {
		t.Fatal(err)
	}
	if string(reply) != "reply:payload" {
		t.Fatalf("half-close lost upstream reply: %q", reply)
	}
	<-done
}
