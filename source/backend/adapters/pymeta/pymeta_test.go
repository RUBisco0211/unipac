package pymeta

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeVenv lays out a virtual environment the way pipx and uv tool do: the
// distribution's METADATA under lib/python<version>/site-packages/<name>-<ver>.dist-info.
func writeVenv(t *testing.T, project, version, metadata string) string {
	t.Helper()
	venv := t.TempDir()
	dist := filepath.Join(venv, "lib", "python3.13", "site-packages", normalize(project)+"-"+version+".dist-info")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "METADATA"), []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}
	return venv
}

// A real METADATA file becomes the page's fields, with the project page found
// among the addresses it declares.
func TestFromVenv(t *testing.T) {
	metadata := "Metadata-Version: 2.1\n" +
		"Name: agentseek\n" +
		"Version: 0.1.6\n" +
		"Summary: A database-native Agent Harness for runtime data workflows\n" +
		"Author-Email: Chojan Shang <psiace@apache.org>\n" +
		"License-Expression: MIT\n" +
		"Project-URL: Repository, https://github.com/ob-labs/agentseek\n" +
		"Project-URL: Documentation, https://docs.example/agentseek\n" +
		"Requires-Python: <3.14,>=3.12\n" +
		"\n" +
		"The long description starts here and is not a header.\n"
	venv := writeVenv(t, "agentseek", "0.1.6", metadata)

	detail, err := FromVenv(venv, "agentseek")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if detail.Name != "agentseek" || detail.Version != "0.1.6" {
		t.Errorf("name/version = %q %q", detail.Name, detail.Version)
	}
	if !strings.Contains(detail.Summary, "Agent Harness") {
		t.Errorf("summary = %q", detail.Summary)
	}
	if detail.Publisher != "Chojan Shang" {
		t.Errorf("publisher = %q", detail.Publisher)
	}
	if detail.License != "MIT" {
		t.Errorf("license = %q", detail.License)
	}
	if detail.Homepage != "https://github.com/ob-labs/agentseek" {
		t.Errorf("homepage = %q", detail.Homepage)
	}
}

// The directory name is normalized even when the project's own name is not.
func TestFromVenvFindsANormalizedName(t *testing.T) {
	metadata := "Name: Some_Tool.Name\nVersion: 2.0.0\nSummary: a tool\n"
	venv := writeVenv(t, "Some_Tool.Name", "2.0.0", metadata)

	if _, err := FromVenv(venv, "some-tool-name"); err != nil {
		t.Fatalf("read: %v", err)
	}
	if _, err := FromVenv(venv, "Some_Tool.Name"); err != nil {
		t.Fatalf("read by the declared name: %v", err)
	}
}

// A project that is not in the environment is reported as missing, not as an
// empty answer.
func TestFromVenvWithoutTheProject(t *testing.T) {
	venv := writeVenv(t, "cowsay", "6.1", "Name: cowsay\nVersion: 6.1\n")
	if _, err := FromVenv(venv, "not-there"); err == nil {
		t.Fatal("a missing project read as an answer")
	}
}

// An environment with no site-packages at all is missing too, rather than
// panicking on a glob with no matches.
func TestFromVenvWithoutSitePackages(t *testing.T) {
	if _, err := FromVenv(t.TempDir(), "cowsay"); err == nil {
		t.Fatal("an empty directory read as an answer")
	}
}
