package sync_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/szechyjs/avc-sync/internal/models"
	syncer "github.com/szechyjs/avc-sync/internal/sync"
)

func newTestSyncer(t *testing.T) (*syncer.Syncer, string) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	s, err := syncer.New()
	if err != nil {
		t.Fatalf("syncer.New(): %v", err)
	}
	return s, tmp
}

func readProfiles(t *testing.T, home string) models.AWSConnectionProfiles {
	t.Helper()
	path := filepath.Join(home, ".config", "AWSVPNClient", "ConnectionProfiles")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading ConnectionProfiles: %v", err)
	}
	var root models.AWSConnectionProfiles
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("parsing ConnectionProfiles: %v", err)
	}
	return root
}

func readState(t *testing.T, home string) models.SyncState {
	t.Helper()
	path := filepath.Join(home, ".config", "AWSVPNClient", ".avc-sync-state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading state file: %v", err)
	}
	var state models.SyncState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("parsing state file: %v", err)
	}
	return state
}

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func TestSync_CreatesDirectoriesAndConnectionProfiles(t *testing.T) {
	s, home := newTestSyncer(t)

	cfg := &models.MDMConfig{
		VpnProfiles: []models.VpnProfile{
			{ProfileName: "TestVPN", OvpnContent: "client\ndev tun\n"},
		},
	}
	if err := s.Sync(cfg); err != nil {
		t.Fatalf("Sync() error: %v", err)
	}

	root := readProfiles(t, home)
	if root.Version != "1" {
		t.Errorf("Version: got %q, want \"1\"", root.Version)
	}
	if root.LastSelectedProfileIndex != -1 {
		t.Errorf("LastSelectedProfileIndex: got %d, want -1", root.LastSelectedProfileIndex)
	}
	if len(root.ConnectionProfiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(root.ConnectionProfiles))
	}
	p := root.ConnectionProfiles[0]
	if p.ProfileName != "TestVPN" {
		t.Errorf("ProfileName: got %q", p.ProfileName)
	}
	if filepath.Ext(p.OvpnConfigFilePath) != "" {
		t.Errorf("expected no file extension, got %q", p.OvpnConfigFilePath)
	}
	if _, err := os.Stat(p.OvpnConfigFilePath); err != nil {
		t.Errorf("ovpn config file missing: %v", err)
	}

	// State file should record the managed profile.
	state := readState(t, home)
	if len(state.ManagedProfiles) != 1 || state.ManagedProfiles[0] != "TestVPN" {
		t.Errorf("state ManagedProfiles: got %v, want [TestVPN]", state.ManagedProfiles)
	}
}

func TestSync_OnlyRemovesManagedProfiles(t *testing.T) {
	s, home := newTestSyncer(t)

	// First sync: MDM provides two profiles.
	cfg := &models.MDMConfig{
		VpnProfiles: []models.VpnProfile{
			{ProfileName: "MDM-VPN1", OvpnContent: "client\n"},
			{ProfileName: "MDM-VPN2", OvpnContent: "client\n"},
		},
	}
	if err := s.Sync(cfg); err != nil {
		t.Fatalf("initial Sync() error: %v", err)
	}

	// Simulate the user manually adding a profile by injecting it directly
	// into ConnectionProfiles (bypassing avc-sync / the state file).
	root := readProfiles(t, home)
	root.ConnectionProfiles = append(root.ConnectionProfiles, models.AWSProfile{
		ProfileName:          "User-Added",
		OvpnConfigFilePath:   filepath.Join(home, ".config", "AWSVPNClient", "OpenVpnConfigs", "User-Added"),
		CompatibilityVersion: "1",
	})
	data, _ := json.Marshal(root)
	mustWriteFile(t, filepath.Join(home, ".config", "AWSVPNClient", "ConnectionProfiles"), data)
	// Also write the ovpn file so removal can be verified.
	mustWriteFile(t, filepath.Join(home, ".config", "AWSVPNClient", "OpenVpnConfigs", "User-Added"), []byte("client\n"))

	// Second sync: MDM removes MDM-VPN2 but keeps MDM-VPN1.
	cfg.VpnProfiles = cfg.VpnProfiles[:1]
	if err := s.Sync(cfg); err != nil {
		t.Fatalf("second Sync() error: %v", err)
	}

	result := readProfiles(t, home)
	names := map[string]bool{}
	for _, p := range result.ConnectionProfiles {
		names[p.ProfileName] = true
	}

	if !names["MDM-VPN1"] {
		t.Error("MDM-VPN1 should still be present")
	}
	if names["MDM-VPN2"] {
		t.Error("MDM-VPN2 should have been removed (was MDM-managed)")
	}
	if !names["User-Added"] {
		t.Error("User-Added should be preserved (was not MDM-managed)")
	}
}

