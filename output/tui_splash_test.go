package output

import (
	"strings"
	"testing"

	"github.com/pranvgarg/toolsniff/config"
)

func TestRenderSplashShowsInjectedVersion(t *testing.T) {
	styles := NewThemeStyles(config.DefaultThemeSettings())
	got := renderSplash(nil, 120, 30, "1.2.3", styles)
	if !strings.Contains(got, "toolsniff  v1.2.3") {
		t.Fatalf("splash did not contain injected version: %q", got)
	}
}
