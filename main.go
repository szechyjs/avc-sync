package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/szechyjs/avc-sync/internal/cfprefs"
	"github.com/szechyjs/avc-sync/internal/models"
	"github.com/szechyjs/avc-sync/internal/sync"
	"howett.net/plist"
)

const (
	// preferenceDomain is the MDM Custom Settings payload domain.
	preferenceDomain = "io.k8jss.avc-sync"

	// preferenceKey is the top-level key inside the payload that holds
	// the array of VPN profiles.
	preferenceKey = "VpnProfiles"

	// forceCleanupKey is the top-level boolean key that triggers removal of
	// all profiles not present in VpnProfiles, including user-added ones.
	forceCleanupKey = "ForceCleanup"
)

// version is set at build time via -ldflags "-X main.version=<value>".
var (
	version = "dev"
	commit  = "none" //nolint:unused
	date    = "unknown"
)

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("avc-sync %s, built at %s\n", version, date)
		os.Exit(0)
	}

	// Small startup delay to ensure the preference file has been fully
	// written by cfprefsd before we attempt to read it.
	time.Sleep(2 * time.Second)

	cfg, err := readManagedConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "avc-sync: failed to read managed configuration: %v\n", err)
		os.Exit(1)
	}

	syncer, err := sync.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "avc-sync: failed to initialize syncer: %v\n", err)
		os.Exit(1)
	}

	if err := syncer.Sync(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "avc-sync: sync failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("avc-sync: synced %d profile(s) successfully\n", len(cfg.VpnProfiles))
}

func readManagedConfig() (*models.MDMConfig, error) {
	xmlBytes, err := cfprefs.CopyAppValue(preferenceDomain, preferenceKey)
	if err != nil {
		// Key absent means the MDM payload was removed. Return an empty config
		// so the sync pass can clean up any previously managed profiles.
		fmt.Fprintf(os.Stderr, "avc-sync: no managed configuration found, removing any previously managed profiles\n")
		return &models.MDMConfig{}, nil
	}

	// ForceCleanup is a top-level boolean key in the same domain. It must be
	// read separately because CopyAppValue only returns a single key's value.
	var forceCleanup bool
	if fcBytes, err := cfprefs.CopyAppValue(preferenceDomain, forceCleanupKey); err == nil {
		_, _ = plist.Unmarshal(fcBytes, &forceCleanup)
	}

	return parseMDMPayload(xmlBytes, forceCleanup)
}

// parseMDMPayload decodes the VpnProfiles plist value (as returned by
// cfprefs.CopyAppValue) together with the separately-fetched forceCleanup flag
// into an MDMConfig. It is a pure function to allow unit testing without CGO.
func parseMDMPayload(vpnProfilesXML []byte, forceCleanup bool) (*models.MDMConfig, error) {
	// CFPreferences returns the value for a single key (the array), so we
	// wrap it in a dict to match our MDMConfig struct layout.
	wrapped := map[string]interface{}{
		preferenceKey: nil,
	}

	var rawValue interface{}
	if _, err := plist.Unmarshal(vpnProfilesXML, &rawValue); err != nil {
		return nil, fmt.Errorf("parsing preference value: %w", err)
	}
	wrapped[preferenceKey] = rawValue

	wrappedBytes, err := plist.Marshal(wrapped, plist.XMLFormat)
	if err != nil {
		return nil, fmt.Errorf("re-wrapping preference value: %w", err)
	}

	var cfg models.MDMConfig
	if _, err := plist.Unmarshal(wrappedBytes, &cfg); err != nil {
		return nil, fmt.Errorf("decoding MDM config: %w", err)
	}

	cfg.ForceCleanup = forceCleanup
	return &cfg, nil
}
