// Command unipac is UniPac's native desktop app.
package main

import (
	"context"
	"log"

	"github.com/RUBisco0211/unipac/source/views"
	"github.com/egoist/mygo"
)

func main() {
	service := NewApp()

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
