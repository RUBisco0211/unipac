package managers

import (
	"unipac-wails/backend/core/manager"
	"unipac-wails/backend/managers/brew"
)

func GetAdapterConstructors() []manager.AdapterConstructor {
	return []manager.AdapterConstructor{
		brew.NewAdapter,
	}
}
