package core

import "testing"

func TestConnectionInfoUsesActualListener(t *testing.T) {
	s := openTestService(t)
	if err := s.StartProxy("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	info, err := s.ConnectionInfo("0.0.0.0:8888")
	if err != nil {
		t.Fatal(err)
	}
	if !info.Listening || info.Port == "0" || len(info.Addresses) != 0 {
		t.Fatalf("must use actual loopback listener, not draft: %+v", info)
	}
}

func TestConnectionInfoUsesBindablePrivateAddresses(t *testing.T) {
	candidates := []LANAddress{{IP: "127.0.0.1"}, {IP: "192.168.1.8", InterfaceName: "Wi-Fi"}, {IP: "10.8.0.2", InterfaceName: "VPN"}, {IP: "8.8.8.8"}, {IP: "169.254.1.2"}, {IP: "fd00::2", InterfaceName: "IPv6"}, {IP: "192.168.1.8"}}
	info, err := connectionInfo("0.0.0.0:8888", candidates)
	if err != nil {
		t.Fatal(err)
	}
	if info.Port != "8888" || len(info.Addresses) != 2 {
		t.Fatalf("unexpected candidates: %+v", info)
	}
	info, err = connectionInfo("192.168.1.8:9000", candidates)
	if err != nil || len(info.Addresses) != 1 || info.Addresses[0].IP != "192.168.1.8" || info.Port != "9000" {
		t.Fatalf("bound interface: %+v %v", info, err)
	}
	info, _ = connectionInfo("127.0.0.1:8888", candidates)
	if len(info.Addresses) != 0 {
		t.Fatal("loopback listener advertised for phone")
	}
	info, _ = connectionInfo("[::]:8888", candidates)
	if len(info.Addresses) != 3 {
		t.Fatalf("dual stack: %+v", info)
	}
	if _, err = connectionInfo("broken", candidates); err == nil {
		t.Fatal("invalid listener accepted")
	}
}
