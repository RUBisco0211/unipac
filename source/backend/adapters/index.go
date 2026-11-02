package adapters

import (
	"github.com/RUBisco0211/unipac/source/backend/adapters/brew"
	"github.com/RUBisco0211/unipac/source/backend/adapters/cargo"
	"github.com/RUBisco0211/unipac/source/backend/adapters/npm"
	"github.com/RUBisco0211/unipac/source/backend/adapters/pip"
	"github.com/RUBisco0211/unipac/source/backend/adapters/pipx"
	"github.com/RUBisco0211/unipac/source/backend/adapters/uvtool"
	"github.com/RUBisco0211/unipac/source/backend/core/manager"
)

func GetAdapterConstructors() []manager.AdapterConstructor {
	return []manager.AdapterConstructor{
		brew.NewAdapter,
		npm.NewAdapter,
		pip.NewAdapter,
		pipx.NewAdapter,
		cargo.NewAdapter,
		uvtool.NewAdapter,
	}
}
