// Command unipac is UniPac's native desktop app.
package main

import (
	"context"
	"log"

	"github.com/RUBisco0211/unipac/source/views"
	"github.com/egoist/mygo"
)

func main() {
	if demoMode() {
		log.Print("UniPac demo mode: simulated packages and tasks, no package managers are run")
		mygo.App.WhenReady(func() {
			views.New(newDemoService()).Run()
		})
	} else {
		service := NewApp()
		mygo.App.WhenReady(func() {
			if err := service.Startup(context.Background()); err != nil {
				mygo.Dialog.Error("UniPac failed to start", err.Error())
				mygo.App.Quit()
				return
			}
			views.New(service).Run()
		})
	}

	mygo.App.OnWindowAllClosed(func() { mygo.App.Quit() })

	err := mygo.App.Run()
	Cleanup()
	if err != nil {
		log.Fatal(err)
	}
}
