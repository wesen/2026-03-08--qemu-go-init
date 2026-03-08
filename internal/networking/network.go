package networking

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

const (
	defaultDHCPTimeout = 15 * time.Second
	defaultDHCPRetry   = 3
	resolvConfPath     = "/etc/resolv.conf"
)

var xidCounter = atomic.Uint32{}

type Result struct {
	Method           string   `json:"method"`
	Configured       bool     `json:"configured"`
	Fallback         bool     `json:"fallback,omitempty"`
	InterfaceName    string   `json:"interfaceName,omitempty"`
	HardwareAddr     string   `json:"hardwareAddr,omitempty"`
	OperState        string   `json:"operState,omitempty"`
	LinkFlags        []string `json:"linkFlags,omitempty"`
	MTU              int      `json:"mtu,omitempty"`
	Address          string   `json:"address,omitempty"`
	CIDR             string   `json:"cidr,omitempty"`
	Gateway          string   `json:"gateway,omitempty"`
	DNSServers       []string `json:"dnsServers,omitempty"`
	DomainName       string   `json:"domainName,omitempty"`
	LeaseSeconds     int64    `json:"leaseSeconds,omitempty"`
	RenewalSeconds   int64    `json:"renewalSeconds,omitempty"`
	RebindingSeconds int64    `json:"rebindingSeconds,omitempty"`
	ResolverConfig   string   `json:"resolverConfig,omitempty"`
	Step             string   `json:"step,omitempty"`
	DHCPError        string   `json:"dhcpError,omitempty"`
	Error            string   `json:"error,omitempty"`
}

type Config struct {
	PreferredInterface string
	Timeout            time.Duration
	Retry              int
	EnableDHCP         bool
	EnableQEMUFallback bool
}

type InterfaceChoice struct {
	Name         string
	HardwareAddr string
	Loopback     bool
}

type leaseDetails struct {
	Address          net.IP
	PrefixLength     int
	CIDR             string
	Gateway          net.IP
	DNSServers       []net.IP
	DomainName       string
	LeaseTime        time.Duration
	RenewalTime      time.Duration
	RebindingTime    time.Duration
	ResolverContents string
}

func LoadConfigFromEnv() Config {
	return Config{
		PreferredInterface: os.Getenv("GO_INIT_NETWORK_INTERFACE"),
		Timeout:            durationEnv("GO_INIT_DHCP_TIMEOUT", defaultDHCPTimeout),
		Retry:              intEnv("GO_INIT_DHCP_RETRY", defaultDHCPRetry),
		EnableDHCP:         boolEnv("GO_INIT_ENABLE_DHCP", true),
		EnableQEMUFallback: boolEnv("GO_INIT_ENABLE_QEMU_USERNET_FALLBACK", true),
	}
}

