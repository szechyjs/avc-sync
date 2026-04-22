package models

// VpnProfile represents a single VPN profile as configured in the MDM payload.
type VpnProfile struct {
	ProfileName string `plist:"ProfileName"`
	OvpnContent string `plist:"OvpnContent"`
}

// MDMConfig is the top-level structure of the managed preferences payload.
type MDMConfig struct {
	VpnProfiles []VpnProfile `plist:"VpnProfiles"`
	// ForceCleanup removes all profiles not present in VpnProfiles, including
	// ones the user added manually. Intended for one-time migration scenarios
	// where administrators want to enforce a clean, fully-managed profile set.
	ForceCleanup bool `plist:"ForceCleanup"`
}

// AWSProfile is a single entry in the AWS VPN Client's ConnectionProfiles file.
type AWSProfile struct {
	ProfileName          string `json:"ProfileName"`
	OvpnConfigFilePath   string `json:"OvpnConfigFilePath"`
	CvpnEndpointId       string `json:"CvpnEndpointId"`
	CvpnEndpointRegion   string `json:"CvpnEndpointRegion"`
	CompatibilityVersion string `json:"CompatibilityVersion"`
	FederatedAuthType    int    `json:"FederatedAuthType"`
}

// SyncState tracks the profile names currently managed by avc-sync.
// It is persisted alongside ConnectionProfiles so that user-added profiles
// are never removed by subsequent sync runs.
type SyncState struct {
	ManagedProfiles []string `json:"ManagedProfiles"`
}

// AWSConnectionProfiles is the root structure of ~/.config/AWSVPNClient/ConnectionProfiles.
type AWSConnectionProfiles struct {
	Version                  string       `json:"Version"`
	LastSelectedProfileIndex int          `json:"LastSelectedProfileIndex"`
	ConnectionProfiles       []AWSProfile `json:"ConnectionProfiles"`
}
