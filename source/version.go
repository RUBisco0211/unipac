package main

// appVersion is the version the About section shows. mygo.json is the manifest
// the bundle is built from and the place the version is written down; this copy
// exists because a bundled binary cannot read that file at runtime, and
// TestAppVersionMatchesTheManifest keeps the two equal.
const appVersion = "0.2.0"
