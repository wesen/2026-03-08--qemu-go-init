package networking

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"
)

func TestSelectInterfacePrefersExplicitName(t *testing.T) {
	choices := []InterfaceChoice{
		{Name: "lo", Loopback: true},
		{Name: "ens3", HardwareAddr: "52:54:00:12:34:56"},
		{Name: "eth0", HardwareAddr: "52:54:00:aa:bb:cc"},
	}

	choice, err := SelectInterface(choices, "eth0")
	if err != nil {
		t.Fatalf("SelectInterface: %v", err)
	}
	if choice.Name != "eth0" {
		t.Fatalf("got %q, want eth0", choice.Name)
	}
}

func TestSelectInterfaceChoosesFirstUsableNonLoopback(t *testing.T) {
	choices := []InterfaceChoice{
		{Name: "lo", Loopback: true},
		{Name: "dummy0"},
		{Name: "ens3", HardwareAddr: "52:54:00:12:34:56"},
	}

	choice, err := SelectInterface(choices, "")
	if err != nil {
		t.Fatalf("SelectInterface: %v", err)
	}
	if choice.Name != "ens3" {
		t.Fatalf("got %q, want ens3", choice.Name)
	}
}

func TestRenderResolvConf(t *testing.T) {
	contents := renderResolvConf([]net.IP{net.IPv4(10, 0, 2, 3), net.IPv4(1, 1, 1, 1)}, "qemu.internal")

	if !strings.Contains(contents, "search qemu.internal") {
		t.Fatalf("missing search line: %q", contents)
	}
	if !strings.Contains(contents, "nameserver 10.0.2.3") {
		t.Fatalf("missing first nameserver: %q", contents)
	}
	if !strings.Contains(contents, "nameserver 1.1.1.1") {
		t.Fatalf("missing second nameserver: %q", contents)
	}
}

func TestDetailsFromLease(t *testing.T) {
	ack := &dhcpv4.DHCPv4{
		YourIPAddr: net.IPv4(10, 0, 2, 15),
	}
	ack.UpdateOption(dhcpv4.OptSubnetMask(net.CIDRMask(24, 32)))
	ack.UpdateOption(dhcpv4.OptRouter(net.IPv4(10, 0, 2, 2)))
	ack.UpdateOption(dhcpv4.OptDNS(net.IPv4(10, 0, 2, 3), net.IPv4(1, 1, 1, 1)))
	ack.UpdateOption(dhcpv4.OptDomainName("qemu.internal"))
	ack.UpdateOption(dhcpv4.OptIPAddressLeaseTime(5 * time.Minute))

	lease := &nclient4.Lease{ACK: ack}
	details, err := detailsFromLease(lease)
	if err != nil {
		t.Fatalf("detailsFromLease: %v", err)
	}

	if got, want := details.CIDR, "10.0.2.15/24"; got != want {
		t.Fatalf("CIDR = %q, want %q", got, want)
	}
	if got, want := details.Gateway.String(), "10.0.2.2"; got != want {
		t.Fatalf("gateway = %q, want %q", got, want)
	}
	if got, want := details.DomainName, "qemu.internal"; got != want {
		t.Fatalf("domain = %q, want %q", got, want)
	}
	if got, want := details.LeaseTime, 5*time.Minute; got != want {
		t.Fatalf("lease time = %s, want %s", got, want)
	}
	if !strings.Contains(details.ResolverContents, "nameserver 10.0.2.3") {
		t.Fatalf("resolver contents missing DHCP DNS server: %q", details.ResolverContents)
	}
}
