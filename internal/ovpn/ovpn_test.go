package ovpn_test

import (
	"testing"

	"github.com/szechyjs/avc-sync/internal/ovpn"
)

// TestParse_AWSEndpointWithSAML tests a real AWS Client VPN ovpn file that
// includes auth-federate (SAML). Both endpoint fields and SAML fields should
// be populated.
func TestParse_AWSEndpointWithSAML(t *testing.T) {
	content := `client
dev tun
proto udp
remote cvpn-endpoint-0a2d15dc11c26eea0.prod.clientvpn.us-east-1.amazonaws.com 443
remote-random-hostname
resolv-retry infinite
nobind
remote-cert-tls server
cipher AES-256-GCM
verb 3
auth-federate
`
	cfg := ovpn.Parse(content)

	if cfg.CvpnEndpointId != "cvpn-endpoint-0a2d15dc11c26eea0" {
		t.Errorf("CvpnEndpointId: got %q, want %q", cfg.CvpnEndpointId, "cvpn-endpoint-0a2d15dc11c26eea0")
	}
	if cfg.CvpnEndpointRegion != "us-east-1" {
		t.Errorf("CvpnEndpointRegion: got %q, want %q", cfg.CvpnEndpointRegion, "us-east-1")
	}
	if cfg.CompatibilityVersion != "2" {
		t.Errorf("CompatibilityVersion: got %q, want \"2\"", cfg.CompatibilityVersion)
	}
	if cfg.FederatedAuthType != 1 {
		t.Errorf("FederatedAuthType: got %d, want 1", cfg.FederatedAuthType)
	}
}

// TestParse_AWSEndpointWithoutSAML tests an AWS endpoint ovpn that does NOT
// have auth-federate. Endpoint fields should be populated but auth type is 0.
func TestParse_AWSEndpointWithoutSAML(t *testing.T) {
	content := `client
dev tun
proto udp
remote cvpn-endpoint-0a2d15dc11c26eea0.prod.clientvpn.us-east-1.amazonaws.com 443
remote-cert-tls server
auth-user-pass
`
	cfg := ovpn.Parse(content)

	if cfg.CvpnEndpointId != "cvpn-endpoint-0a2d15dc11c26eea0" {
		t.Errorf("CvpnEndpointId: got %q", cfg.CvpnEndpointId)
	}
	if cfg.CvpnEndpointRegion != "us-east-1" {
		t.Errorf("CvpnEndpointRegion: got %q", cfg.CvpnEndpointRegion)
	}
	// No auth-federate → CompatibilityVersion stays "1", FederatedAuthType stays 0.
	if cfg.CompatibilityVersion != "1" {
		t.Errorf("CompatibilityVersion: got %q, want \"1\"", cfg.CompatibilityVersion)
	}
	if cfg.FederatedAuthType != 0 {
		t.Errorf("FederatedAuthType: got %d, want 0", cfg.FederatedAuthType)
	}
}

// TestParse_GenericEndpoint tests a non-AWS ovpn config.
func TestParse_GenericEndpoint(t *testing.T) {
	content := `client
dev tun
proto udp
remote vpn.prod.example.com 443
auth-user-pass
`
	cfg := ovpn.Parse(content)

	if cfg.CvpnEndpointId != "" {
		t.Errorf("CvpnEndpointId: got %q, want empty", cfg.CvpnEndpointId)
	}
	if cfg.CvpnEndpointRegion != "" {
		t.Errorf("CvpnEndpointRegion: got %q, want empty", cfg.CvpnEndpointRegion)
	}
	if cfg.CompatibilityVersion != "1" {
		t.Errorf("CompatibilityVersion: got %q, want \"1\"", cfg.CompatibilityVersion)
	}
	if cfg.FederatedAuthType != 0 {
		t.Errorf("FederatedAuthType: got %d, want 0", cfg.FederatedAuthType)
	}
}

// TestParse_AuthFederateWithLeadingWhitespace ensures the directive is matched
// even with leading/trailing whitespace, mirroring the AWS VPN Client regex.
func TestParse_AuthFederateWithLeadingWhitespace(t *testing.T) {
	content := "client\n  auth-federate  \ndev tun\n"
	cfg := ovpn.Parse(content)
	if cfg.FederatedAuthType != 1 {
		t.Errorf("FederatedAuthType: got %d, want 1 (auth-federate with whitespace)", cfg.FederatedAuthType)
	}
	if cfg.CompatibilityVersion != "2" {
		t.Errorf("CompatibilityVersion: got %q, want \"2\"", cfg.CompatibilityVersion)
	}
}

// TestParse_NonStandaloneAuthFederate ensures a line like `auth-federate-extra`
// does not trigger SAML detection.
func TestParse_NonStandaloneAuthFederate(t *testing.T) {
	content := "client\nauth-federate-extra\ndev tun\n"
	cfg := ovpn.Parse(content)
	if cfg.FederatedAuthType != 0 {
		t.Errorf("FederatedAuthType: got %d, want 0 (not a standalone auth-federate)", cfg.FederatedAuthType)
	}
}

// TestParse_EndpointEnvSegmentVariants ensures the env segment (e.g. "prod",
// "stage") is accepted as any alpha string per the AWS VPN Client regex.
func TestParse_EndpointEnvSegmentVariants(t *testing.T) {
	content := `client
remote cvpn-endpoint-abc123def456.stage.clientvpn.eu-west-1.amazonaws.com 1194
`
	cfg := ovpn.Parse(content)

	if cfg.CvpnEndpointId != "cvpn-endpoint-abc123def456" {
		t.Errorf("CvpnEndpointId: got %q", cfg.CvpnEndpointId)
	}
	if cfg.CvpnEndpointRegion != "eu-west-1" {
		t.Errorf("CvpnEndpointRegion: got %q", cfg.CvpnEndpointRegion)
	}
}

// TestParse_NoRemoteDirective tests a minimal config with no remote line.
func TestParse_NoRemoteDirective(t *testing.T) {
	cfg := ovpn.Parse("client\ndev tun\n")

	if cfg.CvpnEndpointId != "" || cfg.CvpnEndpointRegion != "" {
		t.Errorf("expected empty endpoint fields, got id=%q region=%q",
			cfg.CvpnEndpointId, cfg.CvpnEndpointRegion)
	}
	if cfg.CompatibilityVersion != "1" || cfg.FederatedAuthType != 0 {
		t.Errorf("expected default fields for minimal config")
	}
}
