package sync_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/szechyjs/avc-sync/internal/models"
	syncer "github.com/szechyjs/avc-sync/internal/sync"
)

func newTestSyncer(t *testing.T) (*syncer.Syncer, string) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	s, err := syncer.New()
	require.NoError(t, err)
	return s, tmp
}

func readProfiles(t *testing.T, home string) models.AWSConnectionProfiles {
	t.Helper()
	path := filepath.Join(home, ".config", "AWSVPNClient", "ConnectionProfiles")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var root models.AWSConnectionProfiles
	require.NoError(t, json.Unmarshal(data, &root))
	return root
}

func readState(t *testing.T, home string) models.SyncState {
	t.Helper()
	path := filepath.Join(home, ".config", "AWSVPNClient", ".avc-sync-state.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var state models.SyncState
	require.NoError(t, json.Unmarshal(data, &state))
	return state
}

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, data, 0644))
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(path, 0755))
}

func TestSync_CreatesDirectoriesAndConnectionProfiles(t *testing.T) {
	s, home := newTestSyncer(t)

	cfg := &models.MDMConfig{
		VpnProfiles: []models.VpnProfile{
			{ProfileName: "TestVPN", OvpnContent: "client\ndev tun\n"},
		},
	}
	require.NoError(t, s.Sync(cfg))

	root := readProfiles(t, home)
	assert.Equal(t, "1", root.Version)
	assert.Equal(t, -1, root.LastSelectedProfileIndex)
	require.Len(t, root.ConnectionProfiles, 1)

	p := root.ConnectionProfiles[0]
	assert.Equal(t, "TestVPN", p.ProfileName)
	assert.Empty(t, filepath.Ext(p.OvpnConfigFilePath), "ovpn config path should have no extension")
	assert.FileExists(t, p.OvpnConfigFilePath)

	state := readState(t, home)
	assert.Equal(t, []string{"TestVPN"}, state.ManagedProfiles)
}

func TestSync_OnlyRemovesManagedProfiles(t *testing.T) {
	s, home := newTestSyncer(t)

	cfg := &models.MDMConfig{
		VpnProfiles: []models.VpnProfile{
			{ProfileName: "MDM-VPN1", OvpnContent: "client\n"},
			{ProfileName: "MDM-VPN2", OvpnContent: "client\n"},
		},
	}
	require.NoError(t, s.Sync(cfg))

	// Simulate user manually adding a profile.
	root := readProfiles(t, home)
	root.ConnectionProfiles = append(root.ConnectionProfiles, models.AWSProfile{
		ProfileName:          "User-Added",
		OvpnConfigFilePath:   filepath.Join(home, ".config", "AWSVPNClient", "OpenVpnConfigs", "User-Added"),
		CompatibilityVersion: "1",
	})
	data, _ := json.Marshal(root)
	mustWriteFile(t, filepath.Join(home, ".config", "AWSVPNClient", "ConnectionProfiles"), data)
	mustWriteFile(t, filepath.Join(home, ".config", "AWSVPNClient", "OpenVpnConfigs", "User-Added"), []byte("client\n"))

	// Second sync: MDM drops MDM-VPN2.
	cfg.VpnProfiles = cfg.VpnProfiles[:1]
	require.NoError(t, s.Sync(cfg))

	result := readProfiles(t, home)
	names := map[string]bool{}
	for _, p := range result.ConnectionProfiles {
		names[p.ProfileName] = true
	}

	assert.True(t, names["MDM-VPN1"], "MDM-VPN1 should still be present")
	assert.False(t, names["MDM-VPN2"], "MDM-VPN2 should have been removed")
	assert.True(t, names["User-Added"], "User-Added should be preserved")
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
		require.NoError(t, s.Sync(cfg), "run %d", i+1)
	}

	root := readProfiles(t, home)
	assert.Len(t, root.ConnectionProfiles, 2)
}

func TestSync_RemovesMDMManagedProfileWhenDroppedFromPayload(t *testing.T) {
	s, home := newTestSyncer(t)

	cfg := &models.MDMConfig{
		VpnProfiles: []models.VpnProfile{
			{ProfileName: "Keep", OvpnContent: "client\n"},
			{ProfileName: "Drop", OvpnContent: "client\n"},
		},
	}
	require.NoError(t, s.Sync(cfg))

	initial := readProfiles(t, home)
	var droppedPath string
	for _, p := range initial.ConnectionProfiles {
		if p.ProfileName == "Drop" {
			droppedPath = p.OvpnConfigFilePath
		}
	}

	cfg.VpnProfiles = cfg.VpnProfiles[:1]
	require.NoError(t, s.Sync(cfg))

	root := readProfiles(t, home)
	require.Len(t, root.ConnectionProfiles, 1)
	assert.Equal(t, "Keep", root.ConnectionProfiles[0].ProfileName)
	assert.NoFileExists(t, droppedPath, "dropped profile's config file should be deleted")
}

