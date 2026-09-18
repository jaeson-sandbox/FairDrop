package transfer

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/netip"
)

// This file holds session identity and the URL a receiver is handed: two
// independent random draws, and the capability URL built from one of them.
// The session ID and the capability token are never derived from each other --
// the token is an HTTP capability and the ID is correlation the UI is shown,
// so learning either must teach nothing about the other.

const (
	// identityBytes is the width of each independent random identifier. Two
	// separate draws of this many bytes give a session ID and a capability
	// token of 128 bits each, which is the contract's floor.
	identityBytes = 16

	// downloadPathPrefix must stay identical to the route internal/server
	// registers as "/download/{token}" in handler.go. The two cannot share a
	// constant without inverting the dependency direction -- the server
	// imports this package -- so a change to either one has to move both.
	downloadPathPrefix = "/download/"
)

// newIdentity draws the session ID and the capability token as two independent
// values. Neither is derived from the other: the token is an HTTP capability
// and the session ID is correlation the UI is shown, so learning either one
// must teach nothing about the other.
func (c *Coordinator) newIdentity() (SessionID, CapabilityToken, error) {
	id, err := c.randomHex()
	if err != nil {
		return "", "", err
	}
	token, err := c.randomHex()
	if err != nil {
		return "", "", err
	}
	return SessionID(id), CapabilityToken(token), nil
}

func (c *Coordinator) randomHex() (string, error) {
	source := c.entropy
	if source == nil {
		source = rand.Reader
	}
	raw := make([]byte, identityBytes)
	if _, err := io.ReadFull(source, raw); err != nil {
		// Called from Stage before c.mu.Lock(): no state has changed and no
		// resource has been acquired, so this is a pre-transfer setup failure,
		// not an interrupted transfer (D-025).
		return "", WrapError(ErrSetupFailed, "FairDrop could not create a transfer session", err)
	}
	return hex.EncodeToString(raw), nil
}

// capabilityURL is the one place the token becomes a shareable string.
func capabilityURL(address netip.Addr, port int, token CapabilityToken) string {
	endpoint := netip.AddrPortFrom(address, uint16(port))
	return "http://" + endpoint.String() + downloadPathPrefix + string(token)
}
