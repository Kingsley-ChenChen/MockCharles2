package core

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestCertificateSetupWithoutCapture(t *testing.T) {
	s := openTestService(t)
	if _, err := s.GenerateCertificate(); err != nil {
		t.Fatal(err)
	}
	info, err := s.PrepareConnection("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := "127.0.0.1:" + info.Port
	p, _ := url.Parse("http://" + address)
	tr := &http.Transport{Proxy: http.ProxyURL(p)}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: time.Second * 3}
	check := func(target string, status int) {
		t.Helper()
		resp, err := client.Get(target)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != status {
			t.Fatalf("%s: status %d", target, resp.StatusCode)
		}
	}
	check("http://mc.invalid", 200)
	check("http://example.test", 503)
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	if _, err := s.PrepareConnection(occupied.Addr().String()); err == nil {
		t.Fatal("occupied port accepted")
	}
	check("http://mc.invalid", 200)
	if err := s.StartProxy(occupied.Addr().String()); err == nil {
		t.Fatal("capture accepted occupied port")
	}
	check("http://mc.invalid", 200)
	if s.Snapshot().ProxyAddress != "" || len(s.Flows()) != 0 || len(s.Snapshot().Config.Devices) != 0 {
		t.Fatal("setup started capture")
	}
	if err := s.StartProxy(address); err != nil {
		t.Fatal(err)
	}
	check("http://mc.invalid", 200)
	if s.Snapshot().ProxyAddress != address {
		t.Fatal("capture did not start on setup port")
	}
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer origin.Close()
	check(origin.URL, 204)
	deadline := time.Now().Add(time.Second)
	for len(s.Flows()) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(s.Flows()) != 1 {
		t.Fatal("capture did not record request")
	}
	s.ClearFlows()
	if err := s.StopProxy(); err != nil {
		t.Fatal(err)
	}
	check("http://mc.invalid", 200)
	check("http://example.test", 503)
	if s.Snapshot().ProxyAddress != "" || len(s.Flows()) != 0 {
		t.Fatal("stop did not return to setup")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if conn, err := net.DialTimeout("tcp", address, time.Second); err == nil {
		conn.Close()
		t.Fatal("close left listener active")
	}
	if _, err := s.PrepareConnection(address); err == nil {
		t.Fatal("closed service restarted")
	}
}
