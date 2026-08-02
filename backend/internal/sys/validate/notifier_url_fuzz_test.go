package validate

import (
	"net"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/sysadminsmedia/homebox/backend/internal/sys/config"
)

func lowerScheme(raw string) string {
	scheme, rest, ok := strings.Cut(raw, "://")
	if !ok {
		return raw
	}

	return strings.ToLower(scheme) + "://" + rest
}

func shoutrrrTargetHost(raw string) (string, bool) {
	scheme, rest, ok := strings.Cut(raw, "://")
	if !ok {
		return "", false
	}

	scheme = strings.ToLower(scheme)

	var target string

	switch scheme {
	case "generic":
		target = rest
	case "generic+http", "generic+https":
		target = strings.TrimPrefix(scheme, "generic+") + "://" + rest
	default:
		return "", false
	}

	if !strings.Contains(target, "://") {
		target = "https://" + target
	}

	u, err := url.Parse(target)
	if err != nil {
		return "", false
	}

	host := u.Hostname()

	return host, host != ""
}

func shoutrrrTargetIP(raw string) net.IP {
	host, ok := shoutrrrTargetHost(raw)
	if !ok {
		return nil
	}

	return net.ParseIP(host)
}

func FuzzValidateNotifierURL(f *testing.F) {
	seeds := []string{
		validDiscordURL,
		"generic://https://93.184.216.34/webhook",
		"generic://93.184.216.34/webhook",
		"generic://http://localhost:8080/webhook",
		"generic://http://127.0.0.1:8080/webhook",
		"generic://http://10.0.0.1/webhook",
		"generic://http://172.16.0.1/webhook",
		"generic://http://169.254.169.254/latest/meta-data",
		"generic://http://169.254.1.1/webhook",
		"generic+https://127.0.0.1/webhook",
		"GENERIC+HTTP://127.0.0.1/webhook",
		"Generic+Https://169.254.169.254/latest/meta-data",
		urlGenericIPv4Local,
		urlGenericIPv6Local,
		"generic://http://[::1]:8080/webhook",
		"generic://http://[fe80::1]/webhook",
		"generic://http://[fd00:ec2::254]/webhook",
		"generic://http://[2001:db8::1]/webhook",
		"generic://",
		"generic://http://[",
		"generic://http://user:pass@127.0.0.1/x",
		"not-a-generic://whatever",
		"",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	cfg := &config.NotifierConf{
		BlockLocalhost:     true,
		BlockLocalNets:     true,
		BlockBogonNets:     true,
		BlockCloudMetadata: true,
		Dns64Nets:          dns64DefaultNets,
	}

	f.Fuzz(func(t *testing.T, raw string) {
		targetHost, parsedTarget := shoutrrrTargetHost(raw)
		if parsedTarget && net.ParseIP(targetHost) == nil {
			t.Skip("generic hostname targets require external resolution")
		}

		err := ValidateNotifierURL(raw, cfg)

		require.Equalf(t, ValidateNotifierURL(lowerScheme(raw), cfg) == nil, err == nil,
			"validator disagrees on %q versus its scheme-normalized form", raw)

		if err != nil {
			return
		}

		ip := shoutrrrTargetIP(raw)
		if ip == nil {
			return
		}

		require.Falsef(t, ip.IsLoopback(), "accepted %q targets loopback IP %s", raw, ip)
		require.Falsef(t, ip.IsPrivate(), "accepted %q targets private IP %s", raw, ip)
		require.Falsef(t, ip.IsLinkLocalUnicast(), "accepted %q targets link-local IP %s", raw, ip)
	})
}
