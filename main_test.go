package main

import (
	"testing"

	"github.com/szechyjs/avc-sync/internal/models"
	"howett.net/plist"
)

// marshalProfiles serialises a slice of VpnProfile as an XML plist — this is
// exactly what cfprefs.CopyAppValue returns for the VpnProfiles key.
func marshalProfiles(t *testing.T, profiles []models.VpnProfile) []byte {
	t.Helper()
	b, err := plist.Marshal(profiles, plist.XMLFormat)
	if err != nil {
		t.Fatalf("marshal profiles: %v", err)
	}
	return b
}

func TestParseMDMPayload_ForceCleanupTrue(t *testing.T) {
	xml := marshalProfiles(t, nil)

	cfg, err := parseMDMPayload(xml, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.ForceCleanup {
		t.Error("ForceCleanup should be true when MDM sets it, got false")
	}
}

func TestParseMDMPayload_ForceCleanupFalse(t *testing.T) {
	xml := marshalProfiles(t, nil)

	cfg, err := parseMDMPayload(xml, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ForceCleanup {
		t.Error("ForceCleanup should be false when MDM does not set it, got true")
	}
}

func TestParseMDMPayload_ProfilesPopulated(t *testing.T) {
	profiles := []models.VpnProfile{
		{ProfileName: "prod", OvpnContent: "client\n"},
		{ProfileName: "staging", OvpnContent: "client\n"},
	}
	xml := marshalProfiles(t, profiles)

	cfg, err := parseMDMPayload(xml, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.VpnProfiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(cfg.VpnProfiles))
	}
	if cfg.VpnProfiles[0].ProfileName != "prod" {
		t.Errorf("expected first profile name 'prod', got %q", cfg.VpnProfiles[0].ProfileName)
	}
	if cfg.VpnProfiles[1].ProfileName != "staging" {
		t.Errorf("expected second profile name 'staging', got %q", cfg.VpnProfiles[1].ProfileName)
	}
}

func TestParseMDMPayload_InvalidXML(t *testing.T) {
	_, err := parseMDMPayload([]byte("not valid plist"), false)
	if err == nil {
		t.Error("expected an error for invalid plist input, got nil")
	}
}
