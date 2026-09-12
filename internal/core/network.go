package core

import (
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strconv"
)

type LANAddress struct {
	IP            string `json:"ip"`
	InterfaceName string `json:"interfaceName"`
}
type ConnectionInfo struct {
	Addresses []LANAddress `json:"addresses"`
	Port      string       `json:"port"`
	Listening bool         `json:"listening"`
}

func (s *Service) ConnectionInfo(configuredAddress string) (ConnectionInfo, error) {
	actual := s.Snapshot().ProxyAddress
	address := configuredAddress
	if actual != "" {
		address = actual
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return ConnectionInfo{}, err
	}
	candidates := []LANAddress{}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return ConnectionInfo{}, fmt.Errorf("读取网卡 %s: %w", iface.Name, err)
		}
		for _, addr := range addrs {
			prefix, err := netip.ParsePrefix(addr.String())
			if err == nil {
				candidates = append(candidates, LANAddress{IP: prefix.Addr().Unmap().String(), InterfaceName: iface.Name})
			}
		}
	}
	info, err := connectionInfo(address, candidates)
	info.Listening = actual != ""
	return info, err
}
func connectionInfo(address string, candidates []LANAddress) (ConnectionInfo, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return ConnectionInfo{}, fmt.Errorf("监听地址无效: %w", err)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 0 || n > 65535 {
		return ConnectionInfo{}, fmt.Errorf("监听端口无效: %s", port)
	}
	var bound netip.Addr
	if host != "" {
		bound, err = netip.ParseAddr(host)
		if err != nil {
			return ConnectionInfo{}, fmt.Errorf("请使用 IP 形式的监听地址")
		}
		bound = bound.Unmap()
	}
	info := ConnectionInfo{Port: strconv.Itoa(n), Addresses: []LANAddress{}}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		ip, err := netip.ParseAddr(candidate.IP)
		if err != nil {
			continue
		}
		ip = ip.Unmap()
		if !ip.IsPrivate() {
			continue
		}
		if bound.IsValid() {
			if !bound.IsUnspecified() && bound != ip {
				continue
			}
			if bound.IsUnspecified() && bound.Is4() && !ip.Is4() {
				continue
			}
		}
		key := ip.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		candidate.IP = key
		info.Addresses = append(info.Addresses, candidate)
	}
	sort.Slice(info.Addresses, func(i, j int) bool {
		a, b := netip.MustParseAddr(info.Addresses[i].IP), netip.MustParseAddr(info.Addresses[j].IP)
		if a.Is4() != b.Is4() {
			return a.Is4()
		}
		return a.Compare(b) < 0
	})
	return info, nil
}
