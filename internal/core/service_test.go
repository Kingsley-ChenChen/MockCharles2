package core

import (
	"bufio"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConfigValidationAndRevision(t *testing.T) {
	s := openTestService(t)
	base := Config{
		Projects: []Project{{ID: "p1", Name: "one"}, {ID: "p2", Name: "two"}},
		Devices: []Device{
			{IP: "10.0.0.1", ProjectID: "p1", LinkedRuleSetIDs: []string{"s1"}, ActiveRuleSetID: "s1", LastSelectedRuleSetID: "s1"},
			{IP: "10.0.0.2", ProjectID: "p1", LinkedRuleSetIDs: []string{"s1"}},
		},
		Rules:    []Rule{{ID: "r1", ProjectID: "p1", Name: "fixed", Method: "GET", URL: "http://example.test/a", Status: 201, Enabled: true}},
		RuleSets: []RuleSet{{ID: "s1", ProjectID: "p1", Name: "shared", RuleIDs: []string{"r1"}}},
	}
	if err := s.SaveConfig(base, 0); err != nil {
		t.Fatalf("save valid config: %v", err)
	}
	got := s.Snapshot().Config
	if got.Revision != 1 {
		t.Fatalf("revision = %d, want 1", got.Revision)
	}
	if got.Devices[0].ActiveRuleSetID != "s1" || got.Devices[1].ActiveRuleSetID != "" {
		t.Fatal("shared set activation was not device-independent")
	}
	if err := s.SaveConfig(got, 0); err == nil {
		t.Fatal("stale revision accepted")
	}

	invalid := got
	invalid.Devices[0].ActiveRuleSetID = "missing"
	if err := s.SaveConfig(invalid, 1); err == nil {
		t.Fatal("unlinked activation accepted")
	}

	cross := got
	cross.Devices[0].LinkedRuleSetIDs = []string{"s2"}
	cross.Devices[0].ActiveRuleSetID = "s2"
	cross.RuleSets = append(cross.RuleSets, RuleSet{ID: "s2", ProjectID: "p2", RuleIDs: nil})
	if err := s.SaveConfig(cross, 1); err == nil {
		t.Fatal("cross-project association accepted")
	}
	badLast := cloneConfig(got)
	badLast.Devices[0].LastSelectedRuleSetID = "missing"
	if err := s.SaveConfig(badLast, 1); err == nil {
		t.Fatal("unlinked last selected rule set accepted")
	}
}

func TestRestartRestoresOnlyConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	persisted := Config{Projects: []Project{{ID: "p", Name: "saved"}}, Rules: []Rule{{ID: "r", ProjectID: "p", Name: "rule", Method: "GET", URL: "http://example.invalid/", Status: 200}}, RuleSets: []RuleSet{{ID: "s", ProjectID: "p", Name: "set", RuleIDs: []string{"r"}}}, Devices: []Device{{IP: "127.0.0.1", ProjectID: "p", LinkedRuleSetIDs: []string{"s"}, LastSelectedRuleSetID: "s"}}}
	if err := s.SaveConfig(persisted, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.StartProxy("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, "http://example.invalid/", nil)
	req.Header.Set("X-Forwarded-For", "10.1.2.3")
	_, _ = proxyClient(s).Do(req)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s2.Close() })
	snap := s2.Snapshot()
	if len(snap.Config.Projects) != 1 || snap.Config.Projects[0].Name != "saved" {
		t.Fatalf("config not restored: %#v", snap.Config)
	}
	if snap.ProxyAddress != "" {
		t.Fatalf("listener restored: %q", snap.ProxyAddress)
	}
	if len(s2.Flows()) != 0 {
		t.Fatalf("traffic restored: %#v", s2.Flows())
	}
	if len(snap.Config.Devices) != 1 || snap.Config.Devices[0].IP != "127.0.0.1" {
		t.Fatalf("observed socket IP not restored: %#v", snap.Config.Devices)
	}
	if snap.Config.Devices[0].ActiveRuleSetID != "" || snap.Config.Devices[0].LastSelectedRuleSetID != "s" {
		t.Fatalf("stopped selection not restored: %#v", snap.Config.Devices[0])
	}
}

