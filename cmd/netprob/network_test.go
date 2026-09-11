package main

import (
	"net"
	"reflect"
	"testing"
)

func TestUsableAgentAddresses(t *testing.T) {
	addresses := []net.Addr{
		&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)},
		&net.IPNet{IP: net.ParseIP("169.254.10.1"), Mask: net.CIDRMask(16, 32)},
		&net.IPNet{IP: net.ParseIP("10.20.30.40"), Mask: net.CIDRMask(24, 32)},
		&net.IPAddr{IP: net.ParseIP("203.0.113.20")},
		&net.IPAddr{IP: net.ParseIP("10.20.30.40")},
		&net.IPNet{IP: net.ParseIP("::1"), Mask: net.CIDRMask(128, 128)},
		&net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)},
		&net.IPNet{IP: net.ParseIP("2001:db8::20"), Mask: net.CIDRMask(64, 128)},
	}

	want := []string{"10.20.30.40", "203.0.113.20", "2001:db8::20"}
	if got := usableAgentAddresses(addresses); !reflect.DeepEqual(got, want) {
		t.Fatalf("usableAgentAddresses() = %#v, want %#v", got, want)
	}
}

func TestChoosePrimaryAddress(t *testing.T) {
	connectionAddress := &net.TCPAddr{IP: net.ParseIP("213.163.198.21"), Port: 47468}
	addresses := []string{"10.10.0.1", "213.163.198.21"}

	if got := choosePrimaryAddress("", connectionAddress, addresses, "203.0.113.10"); got != "213.163.198.21" {
		t.Fatalf("default-route primary = %q", got)
	}
	if got := choosePrimaryAddress("100.81.110.123", connectionAddress, addresses, ""); got != "100.81.110.123" {
		t.Fatalf("override primary = %q", got)
	}
}
