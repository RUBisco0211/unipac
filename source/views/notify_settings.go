// Where a system keeps the switch that allows the app to show notifications.
package views

import "runtime"

// notificationSettingsURL is the address that opens the system's page for the
// app's notifications: macOS's System Settings pane, Windows' Settings page, and
// an empty string where the system has no such page to open.
func notificationSettingsURL() string {
	switch runtime.GOOS {
	case "darwin":
		// System Settings > Notifications, the pane whose list carries the
		// app and its switch.
		return "x-apple.systempreferences:com.apple.Notifications-Settings.extension"
	case "windows":
		return "ms-settings:notifications"
	default:
		return ""
	}
}