func TestProxyFixedForwardingHeadersDiscoveryAndFlows(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/plain" {
			if r.Header.Get("X-Project") != "yes" {
				t.Errorf("project request header missing")
			}
			if r.Header.Get("X-Interface") != "project" {
				t.Errorf("interface override = %q", r.Header.Get("X-Interface"))
			}
			if r.Header.Get("X-Hop") != "" {
				t.Errorf("connection-nominated hop header forwarded")
			}
		}
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("X-Upstream", "present")
		w.WriteHeader(202)
		_, _ = w.Write(append([]byte("up:"), body...))
	}))
	defer upstream.Close()

	s := openTestService(t)
	config := Config{
		Projects: []Project{{ID: "p", Name: "project", RequestHeaders: map[string]string{"X-Project": "yes", "X-Interface": "project"}, ResponseHeaders: map[string]string{"X-Response": "project"}}},
		Devices:  []Device{{IP: "127.0.0.1", ProjectID: "p", LinkedRuleSetIDs: []string{"set"}, ActiveRuleSetID: "set"}},
		Rules: []Rule{
			{ID: "first", ProjectID: "p", Name: "first", Method: "*", URL: upstream.URL + "/fixed", Status: 203, Body: "mocked", Headers: map[string]string{"X-Response": "rule"}, Enabled: true},
			{ID: "second", ProjectID: "p", Name: "second", Method: "POST", URL: upstream.URL + "/fixed", Status: 204, Enabled: true},
		},
		RuleSets: []RuleSet{{ID: "set", ProjectID: "p", Name: "set", RuleIDs: []string{"first", "second"}}},
	}
	if err := s.SaveConfig(config, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.StartProxy("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}

	fixed, err := doProxyRequest(s, "203.0.113.1", http.MethodPost, upstream.URL+"/fixed", "ignored", map[string]string{"X-Interface": "last"})
	if err != nil {
		t.Fatal(err)
	}
	assertResponse(t, fixed, 203, "mocked", "X-Response", "rule")

	forwarded, err := doProxyRequest(s, "203.0.113.1", http.MethodPost, upstream.URL+"/forward", "payload", map[string]string{"X-Interface": "last", "Connection": "X-Hop", "X-Hop": "secret"})
	if err != nil {
		t.Fatal(err)
	}
	assertResponse(t, forwarded, 202, "up:payload", "X-Response", "project")

	snap := s.Snapshot()
	if len(snap.Config.Devices) != 1 || snap.Config.Devices[0].IP != "127.0.0.1" {
		t.Fatalf("forwarded header spoof changed identity: %#v", snap.Config.Devices)
	}
	flows := s.Flows()
	if len(flows) != 2 || flows[0].Source != "fixed" || flows[1].Source != "forward" {
		t.Fatalf("flows = %#v", flows)
	}
	s.ClearFlows()
	if len(s.Flows()) != 0 {
		t.Fatal("clear flows failed")
	}
}

func TestProxyFailuresAreVisible(t *testing.T) {
	s := openTestService(t)
	if err := s.StartProxy("bad address"); err == nil {
		t.Fatal("bad listener address accepted")
	}
	if s.Snapshot().ProxyAddress != "" {
		t.Fatal("failed listener published an address")
	}
	if err := s.StartProxy("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	if err := s.StartProxy("127.0.0.1:0"); err == nil {
		t.Fatal("duplicate start accepted")
	}
	conn, err := net.Dial("tcp", s.Snapshot().ProxyAddress)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, _ = io.WriteString(conn, "CONNECT example.test:443 HTTP/1.1\r\nHost: example.test:443\r\n\r\n")
	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("CONNECT status = %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
}

func TestProxyCaptureIsBoundedWithoutTruncatingForwarding(t *testing.T) {
	payload := strings.Repeat("x", bodyCaptureLimit*2)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if string(body) != payload {
			t.Errorf("forwarded body length = %d, want %d", len(body), len(payload))
		}
		_, _ = w.Write(body)
	}))
	defer upstream.Close()
	s := openTestService(t)
	if err := s.StartProxy("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	resp, err := doProxyRequest(s, "10.0.0.8", http.MethodPost, upstream.URL, payload, nil)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(body) != payload {
		t.Fatalf("client body length = %d, want %d", len(body), len(payload))
	}
	flow := s.Flows()[0]
	if len(flow.RequestBody) != bodyCaptureLimit || len(flow.ResponseBody) != bodyCaptureLimit {
		t.Fatalf("capture lengths = %d/%d", len(flow.RequestBody), len(flow.ResponseBody))
	}
}

