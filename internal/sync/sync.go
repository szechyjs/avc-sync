// Package sync handles reading MDM-managed VPN profiles and syncing them
// into the AWS VPN Client's configuration directory.
package sync

import (
"encoding/json"
"fmt"
"os"
"path/filepath"

"github.com/szechyjs/avc-sync/internal/models"
"github.com/szechyjs/avc-sync/internal/ovpn"
)

const (
awsConfigDir       = ".config/AWSVPNClient"
ovpnSubDir         = "OpenVpnConfigs"
connectionProfiles = "ConnectionProfiles"
profilesVersion    = "1"
)

// Syncer manages the sync lifecycle for a single user session.
type Syncer struct {
homeDir      string
awsDir       string
ovpnDir      string
profilesPath string
}

// New creates a Syncer rooted at the user's home directory.
func New() (*Syncer, error) {
home, err := os.UserHomeDir()
if err != nil {
return nil, fmt.Errorf("unable to determine home directory: %w", err)
}
awsDir := filepath.Join(home, awsConfigDir)
return &Syncer{
homeDir:      home,
awsDir:       awsDir,
ovpnDir:      filepath.Join(awsDir, ovpnSubDir),
profilesPath: filepath.Join(awsDir, connectionProfiles),
}, nil
}

// Sync applies the MDM-managed profile list as the source of truth.
// It writes new ovpn config files, updates ConnectionProfiles, and removes
// profiles that are no longer present in the MDM configuration.
func (s *Syncer) Sync(cfg *models.MDMConfig) error {
if err := os.MkdirAll(s.ovpnDir, 0755); err != nil {
return fmt.Errorf("creating config dirs: %w", err)
}

existing, err := s.loadProfiles()
if err != nil {
return fmt.Errorf("loading ConnectionProfiles: %w", err)
}

mdmByName := make(map[string]models.VpnProfile, len(cfg.VpnProfiles))
for _, p := range cfg.VpnProfiles {
mdmByName[p.ProfileName] = p
}

// Remove ovpn files and registry entries for profiles no longer in MDM.
kept := existing.ConnectionProfiles[:0]
for _, ap := range existing.ConnectionProfiles {
if _, inMDM := mdmByName[ap.ProfileName]; inMDM {
kept = append(kept, ap)
} else {
_ = os.Remove(ap.OvpnConfigFilePath)
}
}
existing.ConnectionProfiles = kept

// Build a lookup of already-registered profile names.
registeredByName := make(map[string]int, len(existing.ConnectionProfiles))
for i, ap := range existing.ConnectionProfiles {
registeredByName[ap.ProfileName] = i
}

// Upsert profiles from MDM.
for _, p := range cfg.VpnProfiles {
ovpnPath := filepath.Join(s.ovpnDir, sanitizeName(p.ProfileName))

if err := os.WriteFile(ovpnPath, []byte(p.OvpnContent), 0644); err != nil {
return fmt.Errorf("writing ovpn config for %q: %w", p.ProfileName, err)
}

parsed := ovpn.Parse(p.OvpnContent)

if idx, exists := registeredByName[p.ProfileName]; exists {
existing.ConnectionProfiles[idx].OvpnConfigFilePath = ovpnPath
existing.ConnectionProfiles[idx].CvpnEndpointId = parsed.CvpnEndpointId
existing.ConnectionProfiles[idx].CvpnEndpointRegion = parsed.CvpnEndpointRegion
existing.ConnectionProfiles[idx].CompatibilityVersion = parsed.CompatibilityVersion
existing.ConnectionProfiles[idx].FederatedAuthType = parsed.FederatedAuthType
} else {
existing.ConnectionProfiles = append(existing.ConnectionProfiles, models.AWSProfile{
ProfileName:          p.ProfileName,
OvpnConfigFilePath:   ovpnPath,
CvpnEndpointId:       parsed.CvpnEndpointId,
CvpnEndpointRegion:   parsed.CvpnEndpointRegion,
CompatibilityVersion: parsed.CompatibilityVersion,
FederatedAuthType:    parsed.FederatedAuthType,
})
}
}

return s.saveProfiles(existing)
}

// loadProfiles reads ConnectionProfiles, returning a valid empty root if the
// file does not yet exist.
func (s *Syncer) loadProfiles() (*models.AWSConnectionProfiles, error) {
data, err := os.ReadFile(s.profilesPath)
if os.IsNotExist(err) {
return &models.AWSConnectionProfiles{
Version:                  profilesVersion,
LastSelectedProfileIndex: -1,
ConnectionProfiles:       []models.AWSProfile{},
}, nil
}
if err != nil {
return nil, err
}
var root models.AWSConnectionProfiles
if err := json.Unmarshal(data, &root); err != nil {
// Treat a corrupt file as empty rather than failing hard.
return &models.AWSConnectionProfiles{
Version:                  profilesVersion,
LastSelectedProfileIndex: -1,
ConnectionProfiles:       []models.AWSProfile{},
}, nil
}
return &root, nil
}

// saveProfiles atomically writes ConnectionProfiles via a temp file.
func (s *Syncer) saveProfiles(root *models.AWSConnectionProfiles) error {
data, err := json.Marshal(root)
if err != nil {
return fmt.Errorf("marshaling ConnectionProfiles: %w", err)
}

tmp, err := os.CreateTemp(s.awsDir, ".ConnectionProfiles-*")
if err != nil {
return fmt.Errorf("creating temp file: %w", err)
}
tmpPath := tmp.Name()

if _, err := tmp.Write(data); err != nil {
tmp.Close()
os.Remove(tmpPath)
return fmt.Errorf("writing temp file: %w", err)
}
if err := tmp.Close(); err != nil {
os.Remove(tmpPath)
return fmt.Errorf("closing temp file: %w", err)
}

if err := os.Rename(tmpPath, s.profilesPath); err != nil {
os.Remove(tmpPath)
return fmt.Errorf("atomic rename of ConnectionProfiles: %w", err)
}
return nil
}

// sanitizeName makes a profile name safe for use as a filename.
func sanitizeName(name string) string {
safe := make([]byte, len(name))
for i := range name {
c := name[i]
if c == '/' || c == '\\' || c == ':' || c == '*' || c == '?' || c == '"' || c == '<' || c == '>' || c == '|' {
safe[i] = '_'
} else {
safe[i] = c
}
}
return string(safe)
}
