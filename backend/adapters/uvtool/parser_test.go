package uvtool

import "testing"

func TestParseInstalledToolsEmpty(t *testing.T) {
	records := parseInstalledTools("No tools installed")
	if len(records) != 0 {
		t.Fatalf("expected no records, got %d", len(records))
	}
}

func TestParseInstalledTools(t *testing.T) {
	output := `ruff v0.8.0 (ruff==0.8.0) (CPython 3.12.7)
  Path: /Users/example/.local/share/uv/tools/ruff
  Executables:
  - /Users/example/.local/bin/ruff
httpie v3.2.4 [required: httpie>=3]
  Python: CPython 3.11.9`

	records := parseInstalledTools(output)
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	if records[0].Name != "ruff" || records[0].Version != "0.8.0" {
		t.Fatalf("unexpected first record: %#v", records[0])
	}
	if records[0].VersionSpecifier != "ruff==0.8.0" {
		t.Fatalf("unexpected version specifier: %q", records[0].VersionSpecifier)
	}
	if records[0].Python != "CPython 3.12.7" {
		t.Fatalf("unexpected python: %q", records[0].Python)
	}
	if records[0].ToolPath != "/Users/example/.local/share/uv/tools/ruff" {
		t.Fatalf("unexpected tool path: %q", records[0].ToolPath)
	}
	if len(records[0].Executables) != 1 || records[0].Executables[0] != "/Users/example/.local/bin/ruff" {
		t.Fatalf("unexpected executables: %#v", records[0].Executables)
	}

	if records[1].Name != "httpie" || records[1].VersionSpecifier != "httpie>=3" {
		t.Fatalf("unexpected second record: %#v", records[1])
	}
}

func TestParseInstalledToolsRealUvOutputShape(t *testing.T) {
	output := `claude-tap v0.1.60 [CPython 3.12.12] (/Users/rubisco/.local/share/uv/tools/claude-tap)
- claude-tap (/Users/rubisco/.local/bin/claude-tap)
netron v9.0.8 [CPython 3.12.12] (/Users/rubisco/.local/share/uv/tools/netron)
- netron (/Users/rubisco/.local/bin/netron)
zotero-mcp-server v0.2.2 [CPython 3.12.12] (/Users/rubisco/.local/share/uv/tools/zotero-mcp-server)
- zotero-mcp (/Users/rubisco/.local/bin/zotero-mcp)`

	records := parseInstalledTools(output)
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d: %#v", len(records), records)
	}
	if records[0].Name != "claude-tap" || records[0].Version != "0.1.60" {
		t.Fatalf("unexpected first record: %#v", records[0])
	}
	if records[0].Python != "CPython 3.12.12" {
		t.Fatalf("unexpected python: %q", records[0].Python)
	}
	if records[0].ToolPath != "/Users/rubisco/.local/share/uv/tools/claude-tap" {
		t.Fatalf("unexpected tool path: %q", records[0].ToolPath)
	}
	if len(records[0].Executables) != 1 || records[0].Executables[0] != "claude-tap" {
		t.Fatalf("unexpected executables: %#v", records[0].Executables)
	}
	for _, record := range records {
		if record.Name == "-" {
			t.Fatalf("executable detail line was parsed as a tool: %#v", records)
		}
	}
}

func TestParseOutdatedTools(t *testing.T) {
	output := `ruff v0.8.0 -> v0.9.1
httpie v3.1.0 (latest: v3.2.4)`

	records := parseOutdatedTools(output)
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].Name != "ruff" || records[0].Version != "0.8.0" || records[0].LatestVersion != "0.9.1" {
		t.Fatalf("unexpected first outdated record: %#v", records[0])
	}
	if records[1].Name != "httpie" || records[1].Version != "3.1.0" || records[1].LatestVersion != "3.2.4" {
		t.Fatalf("unexpected second outdated record: %#v", records[1])
	}
}