func TestValidationRejectsMalformedRuleAndHeaders(t *testing.T) {
	s := openTestService(t)
	base := Config{Projects: []Project{{ID: "p", Name: "project"}}, Rules: []Rule{{ID: "r", ProjectID: "p", Name: "rule", Method: "GET", URL: "http://example.test", Status: 200}}, RuleSets: []RuleSet{{ID: "s", ProjectID: "p", Name: "set", RuleIDs: []string{"r"}}}}
	badURL := cloneConfig(base)
	badURL.Rules[0].URL = "/relative"
	if err := s.SaveConfig(badURL, 0); err == nil {
		t.Fatal("relative rule URL accepted")
	}
	badHeader := cloneConfig(base)
	badHeader.Projects[0].RequestHeaders = map[string]string{"X-Test": "ok\r\ninjected: yes"}
	if err := s.SaveConfig(badHeader, 0); err == nil {
		t.Fatal("header injection accepted")
	}
	duplicate := cloneConfig(base)
	duplicate.RuleSets[0].RuleIDs = []string{"r", "r"}
	if err := s.SaveConfig(duplicate, 0); err == nil {
		t.Fatal("duplicate rule association accepted")
	}
	for _, header := range []string{"Content-Length", "transfer-encoding"} {
		framing := cloneConfig(base)
		framing.Projects[0].ResponseHeaders = map[string]string{header: "7"}
		if err := s.SaveConfig(framing, 0); err == nil {
			t.Fatalf("protocol framing header %q accepted", header)
		}
	}
}

func TestConfiguredHostOverridesBoundRequestAndUnboundKeepsURLHost(t *testing.T) {
	hosts := make(chan string, 2)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hosts <- r.Host; w.WriteHeader(204) }))
	defer upstream.Close()

	bound := openTestService(t)
	config := Config{Projects: []Project{{ID: "p", Name: "project", RequestHeaders: map[string]string{"Host": "configured.example"}}}, Devices: []Device{{IP: "127.0.0.1", ProjectID: "p"}}}
	if err := bound.SaveConfig(config, 0); err != nil {
		t.Fatal(err)
	}
	if err := bound.StartProxy("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	resp, err := doProxyRequest(bound, "", http.MethodGet, upstream.URL+"/bound", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if got := <-hosts; got != "configured.example" {
		t.Fatalf("bound Host = %q", got)
	}

	unbound := openTestService(t)
	if err := unbound.StartProxy("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	resp, err = doProxyRequest(unbound, "", http.MethodGet, upstream.URL+"/unbound", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	want := strings.TrimPrefix(upstream.URL, "http://")
	if got := <-hosts; got != want {
		t.Fatalf("unbound Host = %q, want %q", got, want)
	}
}

