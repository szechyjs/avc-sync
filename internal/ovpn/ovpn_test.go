package ovpn_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/szechyjs/avc-sync/internal/ovpn"
)

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

	assert.Equal(t, "cvpn-endpoint-0a2d15dc11c26eea0", cfg.CvpnEndpointId)
	assert.Equal(t, "us-east-1", cfg.CvpnEndpointRegion)
	assert.Equal(t, "2", cfg.CompatibilityVersion)
	assert.Equal(t, 1, cfg.FederatedAuthType)
}

func TestParse_AWSEndpointWithoutSAML(t *testing.T) {
	content := `client
dev tun
proto udp
remote cvpn-endpoint-0a2d15dc11c26eea0.prod.clientvpn.us-east-1.amazonaws.com 443
remote-cert-tls server
auth-user-pass
`
	cfg := ovpn.Parse(content)

	assert.Equal(t, "cvpn-endpoint-0a2d15dc11c26eea0", cfg.CvpnEndpointId)
	assert.Equal(t, "us-east-1", cfg.CvpnEndpointRegion)
	assert.Equal(t, "1", cfg.CompatibilityVersion)
	assert.Equal(t, 0, cfg.FederatedAuthType)
}

func TestParse_GenericEndpoint(t *testing.T) {
	content := `client
dev tun
proto udp
remote vpn.prod.example.com 443
auth-user-pass
`
	cfg := ovpn.Parse(content)

	assert.Empty(t, cfg.CvpnEndpointId)
	assert.Empty(t, cfg.CvpnEndpointRegion)
	assert.Equal(t, "1", cfg.CompatibilityVersion)
	assert.Equal(t, 0, cfg.FederatedAuthType)
}

func TestParse_AuthFederateWithLeadingWhitespace(t *testing.T) {
	cfg := ovpn.Parse("client\n  auth-federate  \ndev tun\n")

	assert.Equal(t, 1, cfg.FederatedAuthType)
	assert.Equal(t, "2", cfg.CompatibilityVersion)
}

func TestParse_NonStandaloneAuthFederate(t *testing.T) {
	cfg := ovpn.Parse("client\nauth-federate-extra\ndev tun\n")

	assert.Equal(t, 0, cfg.FederatedAuthType)
}

func TestParse_EndpointEnvSegmentVariants(t *testing.T) {
	content := `client
remote cvpn-endpoint-abc123def456.stage.clientvpn.eu-west-1.amazonaws.com 1194
`
	cfg := ovpn.Parse(content)

	assert.Equal(t, "cvpn-endpoint-abc123def456", cfg.CvpnEndpointId)
	assert.Equal(t, "eu-west-1", cfg.CvpnEndpointRegion)
}

func TestParse_NoRemoteDirective(t *testing.T) {
	cfg := ovpn.Parse("client\ndev tun\n")

	assert.Empty(t, cfg.CvpnEndpointId)
	assert.Empty(t, cfg.CvpnEndpointRegion)
	assert.Equal(t, "1", cfg.CompatibilityVersion)
	assert.Equal(t, 0, cfg.FederatedAuthType)
}

func TestValidateName(t *testing.T) {
	valid := []string{
		"Production-VPC",
		"Staging Environment",
		"My VPN (US)",
		"vpn_1",
		"ABC123",
		"",
	}
	for _, name := range valid {
		assert.Truef(t, ovpn.ValidateName(name), "ValidateName(%q) should be true", name)
	}

	invalid := []string{
		"vpn/slash",
		"vpn:colon",
		"vpn@at",
		"vpn#hash",
		"vpn!bang",
		"vpn.dot",
		"vpn+plus",
	}
	for _, name := range invalid {
		assert.Falsef(t, ovpn.ValidateName(name), "ValidateName(%q) should be false", name)
	}
}
