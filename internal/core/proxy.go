package core

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync/atomic"
	"time"
)

const bodyCaptureLimit = 64 << 10

var flowSequence atomic.Uint64

func timeoutContext(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

func (s *Service) StartProxy(address string) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return fmt.Errorf("service is closed")
	}
	if s.server != nil {
		s.mu.Unlock()
		return fmt.Errorf("proxy is already running")
	}
	s.mu.Unlock()
	ln, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	server := &http.Server{Handler: http.HandlerFunc(s.handleProxy), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 30 * time.Second}
	s.mu.Lock()
	if s.server != nil || s.closed {
		s.mu.Unlock()
		ln.Close()
		return fmt.Errorf("proxy is unavailable")
	}
	s.server, s.listener, s.address = server, ln, ln.Addr().String()
	s.mu.Unlock()
	go func() { _ = server.Serve(ln) }()
	return nil
}

func (s *Service) handleProxy(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	snap := s.Snapshot().Config
	ip := clientIP(r)
	flow := Flow{ID: fmt.Sprintf("%d", flowSequence.Add(1)), IP: ip, Method: r.Method, URL: r.URL.String(), Source: "forward", Start: started, RequestHeaders: r.Header.Clone()}
	defer func() { flow.Duration = time.Since(started); s.addFlow(flow) }()
	s.observeIP(ip)
	if r.Method == http.MethodConnect {
		flow.Status = http.StatusNotImplemented
		flow.Source = "unsupported"
		http.Error(w, "CONNECT is not supported in this milestone", http.StatusNotImplemented)
		return
	}
	device, project, activeSet := routeFor(snap, ip)
	_ = device
	if rule := firstMatch(snap, activeSet, r.Method, r.URL.String()); rule != nil {
		flow.Source = "fixed"
		flow.Status = rule.Status
		flow.RequestBody = captureAndDrain(r.Body)
		for k, v := range project.ResponseHeaders {
			w.Header().Set(k, v)
		}
		for k, v := range rule.Headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(rule.Status)
		_, err := io.WriteString(w, rule.Body)
		if err != nil {
			flow.Error = err.Error()
		}
		flow.ResponseHeaders = w.Header().Clone()
		flow.ResponseBody = boundedString([]byte(rule.Body))
		return
	}
	for k, v := range project.RequestHeaders {
		if strings.EqualFold(k, "Host") {
			r.Host = v
		} else {
			r.Header.Set(k, v)
		}
	}
	removeHopHeaders(r.Header)
	r.RequestURI = ""
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	requestCapture := &limitedBuffer{limit: bodyCaptureLimit}
	if r.Body != nil {
		r.Body = io.NopCloser(io.TeeReader(r.Body, requestCapture))
	}
	resp, err := s.transport.RoundTrip(r)
	flow.RequestBody = requestCapture.String()
	if err != nil {
		flow.Status = http.StatusBadGateway
		flow.Error = err.Error()
		http.Error(w, "proxy forwarding failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	removeHopHeaders(resp.Header)
	for k, v := range project.ResponseHeaders {
		resp.Header.Set(k, v)
	}
	copyHeader(w.Header(), resp.Header)
	flow.ResponseHeaders = resp.Header.Clone()
	flow.Status = resp.StatusCode
	w.WriteHeader(resp.StatusCode)
	responseCapture := &limitedBuffer{limit: bodyCaptureLimit}
	_, err = io.Copy(w, io.TeeReader(resp.Body, responseCapture))
	flow.ResponseBody = responseCapture.String()
	if err != nil {
		flow.Error = err.Error()
	}
}

func routeFor(c Config, ip string) (Device, Project, RuleSet) {
	var d Device
	for _, x := range c.Devices {
		if x.IP == ip {
			d = x
			break
		}
	}
	var p Project
	for _, x := range c.Projects {
		if x.ID == d.ProjectID {
			p = x
			break
		}
	}
	var set RuleSet
	for _, x := range c.RuleSets {
		if x.ID == d.ActiveRuleSetID {
			set = x
			break
		}
	}
	return d, p, set
}
func firstMatch(c Config, set RuleSet, method, url string) *Rule {
	for _, id := range set.RuleIDs {
		for i := range c.Rules {
			r := &c.Rules[i]
			if r.ID == id && r.Enabled && (r.Method == "*" || strings.EqualFold(r.Method, method)) && r.URL == url {
				return r
			}
		}
	}
	return nil
}
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return ""
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return ""
	}
	return addr.Unmap().String()
}
func captureAndDrain(body io.ReadCloser) string {
	if body == nil {
		return ""
	}
	defer body.Close()
	b := &limitedBuffer{limit: bodyCaptureLimit}
	_, _ = io.Copy(io.Discard, io.TeeReader(body, b))
	return b.String()
}
func boundedString(b []byte) string {
	if len(b) > bodyCaptureLimit {
		b = b[:bodyCaptureLimit]
	}
	return string(b)
}

type limitedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	original := len(p)
	if remaining := b.limit - b.Len(); remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = b.Buffer.Write(p)
	}
	return original, nil
}
func copyHeader(dst, src http.Header) {
	for k, v := range src {
		dst[k] = append([]string(nil), v...)
	}
}
func removeHopHeaders(h http.Header) {
	for _, value := range h.Values("Connection") {
		for _, name := range strings.Split(value, ",") {
			h.Del(strings.TrimSpace(name))
		}
	}
	for _, name := range []string{"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade"} {
		h.Del(name)
	}
}
