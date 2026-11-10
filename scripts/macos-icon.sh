#!/bin/sh
# Regenerate the macOS icon from assets/icon.png.
#
# Two artifacts come out of this, and both are committed:
#
#   assets/AppIcon.icon/Assets/package.png  the drawing without its background,
#                                           which the Icon Composer document
#                                           composites over its own fill
#   assets/Assets.car                       the asset catalog actool compiles
#                                           from that document: the light, dark
#                                           and tinted icon of macOS 26
#
# mygo.json points macos.icon at the .car, so every build ships the dark icon
# without needing Xcode, and mygo renders the .icns that older macOS reads from
# assets/icon.png, at all eight sizes. Compiling the .icon at build time would
# need Xcode 26 on the machine that builds, and the .icns actool writes has only
# four sizes.
#
# Run it from anywhere; it works in the repository root. It needs macOS with
# Xcode 26 or later for actool, and a Mac at all for the .icon's own renderer.
set -eu
cd "$(dirname "$0")/.."

go run scripts/icongen/main.go

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/out"
cp -R assets/AppIcon.icon "$tmp/AppIcon.icon"
xcrun actool "$tmp/AppIcon.icon" \
	--compile "$tmp/out" \
	--platform macosx \
	--target-device mac \
	--minimum-deployment-target 12.0 \
	--app-icon AppIcon \
	--output-partial-info-plist "$tmp/out/partial.plist" >/dev/null
cp "$tmp/out/Assets.car" assets/Assets.car
echo "assets/Assets.car: $(wc -c <assets/Assets.car) bytes"
