// Command unipac is UniPac's native desktop app.
package main

import (
	"context"
	"log"

	"github.com/RUBisco0211/unipac/source/views"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	updaternative "github.com/egoist/mygo/plugins/updater/native"
)

func main() {
	service := NewApp()
	// The update window is native UI, like the rest of the app, so the app needs
	// no webview to offer an update. It checks in the background ten seconds
	// after launch and then once a day; builds that cannot update themselves,
	// such as a development build, never check.
	mygo.Use(updaternative.Plugin)
	mygo.App.SetMenu(appMenu())

	mygo.App.WhenReady(func() {
		if err := service.Startup(context.Background()); err != nil {
			mygo.Dialog.Error("UniPac failed to start", err.Error())
			mygo.App.Quit()
			return
		}
		views.New(service).Run()
	})

	mygo.App.OnWindowAllClosed(func() { mygo.App.Quit() })

	err := mygo.App.Run()
	Cleanup()
	if err != nil {
		log.Fatal(err)
	}
}

// appMenu is the menu bar the app would otherwise get by default, with "Check
// for Updates…" after About. MyGo installs its default menu only when the app
// sets none, so the one item the app adds means spelling the menu out; the rest
// is the same roles, and they keep their shortcuts and their behavior.
func appMenu() *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu, Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleAbout},
			updater.MenuItem(),
			mygo.Separator(),
			{Role: mygo.RoleServices},
			mygo.Separator(),
			{Role: mygo.RoleHide},
			{Role: mygo.RoleHideOthers},
			{Role: mygo.RoleUnhide},
			mygo.Separator(),
			{Role: mygo.RoleQuit},
		}},
		{Role: mygo.RoleFileMenu},
		{Role: mygo.RoleEditMenu},
		{Role: mygo.RoleViewMenu},
		{Role: mygo.RoleWindowMenu},
	})
}
