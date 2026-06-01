package uvtool

import (
	"regexp"
	"strings"
	"unipac-wails/backend/core/manager"
)

const managerID = "uv-tool"

var (
	toolLinePattern      = regexp.MustCompile(`^([^-][^\s]*)\s+v?([^\s\(\[]+)`)
	parenPattern         = regexp.MustCompile(`\(([^)]*)\)`)
	pythonBracketPattern = regexp.MustCompile(`\[(CPython[^\]]*|PyPy[^\]]*|Python[^\]]*)\]`)
	requiredSpecPattern  = regexp.MustCompile(`\[required:\s*([^\]]+)\]`)
	pathPattern          = regexp.MustCompile(`(?i)^(?:tool\s+)?(?:environment\s+)?path:\s*(.+)$`)
	executablePattern    = regexp.MustCompile(`(?i)^-?\s*(?:executable|script)?s?:?\s*(/.+)$`)
	latestPatterns       = []*regexp.Regexp{
		regexp.MustCompile(`^([^\s]+)\s+v?([^\s\(\[]+)\s+->\s+v?([^\s\)\]]+)`),
		regexp.MustCompile(`^([^\s]+)\s+v?([^\s\(\[]+).*?(?:latest|available)[\s:]+v?([^\s\)\]]+)`),
	}
)

func parseInstalledTools(output string) []toolRecord {
	output = strings.TrimSpace(output)
	if output == "" || strings.EqualFold(output, "No tools installed") {
		return []toolRecord{}
	}

	records := make([]toolRecord, 0)
	var current *toolRecord
	inExecutables := false

	flush := func() {
		if current != nil && current.Name != "" {
			records = append(records, *current)
		}
	}

	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		if isExecutableLine(line) {
			if current != nil {
				current.Executables = append(current.Executables, parseExecutableLine(line))
			}
			inExecutables = false
			continue
		}

		if !isIndented(rawLine) {
			if matches := toolLinePattern.FindStringSubmatch(line); len(matches) == 3 {
				flush()
				current = &toolRecord{Name: matches[1], Version: trimVersion(matches[2])}
				parseInlineDetails(current, line)
				inExecutables = false
				continue
			}
		}

		if current == nil {
			continue
		}

		lower := strings.ToLower(line)
		switch {
		case strings.Contains(lower, "executable"):
			inExecutables = true
			if matches := executablePattern.FindStringSubmatch(line); len(matches) == 2 {
				current.Executables = append(current.Executables, strings.TrimSpace(matches[1]))
			}
		case inExecutables && isExecutableLine(line):
			executable := parseExecutableLine(line)
			if executable != "" {
				current.Executables = append(current.Executables, executable)
			}
		case pathPattern.MatchString(line):
			matches := pathPattern.FindStringSubmatch(line)
			current.ToolPath = strings.TrimSpace(matches[1])
			inExecutables = false
		default:
			parseInlineDetails(current, line)
			inExecutables = false
		}
	}
	flush()

	return records
}

func parseOutdatedTools(output string) []toolRecord {
	output = strings.TrimSpace(output)
	if output == "" || strings.EqualFold(output, "No tools installed") {
		return []toolRecord{}
	}

	records := make([]toolRecord, 0)
	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || isIndented(rawLine) {
			continue
		}
		for _, pattern := range latestPatterns {
			if matches := pattern.FindStringSubmatch(line); len(matches) == 4 {
				records = append(records, toolRecord{
					Name:          matches[1],
					Version:       trimVersion(matches[2]),
					LatestVersion: trimVersion(matches[3]),
				})
				break
			}
		}
	}
	return records
}

func parseInlineDetails(record *toolRecord, line string) {
	if matches := requiredSpecPattern.FindStringSubmatch(line); len(matches) == 2 {
		record.VersionSpecifier = strings.TrimSpace(matches[1])
	}
	if matches := pythonBracketPattern.FindStringSubmatch(line); len(matches) == 2 {
		record.Python = strings.TrimSpace(matches[1])
	}

	for _, match := range parenPattern.FindAllStringSubmatch(line, -1) {
		if len(match) != 2 {
			continue
		}
		value := strings.TrimSpace(match[1])
		if strings.HasPrefix(strings.ToLower(value), "python") || strings.HasPrefix(value, "CPython") || strings.HasPrefix(value, "PyPy") {
			record.Python = value
			continue
		}
		if strings.HasPrefix(value, "/") {
			record.ToolPath = value
			continue
		}
		if value != "" && record.VersionSpecifier == "" {
			record.VersionSpecifier = value
		}
	}
}

func normalizeInstalled(records []toolRecord) []manager.Package {
	pkgs := make([]manager.Package, 0, len(records))
	for _, record := range records {
		if record.Name == "" {
			continue
		}
		pkgs = append(pkgs, manager.Package{
			Name:      record.Name,
			Fullname:  record.VersionSpecifier,
			Version:   record.Version,
			Manager:   managerID,
			Installed: true,
			IsGUI:     false,
		})
	}
	return pkgs
}

func normalizeOutdated(records []toolRecord) []manager.Package {
	pkgs := make([]manager.Package, 0, len(records))
	for _, record := range records {
		if record.Name == "" {
			continue
		}
		pkgs = append(pkgs, manager.Package{
			Name:          record.Name,
			Version:       record.Version,
			LatestVersion: record.LatestVersion,
			Manager:       managerID,
			Installed:     true,
			Outdated:      true,
			IsGUI:         false,
		})
	}
	return pkgs
}

func findTool(records []toolRecord, name string) (toolRecord, bool) {
	for _, record := range records {
		if record.Name == name {
			return record, true
		}
	}
	return toolRecord{}, false
}

func isIndented(line string) bool {
	return strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
}

func isExecutableLine(line string) bool {
	line = strings.TrimSpace(line)
	return strings.HasPrefix(line, "- ")
}

func parseExecutableLine(line string) string {
	executable := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
	if idx := strings.Index(executable, " "); idx > 0 {
		return strings.TrimSpace(executable[:idx])
	}
	return executable
}

func trimVersion(version string) string {
	return strings.Trim(strings.TrimSpace(version), ",;")
}