func TestSync_FirstRunDoesNotRemovePreexistingProfiles(t *testing.T) {
	s, home := newTestSyncer(t)

	awsDir := filepath.Join(home, ".config", "AWSVPNClient")
	mustMkdirAll(t, filepath.Join(awsDir, "OpenVpnConfigs"))

	preExisting := models.AWSConnectionProfiles{
		Version:                  "1",
		LastSelectedProfileIndex: -1,
		ConnectionProfiles: []models.AWSProfile{
			{ProfileName: "Pre-Existing", OvpnConfigFilePath: filepath.Join(awsDir, "OpenVpnConfigs", "Pre-Existing"), CompatibilityVersion: "1"},
		},
	}
	data, _ := json.Marshal(preExisting)
	mustWriteFile(t, filepath.Join(awsDir, "ConnectionProfiles"), data)

	cfg := &models.MDMConfig{
		VpnProfiles: []models.VpnProfile{
			{ProfileName: "MDM-Profile", OvpnContent: "client\n"},
		},
	}
	require.NoError(t, s.Sync(cfg))

	root := readProfiles(t, home)
	names := map[string]bool{}
	for _, p := range root.ConnectionProfiles {
		names[p.ProfileName] = true
	}

	assert.True(t, names["Pre-Existing"], "Pre-Existing profile should be preserved on first run")
	assert.True(t, names["MDM-Profile"], "MDM-Profile should have been added")
}

func TestSync_HandlesEmptyMDMConfig(t *testing.T) {
	s, home := newTestSyncer(t)

	require.NoError(t, s.Sync(&models.MDMConfig{
		VpnProfiles: []models.VpnProfile{{ProfileName: "ToBeRemoved", OvpnContent: "client\n"}},
	}))
	require.NoError(t, s.Sync(&models.MDMConfig{VpnProfiles: []models.VpnProfile{}}))

	root := readProfiles(t, home)
	assert.Empty(t, root.ConnectionProfiles)

	state := readState(t, home)
	assert.Empty(t, state.ManagedProfiles)
}

func TestSync_ForceCleanupRemovesUserAddedProfiles(t *testing.T) {
	s, home := newTestSyncer(t)

	require.NoError(t, s.Sync(&models.MDMConfig{
		VpnProfiles: []models.VpnProfile{{ProfileName: "MDM-VPN", OvpnContent: "client\n"}},
	}))

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

	require.NoError(t, s.Sync(&models.MDMConfig{
		VpnProfiles:  []models.VpnProfile{{ProfileName: "MDM-VPN", OvpnContent: "client\n"}},
		ForceCleanup: true,
	}))

	result := readProfiles(t, home)
	require.Len(t, result.ConnectionProfiles, 1)
	assert.Equal(t, "MDM-VPN", result.ConnectionProfiles[0].ProfileName)
	assert.NoFileExists(t, userOvpnPath, "user profile's ovpn file should be deleted during ForceCleanup")
}

func TestSync_ForceCleanupFalsePreservesUserProfiles(t *testing.T) {
	s, home := newTestSyncer(t)

	require.NoError(t, s.Sync(&models.MDMConfig{
		VpnProfiles: []models.VpnProfile{{ProfileName: "MDM-VPN", OvpnContent: "client\n"}},
	}))

	root := readProfiles(t, home)
	root.ConnectionProfiles = append(root.ConnectionProfiles, models.AWSProfile{
		ProfileName:          "User-VPN",
		OvpnConfigFilePath:   filepath.Join(home, ".config", "AWSVPNClient", "OpenVpnConfigs", "User-VPN"),
		CompatibilityVersion: "1",
	})
	data, _ := json.Marshal(root)
	mustWriteFile(t, filepath.Join(home, ".config", "AWSVPNClient", "ConnectionProfiles"), data)

	require.NoError(t, s.Sync(&models.MDMConfig{
		VpnProfiles:  []models.VpnProfile{{ProfileName: "MDM-VPN", OvpnContent: "client\n"}},
		ForceCleanup: false,
	}))

	result := readProfiles(t, home)
	names := map[string]bool{}
	for _, p := range result.ConnectionProfiles {
		names[p.ProfileName] = true
	}
	assert.True(t, names["User-VPN"], "User-VPN should be preserved when ForceCleanup is false")
}