func Configure(logger *log.Logger) (Result, error) {
	cfg := LoadConfigFromEnv()
	result := Result{
		Method: "userspace-dhcp",
		Step:   "init",
	}
	logger.Printf("networking: config interface=%q timeout=%s retry=%d dhcp=%t qemu-fallback=%t",
		cfg.PreferredInterface,
		cfg.Timeout,
		cfg.Retry,
		cfg.EnableDHCP,
		cfg.EnableQEMUFallback,
	)
	logInterfaceInventory(logger)

	if !cfg.EnableDHCP {
		result.Method = "disabled"
		result.Step = "disabled"
		return result, nil
	}

	link, err := selectLink(cfg.PreferredInterface)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	attrs := link.Attrs()
	result.InterfaceName = attrs.Name
	result.HardwareAddr = attrs.HardwareAddr.String()
	result.OperState = attrs.OperState.String()
	result.LinkFlags = formatFlags(attrs.Flags)
	result.MTU = attrs.MTU
	result.Step = "link-up"
	logger.Printf("networking: selected interface=%s mac=%s mtu=%d oper=%s flags=%s",
		attrs.Name,
		attrs.HardwareAddr,
		attrs.MTU,
		attrs.OperState.String(),
		strings.Join(result.LinkFlags, ","),
	)

	if err := netlink.LinkSetUp(link); err != nil {
		err = fmt.Errorf("set link %s up: %w", attrs.Name, err)
		result.Error = err.Error()
		return result, err
	}
	updatedLink, err := netlink.LinkByName(attrs.Name)
	if err == nil {
		updated := updatedLink.Attrs()
		result.OperState = updated.OperState.String()
		result.LinkFlags = formatFlags(updated.Flags)
		result.MTU = updated.MTU
		logger.Printf("networking: link %s is up oper=%s flags=%s",
			updated.Name,
			updated.OperState.String(),
			strings.Join(result.LinkFlags, ","),
		)
	} else {
		logger.Printf("networking: link %s is up (refresh failed: %v)", attrs.Name, err)
	}
	logAppliedState(logger, link, "pre-dhcp")

	dhcpConn, err := newDHCPBroadcastConn(attrs.Name)
	if err != nil {
		err = fmt.Errorf("open DHCP socket on %s: %w", attrs.Name, err)
		result.Error = err.Error()
		return result, err
	}
	logger.Printf("networking: opened DHCP UDP broadcast socket on %s local=%s", attrs.Name, dhcpConn.LocalAddr())

	dhcpClient, err := nclient4.NewWithConn(dhcpConn, attrs.HardwareAddr,
		nclient4.WithTimeout(cfg.Timeout),
		nclient4.WithRetry(cfg.Retry),
		nclient4.WithLogger(dhcpLogger{logger: logger}),
	)
	if err != nil {
		err = fmt.Errorf("create DHCP client on %s: %w", attrs.Name, err)
		result.Error = err.Error()
		return result, err
	}
	defer dhcpClient.Close()

	result.Step = "dhcp-request"
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout*time.Duration(max(cfg.Retry, 1)))
	defer cancel()

	xid := nextTransactionID()
	logger.Printf("networking: requesting DHCP lease on %s xid=%s deadline=%s", attrs.Name, xid.String(), cfg.Timeout*time.Duration(max(cfg.Retry, 1)))
	requestDone := make(chan struct{})
	defer close(requestDone)
	go logDHCPWait(logger, ctx, requestDone, attrs.Name, xid)
	lease, err := dhcpClient.Request(ctx,
		dhcpv4.WithBroadcast(true),
		dhcpv4.WithTransactionID(xid),
	)
	var details leaseDetails
	if err != nil {
		result.DHCPError = err.Error()
		if cfg.EnableQEMUFallback {
			logger.Printf("networking: DHCP failed on %s (%v), applying QEMU user-net fallback", attrs.Name, err)
			details = qemuUsernetFallback()
			result.Method = "qemu-usernet-fallback"
			result.Fallback = true
			result.Step = "fallback-static"
			logger.Printf("networking: fallback address=%s gateway=%s dns=%s",
				details.CIDR,
				details.Gateway,
				strings.Join(stringifyIPs(details.DNSServers), ","),
			)
		} else {
			err = fmt.Errorf("request DHCP lease on %s: %w", attrs.Name, err)
			result.Error = err.Error()
			return result, err
		}
	} else {
		logger.Printf("networking: acquired DHCP offer on %s xid=%s", attrs.Name, xid.String())

		details, err = detailsFromLease(lease)
		if err != nil {
			err = fmt.Errorf("decode DHCP lease on %s: %w", attrs.Name, err)
			result.Error = err.Error()
			return result, err
		}
	}

	result.Step = "apply-address"
	logger.Printf("networking: applying configuration on %s cidr=%s gateway=%s dns=%s",
		attrs.Name,
		details.CIDR,
		emptyIfBlank(details.Gateway.String(), "<none>"),
		emptyIfBlank(strings.Join(stringifyIPs(details.DNSServers), ","), "<none>"),
	)
	if err := applyLease(link, details); err != nil {
		err = fmt.Errorf("apply DHCP lease on %s: %w", attrs.Name, err)
		result.Error = err.Error()
		return result, err
	}
	logAppliedState(logger, link, "post-configure")

	if details.ResolverContents != "" {
		result.Step = "write-resolver"
		if err := writeResolvConf(details.ResolverContents); err != nil {
			err = fmt.Errorf("write %s: %w", resolvConfPath, err)
			result.Error = err.Error()
			return result, err
		}
		result.ResolverConfig = resolvConfPath
	}

	result.Step = "configured"
	result.Configured = true
	result.Address = details.Address.String()
	result.CIDR = details.CIDR
	if details.Gateway != nil {
		result.Gateway = details.Gateway.String()
	}
	result.DNSServers = stringifyIPs(details.DNSServers)
	result.DomainName = details.DomainName
	result.LeaseSeconds = int64(details.LeaseTime.Seconds())
	result.RenewalSeconds = int64(details.RenewalTime.Seconds())
	result.RebindingSeconds = int64(details.RebindingTime.Seconds())

	logger.Printf("networking: configured %s with %s gateway=%s dns=%s",
		result.InterfaceName,
		result.CIDR,
		emptyIfBlank(result.Gateway, "<none>"),
		emptyIfBlank(strings.Join(result.DNSServers, ","), "<none>"),
	)

	return result, nil
}

func SelectInterface(choices []InterfaceChoice, preferred string) (InterfaceChoice, error) {
	if preferred != "" {
		for _, choice := range choices {
			if choice.Name == preferred {
				return choice, nil
			}
		}
		return InterfaceChoice{}, fmt.Errorf("preferred interface %q not found", preferred)
	}

	for _, choice := range choices {
		if choice.Loopback {
			continue
		}
		if choice.HardwareAddr == "" {
			continue
		}
		return choice, nil
	}

	return InterfaceChoice{}, errors.New("no suitable non-loopback interface found")
}

