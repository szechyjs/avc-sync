package sync_test

import (
"encoding/json"
"os"
"path/filepath"
"testing"

"github.com/szechyjs/avc-sync/internal/models"
syncer "github.com/szechyjs/avc-sync/internal/sync"
)

// newTestSyncer creates a Syncer pointed at a temporary directory,
// isolating tests from the real ~/.config/AWSVPNClient.
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
t.Errorf("expected Version=1, got %q", root.Version)
}
if root.LastSelectedProfileIndex != -1 {
t.Errorf("expected LastSelectedProfileIndex=-1, got %d", root.LastSelectedProfileIndex)
}
if len(root.ConnectionProfiles) != 1 {
t.Fatalf("expected 1 profile, got %d", len(root.ConnectionProfiles))
}

p := root.ConnectionProfiles[0]
if p.ProfileName != "TestVPN" {
t.Errorf("expected ProfileName=TestVPN, got %q", p.ProfileName)
}
if p.CompatibilityVersion != "1" {
t.Errorf("expected CompatibilityVersion=1, got %q", p.CompatibilityVersion)
}
if p.FederatedAuthType != 0 {
t.Errorf("expected FederatedAuthType=0, got %d", p.FederatedAuthType)
}

// Config file should exist with no extension.
if _, err := os.Stat(p.OvpnConfigFilePath); err != nil {
t.Errorf("ovpn config file missing: %v", err)
}
if filepath.Ext(p.OvpnConfigFilePath) != "" {
t.Errorf("ovpn config file should have no extension, got %q", p.OvpnConfigFilePath)
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

func TestSync_RemovesStaleProfiles(t *testing.T) {
s, home := newTestSyncer(t)

cfg := &models.MDMConfig{
VpnProfiles: []models.VpnProfile{
{ProfileName: "Keep", OvpnContent: "client\n"},
{ProfileName: "Remove", OvpnContent: "client\n"},
},
}
if err := s.Sync(cfg); err != nil {
t.Fatalf("initial Sync() error: %v", err)
}

initial := readProfiles(t, home)
var removedPath string
for _, p := range initial.ConnectionProfiles {
if p.ProfileName == "Remove" {
removedPath = p.OvpnConfigFilePath
}
}

cfg.VpnProfiles = cfg.VpnProfiles[:1] // keep only "Keep"
if err := s.Sync(cfg); err != nil {
t.Fatalf("second Sync() error: %v", err)
}

root := readProfiles(t, home)
if len(root.ConnectionProfiles) != 1 {
t.Fatalf("expected 1 profile after removal, got %d", len(root.ConnectionProfiles))
}
if root.ConnectionProfiles[0].ProfileName == "Remove" {
t.Error("stale profile 'Remove' was not deleted from ConnectionProfiles")
}
if _, err := os.Stat(removedPath); !os.IsNotExist(err) {
t.Errorf("stale config file was not removed from disk: %s", removedPath)
}
}

func TestSync_HandlesEmptyMDMConfig(t *testing.T) {
s, home := newTestSyncer(t)

cfg := &models.MDMConfig{
VpnProfiles: []models.VpnProfile{
{ProfileName: "Initial", OvpnContent: "client\n"},
},
}
if err := s.Sync(cfg); err != nil {
t.Fatalf("initial Sync() error: %v", err)
}

empty := &models.MDMConfig{VpnProfiles: []models.VpnProfile{}}
if err := s.Sync(empty); err != nil {
t.Fatalf("empty Sync() error: %v", err)
}

root := readProfiles(t, home)
if len(root.ConnectionProfiles) != 0 {
t.Errorf("expected 0 profiles after empty sync, got %d", len(root.ConnectionProfiles))
}
}
