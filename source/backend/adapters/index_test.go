package adapters_test

import (
	"context"
	"strings"
	"testing"

	"github.com/RUBisco0211/unipac/source/backend/adapters"
)

// Every adapter tells the settings page where its own documentation is: the
// manager's category shows the address and the way to open it, so a missing one
// leaves a hole there rather than failing at runtime.
func TestEveryAdapterDeclaresItsWebsite(t *testing.T) {
	for _, constructor := range adapters.GetAdapterConstructors() {
		info := constructor(context.Background()).Info()
		if !strings.HasPrefix(info.Website, "https://") {
			t.Errorf("%s declares website %q, want an https address", info.ID, info.Website)
		}
	}
}
