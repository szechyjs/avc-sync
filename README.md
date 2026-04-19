# avc-sync

A macOS helper tool that automatically syncs AWS VPN Client profiles pushed via MDM (NinjaOne, Jamf, Kandji, or any MDM supporting Custom Settings payloads).

## How It Works

1. Your MDM pushes a Custom Settings plist to `/Library/Managed Preferences/`
2. A LaunchAgent watches that path and triggers the `avc-sync` binary
3. The binary reads the managed preferences via CoreFoundation (CGO), writes ovpn config files, and updates the AWS VPN Client's `ConnectionProfiles` registry
4. The user opens the AWS VPN Client and their profiles are already present

The LaunchAgent is installed to `/Library/LaunchAgents/` and loads automatically for every user at login — including accounts created after the PKG is installed.

## MDM Payload

Push a **Custom Settings** payload with domain `io.k8jss.avc-sync`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>VpnProfiles</key>
    <array>
        <dict>
            <key>ProfileName</key>
            <string>Production-VPC</string>
            <key>OvpnContent</key>
            <string>client
dev tun
proto udp
remote cvpn-endpoint-xxx.prod.clientvpn.us-east-1.amazonaws.com 443
remote-random-hostname
resolv-retry infinite
nobind
remote-cert-tls server
cipher AES-256-GCM
verb 3
auth-federate
</string>
        </dict>
    </array>
</dict>
</plist>
```

See `examples/mdm-payload.plist` for a complete example.

## AWS VPN Client Paths

| Resource | Path |
|---|---|
| Ovpn config files | `~/.config/AWSVPNClient/OpenVpnConfigs/<ProfileName>` |
| Profile registry | `~/.config/AWSVPNClient/ConnectionProfiles` |

`avc-sync` treats the MDM payload as the source of truth — profiles present in MDM are created or updated, and profiles removed from MDM are deleted from both the registry and disk. Profiles the user added manually are never removed (see `ForceCleanup` below).

## ForceCleanup

By default, `avc-sync` only removes profiles it previously created (tracked via a sidecar state file). Profiles the user added manually are left untouched.

Set `ForceCleanup` to `true` in the MDM payload to remove **all** profiles not present in `VpnProfiles`, including user-added ones. This is intended for one-time migration scenarios — e.g., replacing ad-hoc profiles with a standardised managed set at MDM rollout.

```xml
<key>ForceCleanup</key>
<true/>
```

Remove or set back to `false` after the cleanup to restore normal behaviour.

## Installed Paths

| Resource | Path |
|---|---|
| Binary | `/usr/local/bin/avc-sync` |
| LaunchAgent | `/Library/LaunchAgents/io.k8jss.avc-sync.plist` |
| Logs | `/Library/Logs/avc-sync.log` |

## Build Prerequisites

- macOS with Xcode Command Line Tools (`xcode-select --install`)
- Go 1.21+
- Apple Developer account with:
  - **Developer ID Application** certificate (for signing the binary)
  - **Developer ID Installer** certificate (for signing the `.pkg`)
  - App-specific password for notarization

## Building

```sh
# Local build for the current architecture (unsigned, for testing)
make build

# Full signed + notarized release pkg
DEVELOPER_ID_APP="Developer ID Application: Your Name (TEAMID)" \
DEVELOPER_ID_INSTALLER="Developer ID Installer: Your Name (TEAMID)" \
NOTARIZE_APPLE_ID="your@email.com" \
NOTARIZE_PASSWORD="xxxx-xxxx-xxxx-xxxx" \
NOTARIZE_TEAM_ID="TEAMID" \
make release
```

The `release` target builds a universal binary (arm64 + amd64), signs it with Hardened Runtime, creates a signed `.pkg`, and notarizes + staples it with Apple.

## Running Tests

```sh
make test
```

## Deployment (NinjaOne)

1. Upload `avc-sync-<version>.pkg` to the NinjaOne Software Repository and deploy it to your target devices
2. Create a **Custom Settings** policy with domain `io.k8jss.avc-sync` containing your `VpnProfiles` array
3. Assign the policy to your devices — profiles appear in the AWS VPN Client at next login (or immediately if the user is already logged in when the PKG is installed)

## Testing Locally

```sh
# Build the binary
make build

# Simulate an MDM push
defaults write io.k8jss.avc-sync VpnProfiles -array \
  '{ ProfileName = "TestVPN"; OvpnContent = "client\ndev tun\n"; }'

# Run directly
./avc-sync
```
