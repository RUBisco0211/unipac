package adapters

import (
	"unipac-wails/backend/adapters/brew"
	"unipac-wails/backend/adapters/cargo"
	"unipac-wails/backend/adapters/npm"
	"unipac-wails/backend/adapters/pip"
	"unipac-wails/backend/adapters/pipx"
	"unipac-wails/backend/core/manager"
)

func GetAdapterConstructors() []manager.AdapterConstructor {
	return []manager.AdapterConstructor{
		brew.NewAdapter,
		npm.NewAdapter,
		pip.NewAdapter,
		pipx.NewAdapter,
		cargo.NewAdapter,
	}
}
