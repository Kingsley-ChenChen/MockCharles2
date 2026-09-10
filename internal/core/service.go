package core

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
)

const maxFlows = 500

type Service struct {
	mu        sync.RWMutex
	store     *store
	config    Config
	address   string
	server    *http.Server
	listener  net.Listener
	transport *http.Transport
	flows     []Flow
	closed    bool
}

func Open(path string) (*Service, error) {
	st, config, err := openStore(path)
	if err != nil {
		return nil, err
	}
	transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext, ResponseHeaderTimeout: 15 * time.Second}
	return &Service{store: st, config: config, transport: transport}, nil
}

func (s *Service) Close() error {
	_ = s.StopProxy()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	s.transport.CloseIdleConnections()
	return s.store.db.Close()
}

func (s *Service) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Snapshot{Config: cloneConfig(s.config), ProxyAddress: s.address}
}

func (s *Service) SaveConfig(config Config, expectedRevision int64) error {
	if err := validateConfig(config); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("service is closed")
	}
	if s.config.Revision != expectedRevision {
		return ErrStaleRevision
	}
	saved, err := s.store.save(config, expectedRevision)
	if err != nil {
		return err
	}
	s.config = cloneConfig(saved)
	return nil
}

func (s *Service) Flows() []Flow {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := slices.Clone(s.flows)
	for i := range out {
		out[i].RequestHeaders = cloneHeaderMap(out[i].RequestHeaders)
		out[i].ResponseHeaders = cloneHeaderMap(out[i].ResponseHeaders)
	}
	return out
}
func cloneHeaderMap(m map[string][]string) map[string][]string {
	if m == nil {
		return nil
	}
	out := make(map[string][]string, len(m))
	for k, v := range m {
		out[k] = slices.Clone(v)
	}
	return out
}
func (s *Service) ClearFlows() { s.mu.Lock(); s.flows = nil; s.mu.Unlock() }

func (s *Service) addFlow(flow Flow) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flows = append(s.flows, flow)
	if len(s.flows) > maxFlows {
		s.flows = slices.Clone(s.flows[len(s.flows)-maxFlows:])
	}
}

func (s *Service) observeIP(ip string) {
	if ip == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.config.Devices {
		if d.IP == ip {
			return
		}
	}
	next := cloneConfig(s.config)
	next.Devices = append(next.Devices, Device{IP: ip})
	saved, err := s.store.save(next, s.config.Revision)
	if err == nil {
		s.config = saved
	}
}

