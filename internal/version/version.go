package version

import "strings"

// Version is replaced with the release tag by the release build. Keeping a
// development fallback makes local builds and tests self-describing.
var Version = "dev"

// Current returns the canonical user-facing version. Release tooling may pass
// either 1.2.3 or v1.2.3, but the application stores and displays one form.
func Current() string { return Normalize(Version) }

// Normalize removes release-tag decoration and falls back to dev for empty
// values. The splash screen owns the visual "v" prefix.
func Normalize(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "v")
	if value == "" {
		return "dev"
	}
	return value
}