func selectLink(preferred string) (netlink.Link, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("list links: %w", err)
	}

	choices := make([]InterfaceChoice, 0, len(links))
	linkByName := make(map[string]netlink.Link, len(links))
	for _, link := range links {
		attrs := link.Attrs()
		linkByName[attrs.Name] = link
		choices = append(choices, InterfaceChoice{
			Name:         attrs.Name,
			HardwareAddr: attrs.HardwareAddr.String(),
			Loopback:     attrs.Flags&net.FlagLoopback != 0,
		})
	}

	choice, err := SelectInterface(choices, preferred)
	if err != nil {
		return nil, err
	}

	link, ok := linkByName[choice.Name]
	if !ok {
		return nil, fmt.Errorf("selected interface %q disappeared", choice.Name)
	}

	return link, nil
}

func logInterfaceInventory(logger *log.Logger) {
	if logger == nil {
		return
	}
	links, err := netlink.LinkList()
	if err != nil {
		logger.Printf("networking: unable to enumerate links for debug logging: %v", err)
		return
	}

	summaries := make([]string, 0, len(links))
	for _, link := range links {
		attrs := link.Attrs()
		summaries = append(summaries, fmt.Sprintf("%s(mac=%s oper=%s flags=%s mtu=%d)",
			attrs.Name,
			attrs.HardwareAddr,
			attrs.OperState.String(),
			strings.Join(formatFlags(attrs.Flags), ","),
			attrs.MTU,
		))
	}

	logger.Printf("networking: discovered links => %s", strings.Join(summaries, " | "))
}

func detailsFromLease(lease *nclient4.Lease) (leaseDetails, error) {
	if lease == nil || lease.ACK == nil {
		return leaseDetails{}, errors.New("lease acknowledgement missing")
	}

	ack := lease.ACK
	address := ack.YourIPAddr.To4()
	if address == nil {
		return leaseDetails{}, errors.New("lease did not include an IPv4 address")
	}

	mask := ack.SubnetMask()
	if mask == nil {
		return leaseDetails{}, errors.New("lease did not include a subnet mask")
	}

	prefixLength, bits := mask.Size()
	if bits == 0 {
		return leaseDetails{}, fmt.Errorf("invalid subnet mask %v", mask)
	}

	cidr := (&net.IPNet{IP: address, Mask: mask}).String()
	var gateway net.IP
	if routers := ack.Router(); len(routers) > 0 {
		gateway = routers[0].To4()
	}

	return leaseDetails{
		Address:          address,
		PrefixLength:     prefixLength,
		CIDR:             cidr,
		Gateway:          gateway,
		DNSServers:       ipv4Only(ack.DNS()),
		DomainName:       ack.DomainName(),
		LeaseTime:        ack.IPAddressLeaseTime(0),
		RenewalTime:      ack.IPAddressRenewalTime(0),
		RebindingTime:    ack.IPAddressRebindingTime(0),
		ResolverContents: renderResolvConf(ipv4Only(ack.DNS()), ack.DomainName()),
	}, nil
}

func qemuUsernetFallback() leaseDetails {
	address := net.IPv4(10, 0, 2, 15)
	mask := net.CIDRMask(24, 32)
	dns := []net.IP{net.IPv4(10, 0, 2, 3)}
	return leaseDetails{
		Address:          address,
		PrefixLength:     24,
		CIDR:             (&net.IPNet{IP: address, Mask: mask}).String(),
		Gateway:          net.IPv4(10, 0, 2, 2),
		DNSServers:       dns,
		LeaseTime:        24 * time.Hour,
		RenewalTime:      12 * time.Hour,
		RebindingTime:    18 * time.Hour,
		ResolverContents: renderResolvConf(dns, ""),
	}
}

func applyLease(link netlink.Link, details leaseDetails) error {
	addr, err := netlink.ParseAddr(details.CIDR)
	if err != nil {
		return fmt.Errorf("parse CIDR %s: %w", details.CIDR, err)
	}

	if err := netlink.AddrReplace(link, addr); err != nil {
		return fmt.Errorf("replace address %s: %w", details.CIDR, err)
	}

	if details.Gateway != nil {
		route := &netlink.Route{
			LinkIndex: link.Attrs().Index,
			Gw:        details.Gateway,
		}
		if err := netlink.RouteReplace(route); err != nil {
			return fmt.Errorf("replace default route via %s: %w", details.Gateway, err)
		}
	}

	return nil
}

func renderResolvConf(servers []net.IP, domain string) string {
	if len(servers) == 0 && domain == "" {
		return ""
	}

	var buf bytes.Buffer
	if domain != "" {
		fmt.Fprintf(&buf, "search %s\n", domain)
	}
	for _, server := range servers {
		if server == nil {
			continue
		}
		fmt.Fprintf(&buf, "nameserver %s\n", server.String())
	}
	return buf.String()
}

