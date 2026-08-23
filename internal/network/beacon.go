package network

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"reflect"
	"strings"

	"fairdrop/internal/transfer"
	"github.com/hashicorp/mdns"
)

const (
	processSuffixBytes   = 16
	maxInstanceBaseBytes = 28
)

// StartBeacon publishes the previously selected endpoint. It retains ownership
// only after the responder is live and the caller's context remains valid.
func (m *Manager) StartBeacon(ctx context.Context, request transfer.BeaconRequest) error {
	if ctx == nil {
		return transfer.NewError(transfer.ErrBeaconWarning, "beacon start requires a context")
	}
	if err := validateBeaconRequest(request); err != nil {
		return err
	}
	if err := beaconContextError(ctx); err != nil {
		return err
	}
	if err := m.acquireSelectionGate(ctx, beaconContextError); err != nil {
		return err
	}
	defer m.releaseSelectionGate()
	if err := beaconContextError(ctx); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if err := beaconContextError(ctx); err != nil {
		return err
	}
	if m.selected == nil {
		return transfer.NewError(transfer.ErrBeaconWarning, "select a local network address before starting discovery")
	}
	if beaconHandlePresent(m.beacon) {
		return transfer.NewError(transfer.ErrBeaconWarning, "device discovery is already active")
	}

	identity, err := m.processIdentityLocked(ctx)
	if err != nil {
		return err
	}
	instance := identityLabel(request.Instance, identity.host, identity.suffix)
	hostName := hostFQDN(identity.host, identity.suffix)
	ip := net.IP(m.selected.addr.AsSlice())
	service, err := mdns.NewMDNSService(
		instance,
		transfer.BeaconService,
		"local.",
		hostName,
		request.Port,
		[]net.IP{ip},
		[]string{transfer.BeaconVersionTXT},
	)
	if err != nil {
		return transfer.WrapError(transfer.ErrBeaconWarning, "device discovery configuration failed", err)
	}
	if err := beaconContextError(ctx); err != nil {
		return err
	}

	selectedInterface := m.selected.iface
	config := &mdns.Config{
		Zone:   service,
		Iface:  &selectedInterface,
		Logger: log.New(io.Discard, "", 0),
	}
	handle, startErr := m.deps.start(config)
	if contextErr := beaconContextError(ctx); contextErr != nil {
		return cleanupFailedStart(handle, errors.Join(startErr, contextErr))
	}
	if startErr != nil {
		return cleanupFailedStart(handle, startErr)
	}
	if !beaconHandlePresent(handle) {
		return transfer.NewError(transfer.ErrBeaconWarning, "device discovery did not start")
	}

	m.beacon = handle
	return nil
}

// StopBeacon is idempotent. The active handle is forgotten on every return,
// including when Shutdown reports a diagnostic.
func (m *Manager) StopBeacon() error {
	<-m.selectionGate
	defer m.releaseSelectionGate()

	m.mu.Lock()
	defer m.mu.Unlock()
	if !beaconHandlePresent(m.beacon) {
		m.beacon = nil
		return nil
	}

	handle := m.beacon
	m.beacon = nil
	if err := handle.Shutdown(); err != nil {
		return transfer.WrapError(transfer.ErrBeaconWarning, "device discovery cleanup reported a problem", err)
	}
	return nil
}

func validateBeaconRequest(request transfer.BeaconRequest) error {
	if request.Service != transfer.BeaconService {
		return transfer.NewError(transfer.ErrBeaconWarning, "invalid device discovery service")
	}
	if !validLabel(request.Instance) {
		return transfer.NewError(transfer.ErrBeaconWarning, "invalid device discovery instance")
	}
	if request.Port < 1 || request.Port > 65535 {
		return transfer.NewError(transfer.ErrBeaconWarning, "invalid device discovery port")
	}
	if len(request.TXT) != 1 || request.TXT[0] != transfer.BeaconVersionTXT {
		return transfer.NewError(transfer.ErrBeaconWarning, "invalid device discovery metadata")
	}
	return nil
}

func validLabel(value string) bool {
	if len(value) == 0 || len(value) > maxInstanceBaseBytes || value[0] == '-' || value[len(value)-1] == '-' {
		return false
	}
	for _, char := range []byte(value) {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '-' {
			continue
		}
		return false
	}
	return true
}

func (m *Manager) processIdentityLocked(ctx context.Context) (processIdentity, error) {
	if m.identity != nil {
		return *m.identity, nil
	}
	if err := beaconContextError(ctx); err != nil {
		return processIdentity{}, err
	}
	hostName, err := m.deps.hostname()
	if err != nil {
		return processIdentity{}, transfer.WrapError(transfer.ErrBeaconWarning, "device discovery identity is unavailable", err)
	}
	if err := beaconContextError(ctx); err != nil {
		return processIdentity{}, err
	}

	random := make([]byte, processSuffixBytes)
	if _, err := io.ReadFull(m.deps.entropy, random); err != nil {
		return processIdentity{}, transfer.WrapError(transfer.ErrBeaconWarning, "device discovery identity is unavailable", err)
	}
	if err := beaconContextError(ctx); err != nil {
		return processIdentity{}, err
	}

	identity := processIdentity{host: safeHostLabel(hostName), suffix: hex.EncodeToString(random)}
	m.identity = &identity
	return identity, nil
}

func safeHostLabel(hostName string) string {
	folded := strings.ToLower(hostName)
	var builder strings.Builder
	lastHyphen := false
	for _, char := range []byte(folded) {
		valid := (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9')
		if valid {
			builder.WriteByte(char)
			lastHyphen = false
			continue
		}
		if builder.Len() > 0 && !lastHyphen {
			builder.WriteByte('-')
			lastHyphen = true
		}
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "host"
	}
	return result
}

func identityLabel(base, host, suffix string) string {
	const availablePrefixBytes = 30
	host = truncateLabel(host, availablePrefixBytes-len(base)-1)
	return fmt.Sprintf("%s-%s-%s", base, host, suffix)
}

func hostFQDN(host, suffix string) string {
	const availableHostBytes = 30
	return fmt.Sprintf("%s-%s.local.", truncateLabel(host, availableHostBytes), suffix)
}

func truncateLabel(value string, limit int) string {
	if limit < 1 {
		return "h"
	}
	if len(value) > limit {
		value = value[:limit]
	}
	value = strings.Trim(value, "-")
	if value == "" {
		return "h"
	}
	return value
}

func beaconContextError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return transfer.WrapError(transfer.ErrBeaconWarning, "device discovery start was cancelled", err)
	}
	return nil
}

func cleanupFailedStart(handle beaconHandle, cause error) error {
	if beaconHandlePresent(handle) {
		cause = errors.Join(cause, handle.Shutdown())
	}
	if cause == nil {
		cause = errors.New("device discovery start failed")
	}
	return transfer.WrapError(transfer.ErrBeaconWarning, "device discovery did not start", cause)
}

func beaconHandlePresent(handle beaconHandle) bool {
	if handle == nil {
		return false
	}
	value := reflect.ValueOf(handle)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return !value.IsNil()
	default:
		return true
	}
}
