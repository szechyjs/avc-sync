package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/szechyjs/avc-sync/internal/models"
	"howett.net/plist"
)

// marshalProfiles serialises a slice of VpnProfile as an XML plist — this is
// exactly what cfprefs.CopyAppValue returns for the VpnProfiles key.
func marshalProfiles(t *testing.T, profiles []models.VpnProfile) []byte {
	t.Helper()
	b, err := plist.Marshal(profiles, plist.XMLFormat)
	require.NoError(t, err)
	return b
}

func TestParseMDMPayload_ForceCleanupTrue(t *testing.T) {
	xml := marshalProfiles(t, nil)

	cfg, err := parseMDMPayload(xml, true)
	require.NoError(t, err)
	assert.True(t, cfg.ForceCleanup, "ForceCleanup should be true when MDM sets it")
}

func TestParseMDMPayload_ForceCleanupFalse(t *testing.T) {
	xml := marshalProfiles(t, nil)

	cfg, err := parseMDMPayload(xml, false)
	require.NoError(t, err)
	assert.False(t, cfg.ForceCleanup, "ForceCleanup should be false when MDM does not set it")
}

func TestParseMDMPayload_ProfilesPopulated(t *testing.T) {
	profiles := []models.VpnProfile{
		{ProfileName: "prod", OvpnContent: "client\n"},
		{ProfileName: "staging", OvpnContent: "client\n"},
	}
	xml := marshalProfiles(t, profiles)

	cfg, err := parseMDMPayload(xml, false)
	require.NoError(t, err)
	require.Len(t, cfg.VpnProfiles, 2)
	assert.Equal(t, "prod", cfg.VpnProfiles[0].ProfileName)
	assert.Equal(t, "staging", cfg.VpnProfiles[1].ProfileName)
}

func TestParseMDMPayload_InvalidXML(t *testing.T) {
	_, err := parseMDMPayload([]byte("not valid plist"), false)
	assert.Error(t, err)
}