func writeResolvConf(contents string) error {
	if contents == "" {
		return nil
	}
	if err := os.MkdirAll("/etc", 0o755); err != nil {
		return err
	}
	return os.WriteFile(resolvConfPath, []byte(contents), 0o644)
}

func logAppliedState(logger *log.Logger, link netlink.Link, phase string) {
	if logger == nil || link == nil {
		return
	}
	label := phase
	if label == "" {
		label = "state"
	}
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		logger.Printf("networking: %s address list failed on %s: %v", label, link.Attrs().Name, err)
	} else {
		values := make([]string, 0, len(addrs))
		for _, addr := range addrs {
			values = append(values, addr.String())
		}
		if len(values) == 0 {
			values = append(values, "<none>")
		}
		logger.Printf("networking: %s addresses on %s => %s", label, link.Attrs().Name, strings.Join(values, ","))
	}

	routes, err := netlink.RouteList(link, netlink.FAMILY_V4)
	if err != nil {
		logger.Printf("networking: %s route list failed on %s: %v", label, link.Attrs().Name, err)
		return
	}
	values := make([]string, 0, len(routes))
	for _, route := range routes {
		values = append(values, route.String())
	}
	if len(values) == 0 {
		values = append(values, "<none>")
	}
	logger.Printf("networking: %s routes on %s => %s", label, link.Attrs().Name, strings.Join(values, " | "))
}

func logDHCPWait(logger *log.Logger, ctx context.Context, done <-chan struct{}, iface string, xid dhcpv4.TransactionID) {
	if logger == nil {
		return
	}

	startedAt := time.Now()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ctx.Done():
			logger.Printf("networking: DHCP wait on %s xid=%s ended via context after %s err=%v",
				iface,
				xid.String(),
				time.Since(startedAt).Round(time.Millisecond),
				ctx.Err(),
			)
			return
		case <-ticker.C:
			logger.Printf("networking: still waiting for DHCP on %s xid=%s elapsed=%s",
				iface,
				xid.String(),
				time.Since(startedAt).Round(time.Millisecond),
			)
		}
	}
}

func durationEnv(name string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func intEnv(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func boolEnv(name string, fallback bool) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	switch raw {
	case "":
		return fallback
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func ipv4Only(ips []net.IP) []net.IP {
	result := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			result = append(result, v4)
		}
	}
	return result
}

func stringifyIPs(ips []net.IP) []string {
	result := make([]string, 0, len(ips))
	for _, ip := range ips {
		if ip == nil {
			continue
		}
		result = append(result, ip.String())
	}
	return result
}

func emptyIfBlank(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func formatFlags(flags net.Flags) []string {
	parts := make([]string, 0, 4)
	if flags&net.FlagUp != 0 {
		parts = append(parts, "up")
	}
	if flags&net.FlagBroadcast != 0 {
		parts = append(parts, "broadcast")
	}
	if flags&net.FlagLoopback != 0 {
		parts = append(parts, "loopback")
	}
	if flags&net.FlagPointToPoint != 0 {
		parts = append(parts, "point-to-point")
	}
	if flags&net.FlagMulticast != 0 {
		parts = append(parts, "multicast")
	}
	if len(parts) == 0 {
		return []string{"none"}
	}
	return parts
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func nextTransactionID() dhcpv4.TransactionID {
	counter := xidCounter.Add(1)
	seed := uint32(time.Now().UnixNano()) ^ uint32(os.Getpid()) ^ counter ^ rand.Uint32()
	var xid dhcpv4.TransactionID
	binary.BigEndian.PutUint32(xid[:], seed)
	return xid
}

func newDHCPBroadcastConn(iface string) (net.PacketConn, error) {
	_ = iface
	listenConfig := net.ListenConfig{
		Control: func(network string, address string, raw syscall.RawConn) error {
			var controlErr error
			if err := raw.Control(func(fd uintptr) {
				controlErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_BROADCAST, 1)
			}); err != nil {
				return err
			}
			return controlErr
		},
	}

	return listenConfig.ListenPacket(context.Background(), "udp4", ":68")
}

type dhcpLogger struct {
	logger *log.Logger
}

func (l dhcpLogger) PrintMessage(prefix string, message *dhcpv4.DHCPv4) {
	if l.logger == nil || message == nil {
		return
	}
	l.logger.Printf("dhcp: %s %s", prefix, message.Summary())
}

func (l dhcpLogger) Printf(format string, v ...interface{}) {
	if l.logger == nil {
		return
	}
	l.logger.Printf("dhcp: "+format, v...)
}

func IsPermissionError(err error) bool {
	return errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES)
}
