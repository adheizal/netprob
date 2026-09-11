package main

import (
	"net"
	"sort"
	"strings"
)

func detectAgentAddresses() ([]string, error) {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	return usableAgentAddresses(addresses), nil
}

func choosePrimaryAddress(override string, connectionAddress net.Addr, addresses []string, publicIP string) string {
	if ip := net.ParseIP(strings.TrimSpace(override)); ip != nil && ip.IsGlobalUnicast() {
		return ip.String()
	}
	if tcpAddress, ok := connectionAddress.(*net.TCPAddr); ok && tcpAddress.IP.IsGlobalUnicast() {
		return tcpAddress.IP.String()
	}
	if ip := net.ParseIP(strings.TrimSpace(publicIP)); ip != nil && ip.IsGlobalUnicast() {
		return ip.String()
	}
	for _, address := range addresses {
		if ip := net.ParseIP(strings.TrimSpace(address)); ip != nil && ip.IsGlobalUnicast() {
			return ip.String()
		}
	}
	return ""
}

func usableAgentAddresses(addresses []net.Addr) []string {
	unique := make(map[string]net.IP)
	for _, address := range addresses {
		var ip net.IP
		switch value := address.(type) {
		case *net.IPNet:
			ip = value.IP
		case *net.IPAddr:
			ip = value.IP
		default:
			continue
		}
		if !ip.IsGlobalUnicast() {
			continue
		}
		canonical := ip.String()
		unique[canonical] = ip
	}

	result := make([]string, 0, len(unique))
	for address := range unique {
		result = append(result, address)
	}
	sort.Slice(result, func(i, j int) bool {
		leftIPv4 := unique[result[i]].To4() != nil
		rightIPv4 := unique[result[j]].To4() != nil
		if leftIPv4 != rightIPv4 {
			return leftIPv4
		}
		return result[i] < result[j]
	})
	return result
}
