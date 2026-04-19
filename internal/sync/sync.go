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
	stateFile          = ".avc-sync-state.json"
	profilesVersion    = "1"
)

// Syncer manages the sync lifecycle for a single user session.
type Syncer struct {
	homeDir      string
	awsDir       string
	ovpnDir      string
	profilesPath string
	statePath    string
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
		statePath:    filepath.Join(awsDir, stateFile),
	}, nil
}

// Sync reconciles ConnectionProfiles against the MDM payload.
//
// Only profiles previously written by avc-sync (tracked in the state file)
// are eligible for removal. Profiles the user added manually are never
// touched. New and updated profiles from MDM are always applied.
func (s *Syncer) Sync(cfg *models.MDMConfig) error {
	if err := os.MkdirAll(s.ovpnDir, 0755); err != nil {
		return fmt.Errorf("creating config dirs: %w", err)
	}

	state, err := s.loadState()
	if err != nil {
		return fmt.Errorf("loading sync state: %w", err)
	}

	existing, err := s.loadProfiles()
	if err != nil {
		return fmt.Errorf("loading ConnectionProfiles: %w", err)
	}

	// Build lookup sets.
	mdmByName := make(map[string]models.VpnProfile, len(cfg.VpnProfiles))
	for _, p := range cfg.VpnProfiles {
		mdmByName[p.ProfileName] = p
	}

	managedSet := make(map[string]struct{}, len(state.ManagedProfiles))
	for _, name := range state.ManagedProfiles {
		managedSet[name] = struct{}{}
	}

	// Determine which profiles to remove:
	// - Normally: only profiles previously written by avc-sync (in managed set)
	//   that are no longer in the MDM payload.
	// - ForceCleanup: remove every profile not in the MDM payload, including
	//   ones the user added manually.
	kept := existing.ConnectionProfiles[:0]
	for _, ap := range existing.ConnectionProfiles {
		_, isManaged := managedSet[ap.ProfileName]
		_, inMDM := mdmByName[ap.ProfileName]
		shouldRemove := !inMDM && (cfg.ForceCleanup || isManaged)
		if shouldRemove {
			_ = os.Remove(ap.OvpnConfigFilePath)
		} else {
			kept = append(kept, ap)
		}
	}
	existing.ConnectionProfiles = kept

	// Build index of already-registered profiles for upsert.
	registeredByName := make(map[string]int, len(existing.ConnectionProfiles))
	for i, ap := range existing.ConnectionProfiles {
		registeredByName[ap.ProfileName] = i
	}

	// Upsert all profiles from MDM.
	for _, p := range cfg.VpnProfiles {
		if !ovpn.ValidateName(p.ProfileName) {
			fmt.Fprintf(os.Stderr, "avc-sync: skipping profile with invalid name %q (only a-z, A-Z, 0-9, spaces, ()_- are allowed)\n", p.ProfileName)
			continue
		}

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

	if err := s.saveProfiles(existing); err != nil {
		return err
	}

	// Persist the new managed set — exactly the profiles in the current MDM payload.
	newState := models.SyncState{ManagedProfiles: make([]string, 0, len(cfg.VpnProfiles))}
	for _, p := range cfg.VpnProfiles {
		newState.ManagedProfiles = append(newState.ManagedProfiles, p.ProfileName)
	}
	return s.saveState(&newState)
}

// loadState reads the state file, returning an empty state if it doesn't exist.
func (s *Syncer) loadState() (*models.SyncState, error) {
	data, err := os.ReadFile(s.statePath)
	if os.IsNotExist(err) {
		return &models.SyncState{ManagedProfiles: []string{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var state models.SyncState
	if err := json.Unmarshal(data, &state); err != nil {
		return &models.SyncState{ManagedProfiles: []string{}}, nil
	}
	return &state, nil
}

// saveState atomically writes the state file.
func (s *Syncer) saveState(state *models.SyncState) error {
	return atomicWriteJSON(s.awsDir, s.statePath, state)
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
		return &models.AWSConnectionProfiles{
			Version:                  profilesVersion,
			LastSelectedProfileIndex: -1,
			ConnectionProfiles:       []models.AWSProfile{},
		}, nil
	}
	return &root, nil
}

// saveProfiles atomically writes ConnectionProfiles.
func (s *Syncer) saveProfiles(root *models.AWSConnectionProfiles) error {
	return atomicWriteJSON(s.awsDir, s.profilesPath, root)
}

// atomicWriteJSON marshals v to JSON and writes it to dest via a temp file in
// dir, using os.Rename for an atomic swap.
func atomicWriteJSON(dir, dest string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshaling %s: %w", dest, err)
	}

	tmp, err := os.CreateTemp(dir, ".avc-sync-tmp-*")
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

	if err := os.Rename(tmpPath, dest); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("atomic rename to %s: %w", dest, err)
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