func TestStopProxyForcesBoundedShutdown(t *testing.T) {
	s := openTestService(t)
	if err := s.StartProxy("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	conn, err := net.Dial("tcp", s.Snapshot().ProxyAddress)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, _ = io.WriteString(conn, "GET http://example.test/ HTTP/1.1\r\n")
	started := time.Now()
	_ = s.StopProxy()
	if elapsed := time.Since(started); elapsed > 4*time.Second {
		t.Fatalf("shutdown took %v", elapsed)
	}
	if s.Snapshot().ProxyAddress != "" {
		t.Fatal("stopped address remains visible")
	}
}

func TestInFlightRequestKeepsRoutingSnapshot(t *testing.T) {
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/first" {
			entered <- struct{}{}
			<-release
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	s := openTestService(t)
	old := Config{Projects: []Project{{ID: "p", Name: "project", ResponseHeaders: map[string]string{"X-Version": "old"}}}, Devices: []Device{{IP: "127.0.0.1", ProjectID: "p"}}}
	if err := s.SaveConfig(old, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.StartProxy("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	result := make(chan *http.Response, 1)
	requestErr := make(chan error, 1)
	go func() {
		resp, err := doProxyRequest(s, "", http.MethodGet, upstream.URL+"/first", "", nil)
		result <- resp
		requestErr <- err
	}()
	<-entered
	updated := cloneConfig(s.Snapshot().Config)
	updated.Projects[0].ResponseHeaders["X-Version"] = "new"
	if err := s.SaveConfig(updated, 1); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-requestErr; err != nil {
		t.Fatal(err)
	}
	first := <-result
	_ = first.Body.Close()
	if got := first.Header.Get("X-Version"); got != "old" {
		t.Fatalf("in-flight header = %q", got)
	}
	second, err := doProxyRequest(s, "", http.MethodGet, upstream.URL+"/second", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = second.Body.Close()
	if got := second.Header.Get("X-Version"); got != "new" {
		t.Fatalf("subsequent header = %q", got)
	}
}

type failingResponseWriter struct{ header http.Header }

func (w *failingResponseWriter) Header() http.Header { return w.header }
func (w *failingResponseWriter) WriteHeader(int)     {}
func (w *failingResponseWriter) Write([]byte) (int, error) {
	return 0, errors.New("client write failed")
}

func TestFixedResponseWriteErrorIsRecorded(t *testing.T) {
	s := openTestService(t)
	config := Config{Projects: []Project{{ID: "p", Name: "project"}}, Devices: []Device{{IP: "127.0.0.1", ProjectID: "p", LinkedRuleSetIDs: []string{"s"}, ActiveRuleSetID: "s"}}, Rules: []Rule{{ID: "r", ProjectID: "p", Name: "rule", Method: "GET", URL: "http://example.test/fixed", Status: 200, Body: "body", Enabled: true}}, RuleSets: []RuleSet{{ID: "s", ProjectID: "p", Name: "set", RuleIDs: []string{"r"}}}}
	if err := s.SaveConfig(config, 0); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.test/fixed", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	s.handleProxy(&failingResponseWriter{header: make(http.Header)}, req)
	flows := s.Flows()
	if len(flows) != 1 || !strings.Contains(flows[0].Error, "client write failed") {
		t.Fatalf("flow error = %#v", flows)
	}
}

func openTestService(t *testing.T) *Service {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "config.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func proxyClient(s *Service) *http.Client {
	u, _ := url.Parse("http://" + s.Snapshot().ProxyAddress)
	return &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(u)}}
}

func doProxyRequest(s *Service, ip, method, url, body string, headers map[string]string) (*http.Response, error) {
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("X-Forwarded-For", ip)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return proxyClient(s).Do(req)
}

func assertResponse(t *testing.T, resp *http.Response, status int, body, header, value string) {
	t.Helper()
	defer resp.Body.Close()
	got, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != status || string(got) != body || resp.Header.Get(header) != value {
		t.Fatalf("response = %d %q %q", resp.StatusCode, got, resp.Header.Get(header))
	}
}
