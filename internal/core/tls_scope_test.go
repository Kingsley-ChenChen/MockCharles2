package core

import "testing"

func TestTLSWildcardScope(t *testing.T) {
	for _, tc := range []struct {
		host string
		want bool
	}{{"baidu.com", true}, {"www.baidu.com", true}, {"a.b.baidu.com", true}, {"evilbaidu.com", false}, {"baidu.com.evil.test", false}} {
		if got := intercepts(TLSSettings{Enabled: true, Hosts: []string{"*.baidu.com"}}, tc.host); got != tc.want {
			t.Errorf("%s = %v", tc.host, got)
		}
	}
	if intercepts(TLSSettings{Enabled: true, Hosts: []string{"baidu.com"}}, "www.baidu.com") {
		t.Fatal("old exact scope expanded")
	}
	if intercepts(TLSSettings{Enabled: false, Hosts: []string{"*.baidu.com"}}, "www.baidu.com") {
		t.Fatal("disabled scope intercepted")
	}
	s := openTestService(t)
	c := s.Snapshot().Config
	c.TLS.Hosts = []string{"*.baidu.com"}
	if err := s.SaveConfig(c, c.Revision); err != nil {
		t.Fatal(err)
	}
	c = s.Snapshot().Config
	c.TLS.Hosts = append(c.TLS.Hosts, "*.BAIDU.COM.")
	if s.SaveConfig(c, c.Revision) == nil {
		t.Fatal("duplicate scope accepted")
	}
}