func TestSync_IdempotentOnRepeatedRuns(t *testing.T) {
	s, home := newTestSyncer(t)
	cfg := &models.MDMConfig{
		VpnProfiles: []models.VpnProfile{
			{ProfileName: "VPN1", OvpnContent: "client\n"},
			{ProfileName: "VPN2", OvpnContent: "client\n"},
		},
	}
	for i := 0; i < 5; i++ {
		if err := s.Sync(cfg); err != nil {
			t.Fatalf("Sync() run %d error: %v", i+1, err)
		}
	}
	root := readProfiles(t, home)
	if len(root.ConnectionProfiles) != 2 {
		t.Errorf("expected 2 profiles after 5 runs, got %d", len(root.ConnectionProfiles))
	}
}

func TestSync_RemovesMDMManagedProfileWhenDroppedFromPayload(t *testing.T) {
	s, home := newTestSyncer(t)

	cfg := &models.MDMConfig{
		VpnProfiles: []models.VpnProfile{
			{ProfileName: "Keep", OvpnContent: "client\n"},
			{ProfileName: "Drop", OvpnContent: "client\n"},
		},
	}
	if err := s.Sync(cfg); err != nil {
		t.Fatalf("initial Sync() error: %v", err)
	}

	initial := readProfiles(t, home)
	var droppedPath string
	for _, p := range initial.ConnectionProfiles {
		if p.ProfileName == "Drop" {
			droppedPath = p.OvpnConfigFilePath
		}
	}

	cfg.VpnProfiles = cfg.VpnProfiles[:1]
	if err := s.Sync(cfg); err != nil {
		t.Fatalf("second Sync() error: %v", err)
	}

	root := readProfiles(t, home)
	if len(root.ConnectionProfiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(root.ConnectionProfiles))
	}
	if root.ConnectionProfiles[0].ProfileName != "Keep" {
		t.Errorf("expected Keep to remain, got %q", root.ConnectionProfiles[0].ProfileName)
	}
	if _, err := os.Stat(droppedPath); !os.IsNotExist(err) {
		t.Errorf("dropped profile's config file should have been deleted: %s", droppedPath)
	}
}

func TestSync_FirstRunDoesNotRemovePreexistingProfiles(t *testing.T) {
	s, home := newTestSyncer(t)

	awsDir := filepath.Join(home, ".config", "AWSVPNClient")
	mustMkdirAll(t, filepath.Join(awsDir, "OpenVpnConfigs"))

	// Pre-populate ConnectionProfiles as if the user already had a profile.
	preExisting := models.AWSConnectionProfiles{
		Version:                  "1",
		LastSelectedProfileIndex: -1,
		ConnectionProfiles: []models.AWSProfile{
			{ProfileName: "Pre-Existing", OvpnConfigFilePath: filepath.Join(awsDir, "OpenVpnConfigs", "Pre-Existing"), CompatibilityVersion: "1"},
		},
	}
	data, _ := json.Marshal(preExisting)
	mustWriteFile(t, filepath.Join(awsDir, "ConnectionProfiles"), data)

	// No state file exists — first run of avc-sync on this machine.
	cfg := &models.MDMConfig{
		VpnProfiles: []models.VpnProfile{
			{ProfileName: "MDM-Profile", OvpnContent: "client\n"},
		},
	}
	if err := s.Sync(cfg); err != nil {
		t.Fatalf("Sync() error: %v", err)
	}

	root := readProfiles(t, home)
	names := map[string]bool{}
	for _, p := range root.ConnectionProfiles {
		names[p.ProfileName] = true
	}

	if !names["Pre-Existing"] {
		t.Error("Pre-Existing profile should be preserved on first run")
	}
	if !names["MDM-Profile"] {
		t.Error("MDM-Profile should have been added")
	}
}

