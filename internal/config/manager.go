package config

import (
	"fmt"
	"strings"
)

// ParsePackageManager accepts an explicit supported runner and optional exact
// numeric version. package.json declarations additionally require the version.
func ParsePackageManager(value string) (string, string, error) {
	name, version, hasVersion := strings.Cut(value, "@")
	switch name {
	case "npm", "pnpm", "yarn", "bun":
	default:
		return "", "", fmt.Errorf("expected npm, pnpm, yarn or bun, optionally @major.minor.patch")
	}
	if !hasVersion {
		return name, "", nil
	}
	version, _, _ = strings.Cut(version, "+")
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return "", "", fmt.Errorf("version must be an exact major.minor.patch")
	}
	for _, p := range parts {
		if p == "" || len(p) > 9 || len(p) > 1 && p[0] == '0' {
			return "", "", fmt.Errorf("version must be an exact major.minor.patch")
		}
		for _, ch := range p {
			if ch < '0' || ch > '9' {
				return "", "", fmt.Errorf("version must be an exact major.minor.patch")
			}
		}
	}
	return name, version, nil
}
