package main

import (
	"embed"
	"log/slog"
	"runtime"
	"runtime/debug"
	"unipac-wails/backend/logging"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func Cleanup() {
	if logging.LogFile == nil {
		return
	}
	if err := logging.Close(); err != nil {
		slog.Error("Failed to close log file", "error", err)
	}
}

func main() {

	defer func() {
		if r := recover(); r != nil {
			slog.Error("recovered from panic",
				slog.String("stack", string(debug.Stack())),
				slog.String("error", r.(error).Error()),
			)
			Cleanup()
		}
	}()

	// Create an instance of the app structure
	app := NewApp()
	appMenu := menu.NewMenu()

	if runtime.GOOS == "darwin" {
		appMenu.Append(menu.AppMenu())
		subMenu := appMenu.AddSubmenu("App")
		subMenu.AddText("Settings...", keys.CmdOrCtrl(","), func(cd *menu.CallbackData) {
			wailsRuntime.EventsEmit(app.ctx, "open_settings")
		})
		appMenu.Append(menu.EditMenu())
		appMenu.Append(menu.WindowMenu())
	}

	// Create application with options
	err := wails.Run(&options.App{
		Title:                "UniPac",
		Width:                1024,
		Height:               768,
		MinWidth:             800,
		MinHeight:            600,
		CSSDragProperty:      "--wails-draggable",
		CSSDragValue:         "drag",
		DisablePanicRecovery: true,
		// Frameless: true,
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				FullSizeContent:            true,
				TitlebarAppearsTransparent: true,
				HideTitle:                  true,
				//UseToolbar:                 true,
			},
			Preferences: &mac.Preferences{
				TabFocusesLinks: mac.Enabled,
			},
			// WebviewIsTransparent: true,
		},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		// BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:          app.startup,
		OnShutdown:         app.shutdown,
		Menu:               appMenu,
		Logger:             logger.NewDefaultLogger(),
		LogLevelProduction: logger.WARNING,
		LogLevel:           logger.DEBUG,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		slog.Error("Failed to run wails app", "error", err.Error())
		panic(err)
	}
}