func validateConfig(c Config) error {
	projects := map[string]bool{}
	rules := map[string]Rule{}
	sets := map[string]RuleSet{}
	devices := map[string]bool{}
	for _, p := range c.Projects {
		if p.ID == "" || strings.TrimSpace(p.Name) == "" || projects[p.ID] {
			return fmt.Errorf("invalid or duplicate project %q", p.ID)
		}
		if err := validateHeaders(p.RequestHeaders); err != nil {
			return fmt.Errorf("project %q request headers: %w", p.ID, err)
		}
		if err := validateHeaders(p.ResponseHeaders); err != nil {
			return fmt.Errorf("project %q response headers: %w", p.ID, err)
		}
		projects[p.ID] = true
	}
	for _, r := range c.Rules {
		if r.ID == "" || strings.TrimSpace(r.Name) == "" || (r.Method != "*" && !validHeaderName(r.Method)) || rules[r.ID].ID != "" || !projects[r.ProjectID] {
			return fmt.Errorf("invalid rule %q", r.ID)
		}
		u, err := url.Parse(r.URL)
		if err != nil || u.Scheme != "http" || u.Host == "" {
			return fmt.Errorf("invalid URL for rule %q", r.ID)
		}
		if r.Status < 200 || r.Status > 599 {
			return fmt.Errorf("invalid status for rule %q", r.ID)
		}
		if err := validateHeaders(r.Headers); err != nil {
			return fmt.Errorf("rule %q headers: %w", r.ID, err)
		}
		rules[r.ID] = r
	}
	for _, set := range c.RuleSets {
		if set.ID == "" || strings.TrimSpace(set.Name) == "" || sets[set.ID].ID != "" || !projects[set.ProjectID] {
			return fmt.Errorf("invalid rule set %q", set.ID)
		}
		seen := map[string]bool{}
		for _, id := range set.RuleIDs {
			r, ok := rules[id]
			if !ok || r.ProjectID != set.ProjectID || seen[id] {
				return fmt.Errorf("rule %q is outside rule set project", id)
			}
			seen[id] = true
		}
		sets[set.ID] = set
	}
	for _, d := range c.Devices {
		if d.IP == "" || devices[d.IP] {
			return fmt.Errorf("invalid or duplicate device %q", d.IP)
		}
		addr, err := netip.ParseAddr(d.IP)
		if err != nil || addr.Unmap().String() != d.IP {
			return fmt.Errorf("device IP %q is not canonical", d.IP)
		}
		devices[d.IP] = true
		if d.ProjectID == "" {
			if len(d.LinkedRuleSetIDs) > 0 || d.ActiveRuleSetID != "" || d.LastSelectedRuleSetID != "" {
				return fmt.Errorf("unbound device %q has rule sets", d.IP)
			}
			continue
		}
		if !projects[d.ProjectID] {
			return fmt.Errorf("device %q has missing project", d.IP)
		}
		linked := map[string]bool{}
		for _, id := range d.LinkedRuleSetIDs {
			set, ok := sets[id]
			if !ok || set.ProjectID != d.ProjectID || linked[id] {
				return fmt.Errorf("device %q has cross-project rule set", d.IP)
			}
			linked[id] = true
		}
		if d.ActiveRuleSetID != "" && !linked[d.ActiveRuleSetID] {
			return fmt.Errorf("device %q active rule set is not linked", d.IP)
		}
		if d.LastSelectedRuleSetID != "" && !linked[d.LastSelectedRuleSetID] {
			return fmt.Errorf("device %q last selected rule set is not linked", d.IP)
		}
	}
	return nil
}

func validateHeaders(headers map[string]string) error {
	for k, v := range headers {
		if !validHeaderName(k) {
			return fmt.Errorf("invalid name %q", k)
		}
		if strings.EqualFold(k, "Content-Length") || strings.EqualFold(k, "Transfer-Encoding") {
			return fmt.Errorf("protocol framing header %q is managed by the proxy", k)
		}
		if strings.ContainsAny(v, "\r\n") {
			return fmt.Errorf("invalid value for %q", k)
		}
		for _, r := range v {
			if r == 127 || r < 32 && r != '\t' {
				return fmt.Errorf("invalid value for %q", k)
			}
		}
	}
	return nil
}
func validHeaderName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", r)) {
			return false
		}
	}
	return true
}

func cloneConfig(c Config) Config {
	out := c
	out.Projects = slices.Clone(c.Projects)
	out.Devices = slices.Clone(c.Devices)
	out.Rules = slices.Clone(c.Rules)
	out.RuleSets = slices.Clone(c.RuleSets)
	for i := range out.Projects {
		out.Projects[i].RequestHeaders = cloneStringMap(out.Projects[i].RequestHeaders)
		out.Projects[i].ResponseHeaders = cloneStringMap(out.Projects[i].ResponseHeaders)
	}
	for i := range out.Devices {
		out.Devices[i].LinkedRuleSetIDs = slices.Clone(out.Devices[i].LinkedRuleSetIDs)
	}
	for i := range out.Rules {
		out.Rules[i].Headers = cloneStringMap(out.Rules[i].Headers)
	}
	for i := range out.RuleSets {
		out.RuleSets[i].RuleIDs = slices.Clone(out.RuleSets[i].RuleIDs)
	}
	return out
}
func cloneStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func (s *Service) StopProxy() error {
	s.mu.Lock()
	server := s.server
	s.server = nil
	s.listener = nil
	s.address = ""
	s.mu.Unlock()
	if server == nil {
		return nil
	}
	ctx, cancel := timeoutContext(3 * time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		_ = server.Close()
		return err
	}
	return nil
}