func TestSync_HandlesEmptyMDMConfig(t *testing.T) {
	s, home := newTestSyncer(t)

	cfg := &models.MDMConfig{
		VpnProfiles: []models.VpnProfile{
			{ProfileName: "ToBeRemoved", OvpnContent: "client\n"},
		},
	}
	if err := s.Sync(cfg); err != nil {
		t.Fatalf("initial Sync() error: %v", err)
	}

	if err := s.Sync(&models.MDMConfig{VpnProfiles: []models.VpnProfile{}}); err != nil {
		t.Fatalf("empty Sync() error: %v", err)
	}

	root := readProfiles(t, home)
	if len(root.ConnectionProfiles) != 0 {
		t.Errorf("expected 0 profiles after empty sync, got %d", len(root.ConnectionProfiles))
	}

	state := readState(t, home)
	if len(state.ManagedProfiles) != 0 {
		t.Errorf("expected empty managed set, got %v", state.ManagedProfiles)
	}
}

func TestSync_ForceCleanupRemovesUserAddedProfiles(t *testing.T) {
	s, home := newTestSyncer(t)

	// First sync via MDM to establish one managed profile.
	if err := s.Sync(&models.MDMConfig{
		VpnProfiles: []models.VpnProfile{
			{ProfileName: "MDM-VPN", OvpnContent: "client\n"},
		},
	}); err != nil {
		t.Fatalf("initial Sync() error: %v", err)
	}

	// Simulate user adding a profile directly.
	root := readProfiles(t, home)
	userOvpnPath := filepath.Join(home, ".config", "AWSVPNClient", "OpenVpnConfigs", "User-VPN")
	mustWriteFile(t, userOvpnPath, []byte("client\n"))
	root.ConnectionProfiles = append(root.ConnectionProfiles, models.AWSProfile{
		ProfileName:          "User-VPN",
		OvpnConfigFilePath:   userOvpnPath,
		CompatibilityVersion: "1",
	})
	data, _ := json.Marshal(root)
	mustWriteFile(t, filepath.Join(home, ".config", "AWSVPNClient", "ConnectionProfiles"), data)

	// Sync with ForceCleanup — should remove User-VPN too.
	if err := s.Sync(&models.MDMConfig{
		VpnProfiles:  []models.VpnProfile{{ProfileName: "MDM-VPN", OvpnContent: "client\n"}},
		ForceCleanup: true,
	}); err != nil {
		t.Fatalf("ForceCleanup Sync() error: %v", err)
	}

	result := readProfiles(t, home)
	if len(result.ConnectionProfiles) != 1 {
		t.Fatalf("expected 1 profile after ForceCleanup, got %d", len(result.ConnectionProfiles))
	}
	if result.ConnectionProfiles[0].ProfileName != "MDM-VPN" {
		t.Errorf("expected only MDM-VPN to remain, got %q", result.ConnectionProfiles[0].ProfileName)
	}
	if _, err := os.Stat(userOvpnPath); !os.IsNotExist(err) {
		t.Error("user profile's ovpn file should have been deleted during ForceCleanup")
	}
}

func TestSync_ForceCleanupFalsePreservesUserProfiles(t *testing.T) {
	s, home := newTestSyncer(t)

	// Establish a managed profile.
	if err := s.Sync(&models.MDMConfig{
		VpnProfiles: []models.VpnProfile{
			{ProfileName: "MDM-VPN", OvpnContent: "client\n"},
		},
	}); err != nil {
		t.Fatalf("initial Sync() error: %v", err)
	}

	// Add a user profile directly.
	root := readProfiles(t, home)
	root.ConnectionProfiles = append(root.ConnectionProfiles, models.AWSProfile{
		ProfileName:          "User-VPN",
		OvpnConfigFilePath:   filepath.Join(home, ".config", "AWSVPNClient", "OpenVpnConfigs", "User-VPN"),
		CompatibilityVersion: "1",
	})
	data, _ := json.Marshal(root)
	mustWriteFile(t, filepath.Join(home, ".config", "AWSVPNClient", "ConnectionProfiles"), data)

	// Sync without ForceCleanup — user profile must survive.
	if err := s.Sync(&models.MDMConfig{
		VpnProfiles:  []models.VpnProfile{{ProfileName: "MDM-VPN", OvpnContent: "client\n"}},
		ForceCleanup: false,
	}); err != nil {
		t.Fatalf("Sync() error: %v", err)
	}

	result := readProfiles(t, home)
	names := map[string]bool{}
	for _, p := range result.ConnectionProfiles {
		names[p.ProfileName] = true
	}
	if !names["User-VPN"] {
		t.Error("User-VPN should be preserved when ForceCleanup is false")
	}
}
