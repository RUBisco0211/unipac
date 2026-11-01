# Hidden tests

`go test ./...` must never show a window, take focus, add a Dock icon or touch real user settings. Test the behavior the application owns and bugs it fixes; layout, copy and styling adjustments need no new tests. Extend existing tests before adding new ones, and keep them fast: no real-time waits or repeated full-window frames. Use three layers, from most to least frequent:

1. **Logic tests** for preferences, policies, language resolution and other decisions. Keep them as plain functions or methods on an application struct whose paths and dependencies tests can replace, so no MyGo runtime is needed.
2. **Headless UI tests** with `ui.NewTester`. These render builders off screen and drive input without a desktop session. This is the default for component and view behavior.
3. **Opt-in native smoke tests** behind environment variables. Windows are created hidden by default; only an explicit foreground switch makes them visible.

## Headless UI tests

Build the same view the application builds, apply the theme yourself (no Shell is present) and capture results in closure variables:

```go
func TestSaveButton(t *testing.T) {
	for _, dark := range []bool{false, true} {
		saved := false
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(dark))
			if components.Button(c, "Save", components.ButtonPrimary).Clicked() {
				saved = true
			}
		}, 300, 100)
		if err := tt.Click("Save"); err != nil {
			t.Fatal(err)
		}
		if !saved {
			t.Errorf("dark=%v: click not reported", dark)
		}
	}
}
```

- Locate elements by visible text or accessible label with `Find`, `HasText`, `Texts` and `Focused`; drive them with `Click`, `RightClick`, `Key`, `TypeKey`, `Type`, `Compose`, `Scroll`, `Wheel`, `DropFiles`, `Move`, `Press` and `Release`. `Click` and `RightClick` return an error for an element outside the window, so size the tester to fit it.
- `Scroll` scrolls at once as a touchpad does; `Wheel` is a mouse wheel notch, which scroll containers glide through as the clock advances.
- Check keyboard paths, not just pointer clicks: `tt.Key(0, ui.KeyTab)` then `tt.Key(0, ui.KeySpace)` or `ui.KeyEnter`.
- Inspect menus with `Menu` and `ChooseMenuItem`, screen-reader output with `Announcements`, pointer shape with `Cursor`, and clipboard or opened URLs with their accessors instead of system services.
- The tester's clock stands still until `tt.Advance(d)` moves it, running the frames due meanwhile; advance it for animations, `After` delays, tooltips and separate clicks instead of sleeping.
- After changing state outside the builder, call `tt.Frame()` before asserting. Use `SetSize`, `SetScale`, `SetDark` and `SetPreferences` for narrow layouts, scaling, appearance and reduced motion.
- Assert appearance with `tt.Image().RGBAAt(x, y)` relative to another rendering, such as enabled versus disabled, rather than hard-coded colors.
- Cover both themes when a change affects theme-dependent colors. Read the version-matched `Tester` methods in MyGo's `ui/` before relying on one.

## Native smoke tests

Use these only for behavior a tester cannot reach: window lifecycle, tray, Dock, native menus, input sources or real captures. Gate the entire MyGo runtime in `TestMain`, so ordinary runs never start it. Use the project's own variable prefix; Onevra uses `ONEVRA_NATIVE_SMOKE` and `ONEVRA_NATIVE_FOREGROUND`.

```go
func foreground() bool { return os.Getenv("APP_NATIVE_FOREGROUND") == "1" }

func TestMain(m *testing.M) {
	if os.Getenv("APP_NATIVE_SMOKE") != "1" {
		os.Exit(m.Run()) // Ordinary tests never open windows.
	}
	dir, err := os.MkdirTemp("", "app-smoke-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	mygo.App.SetName("App smoke")
	if !foreground() {
		mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory) // No Dock icon or activation.
	}
	mygo.App.SetPath(mygo.PathUserData, dir) // Never touch real preferences.
	mygo.App.OnWindowAllClosed(func() {})    // Only the end of m.Run quits.
	result := make(chan int, 1)
	mygo.App.WhenReady(func() { go func() { result <- m.Run(); mygo.App.Quit() }() })
	if err := mygo.App.Run(); err != nil {
		os.RemoveAll(dir)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := <-result
	os.RemoveAll(dir)
	os.Exit(code)
}

func TestNativeWindow(t *testing.T) {
	if os.Getenv("APP_NATIVE_SMOKE") != "1" {
		t.Skip("requires APP_NATIVE_SMOKE=1 in a desktop session")
	}
	var shell *plain.Shell
	mygo.RunOnMain(func() { shell = plain.NewShell(plain.ShellOptions{Hidden: !foreground(), Content: view}) })
	defer shell.Window().Destroy()
	// Every other window and UI call goes through mygo.RunOnMain.
}
```

- Create every Shell and window with `Hidden: !foreground()`. Tests needing real focus, tray or Dock interaction must also skip unless the foreground switch is set.
- Construct the application with temporary paths and in-memory state, not its production `main` setup; destroy windows and release trays or menus in `defer`.
- On macOS, AppKit stops display ticks for hidden windows. Tests that need fresh frames or captures must pump display themselves; copy `startNativeFrames` from `examples/gallery/internal/gallery/background_darwin_test.go` in the Onevra module.
- Write captures to a directory from an environment variable, defaulting to `t.TempDir()`.
- Run with `APP_NATIVE_SMOKE=1 GOWORK=off go test ./... -run '^TestNative' -count=1 -timeout 2m`. Lifecycle callbacks and captures do not prove real pointer, keyboard, input method or accessibility behavior; report those as needing manual review.
