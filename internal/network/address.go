package network

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"fairdrop/internal/transfer"
)

func eligibleCandidates(snapshots []interfaceSnapshot) []addressCandidate {
	seen := make(map[string]bool)
	var candidates []addressCandidate
	for _, snapshot := range snapshots {
		if !eligibleInterface(snapshot.iface) {
			continue
		}
		for _, raw := range snapshot.addresses {
			addr, prefix, ok := canonicalIPv4(raw)
			if !ok || excludedAddress(addr, prefix) || addressRank(addr) < 0 {
				continue
			}
			key := strconv.Itoa(snapshot.iface.Index) + "\x00" + snapshot.iface.Name + "\x00" + addr.String()
			if seen[key] {
				continue
			}
			seen[key] = true
			candidates = append(candidates, addressCandidate{iface: snapshot.iface, addr: addr, rank: addressRank(addr)})
		}
	}
	slices.SortFunc(candidates, func(a, b addressCandidate) int {
		if candidateLess(a, b) {
			return -1
		}
		if candidateLess(b, a) {
			return 1
		}
		return 0
	})
	return candidates
}

func candidateID(candidate addressCandidate) string {
	value := strconv.Itoa(candidate.iface.Index) + "\x00" + candidate.iface.Name + "\x00" + candidate.addr.String()
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:16])
}

func candidateLabel(name string) string {
	var builder strings.Builder
	for _, r := range name {
		if builder.Len() >= 64 {
			break
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		builder.WriteRune(r)
	}
	label := strings.TrimSpace(builder.String())
	if label == "" {
		return "Local network"
	}
	return label
}

type interfaceSnapshot struct {
	iface     net.Interface
	addresses []net.Addr
}

type addressCandidate struct {
	iface net.Interface
	addr  netip.Addr
	rank  int
}

func selectCandidate(snapshots []interfaceSnapshot) (addressCandidate, bool) {
	candidates := eligibleCandidates(snapshots)
	if len(candidates) == 0 {
		return addressCandidate{}, false
	}
	return candidates[0], true
}

func eligibleInterface(iface net.Interface) bool {
	required := net.FlagUp | net.FlagBroadcast
	if iface.Flags&required != required {
		return false
	}
	if iface.Flags&(net.FlagLoopback|net.FlagPointToPoint) != 0 {
		return false
	}
	folded := strings.ToLower(iface.Name)
	return !strings.Contains(folded, "docker") &&
		!strings.Contains(folded, "veth") &&
		!strings.Contains(folded, "tun")
}

func canonicalIPv4(raw net.Addr) (netip.Addr, netip.Prefix, bool) {
	if raw == nil {
		return netip.Addr{}, netip.Prefix{}, false
	}

	var addr netip.Addr
	var bits int
	switch value := raw.(type) {
	case *net.IPNet:
		if value == nil {
			return netip.Addr{}, netip.Prefix{}, false
		}
		var ok bool
		addr, ok = netip.AddrFromSlice(value.IP)
		if !ok {
			return netip.Addr{}, netip.Prefix{}, false
		}
		var totalBits int
		bits, totalBits = value.Mask.Size()
		if totalBits != 32 && totalBits != 128 {
			return netip.Addr{}, netip.Prefix{}, false
		}
	case *net.IPAddr:
		if value == nil || value.Zone != "" {
			return netip.Addr{}, netip.Prefix{}, false
		}
		var ok bool
		addr, ok = netip.AddrFromSlice(value.IP)
		if !ok {
			return netip.Addr{}, netip.Prefix{}, false
		}
		bits = addr.BitLen()
	default:
		prefix, err := netip.ParsePrefix(raw.String())
		if err != nil {
			return netip.Addr{}, netip.Prefix{}, false
		}
		addr = prefix.Addr()
		bits = prefix.Bits()
	}

	if addr.Is4In6() {
		addr = addr.Unmap()
		if bits >= 96 {
			bits -= 96
		}
	}
	if !addr.Is4() || bits < 0 || bits > 32 {
		return netip.Addr{}, netip.Prefix{}, false
	}
	return addr, netip.PrefixFrom(addr, bits), true
}

func excludedAddress(addr netip.Addr, prefix netip.Prefix) bool {
	return !addr.IsValid() || addr.IsUnspecified() || addr.IsLoopback() ||
		addr.IsMulticast() || isNetworkAddress(addr, prefix) || isBroadcast(addr, prefix)
}

func isNetworkAddress(addr netip.Addr, prefix netip.Prefix) bool {
	return prefix.IsValid() && prefix.Bits() <= 30 && prefix.Masked().Addr() == addr
}

func isBroadcast(addr netip.Addr, prefix netip.Prefix) bool {
	bytes := addr.As4()
	if bytes == [4]byte{255, 255, 255, 255} {
		return true
	}
	if !prefix.IsValid() || prefix.Bits() >= 31 {
		return false
	}
	base := prefix.Masked().Addr().As4()
	bits := prefix.Bits()
	for index := range base {
		remaining := bits - index*8
		var mask byte
		switch {
		case remaining >= 8:
			mask = 0xff
		case remaining > 0:
			mask = byte(0xff << (8 - remaining))
		}
		base[index] |= ^mask
	}
	return bytes == base
}

func addressRank(addr netip.Addr) int {
	switch {
	case addr.IsPrivate():
		return 0
	case addr.IsGlobalUnicast():
		return 1
	case addr.IsLinkLocalUnicast():
		return 2
	default:
		return -1
	}
}

func candidateLess(left, right addressCandidate) bool {
	if left.rank != right.rank {
		return left.rank < right.rank
	}
	if left.iface.Index != right.iface.Index {
		return left.iface.Index < right.iface.Index
	}
	leftFolded, rightFolded := strings.ToLower(left.iface.Name), strings.ToLower(right.iface.Name)
	if leftFolded != rightFolded {
		return leftFolded < rightFolded
	}
	if left.iface.Name != right.iface.Name {
		return left.iface.Name < right.iface.Name
	}
	return left.addr.Compare(right.addr) < 0
}

func unavailableError(causes []error) error {
	if len(causes) == 0 {
		return transfer.NewError(transfer.ErrNetworkUnavailable, "no usable local network address was found")
	}
	return transfer.WrapError(
		transfer.ErrNetworkUnavailable,
		"no usable local network address was found",
		errors.Join(causes...),
	)
}
