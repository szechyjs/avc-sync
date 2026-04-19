// Package ovpn provides utilities for parsing OpenVPN config file content.
package ovpn

import (
	"regexp"
	"strings"
)

// awsCvpnPattern mirrors the regex used by the AWS VPN Client to extract the
// endpoint ID and region from anywhere in the ovpn file content:
//
//	(cvpn-endpoint-[A-Za-z0-9]+)\.[A-Za-z]+\.clientvpn\.(.+)\.amazonaws\.com
var awsCvpnPattern = regexp.MustCompile(
	`(cvpn-endpoint-[A-Za-z0-9]+)\.[A-Za-z]+\.clientvpn\.(.+)\.amazonaws\.com`,
)

// authFederatePattern matches a standalone `auth-federate` directive,
// optionally surrounded by whitespace — matching the AWS VPN Client logic:
//
//	^\s*auth-federate(\s*)$
var authFederatePattern = regexp.MustCompile(`(?m)^\s*auth-federate\s*$`)

// ParsedConfig holds the fields extracted from an ovpn file that the
// AWS VPN Client stores in ConnectionProfiles.
type ParsedConfig struct {
	CvpnEndpointId       string
	CvpnEndpointRegion   string
	CompatibilityVersion string
	FederatedAuthType    int
}

// Parse extracts AWS VPN Client metadata from raw ovpn file content.
//
// FederatedAuthType is set to 1 (SAML) when the `auth-federate` directive
// is present as a standalone line — matching the AWS VPN Client's own logic.
// CompatibilityVersion is "2" for SAML configs, "1" otherwise.
// CvpnEndpointId and CvpnEndpointRegion are extracted via the same regex the
// AWS VPN Client uses, scanning the full file content.
func Parse(content string) ParsedConfig {
	cfg := ParsedConfig{
		CompatibilityVersion: "1",
		FederatedAuthType:    0,
	}

	// Detect auth-federate directive (determines SAML / CompatibilityVersion).
	if authFederatePattern.MatchString(content) {
		cfg.FederatedAuthType = 1
		cfg.CompatibilityVersion = "2"
	}

	// Extract endpoint ID and region from anywhere in the file.
	if m := awsCvpnPattern.FindStringSubmatch(content); m != nil {
		cfg.CvpnEndpointId = strings.TrimSpace(m[1])
		cfg.CvpnEndpointRegion = strings.TrimSpace(m[2])
	}

	return cfg
}
