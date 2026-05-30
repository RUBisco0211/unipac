package util

import (
	"context"

	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"
)

var (
	ButtonOk     = "Ok"
	ButtonCancel = "Cancel"
)

type DialogButtonEvent func(ctx context.Context) error

type DialogOptions struct {
	Type     wailsrt.DialogType
	Title    string
	Message  string
	OnOk     DialogButtonEvent
	OnCancel DialogButtonEvent
}

func WailsDialog(ctx context.Context, opt DialogOptions) error {
	selection, err := wailsrt.MessageDialog(ctx, wailsrt.MessageDialogOptions{
		Type:          wailsrt.ErrorDialog,
		Title:         opt.Title,
		Message:       opt.Message,
		DefaultButton: ButtonOk,
		CancelButton:  ButtonCancel,
	})
	if err != nil {
		return err
	}
	switch selection {
	case ButtonOk:
		if opt.OnOk == nil {
			return nil
		}
		return opt.OnOk(ctx)
	case ButtonCancel:
		if opt.OnCancel == nil {
			return nil
		}
		return opt.OnCancel(ctx)
	default:
	}
	return nil
}
