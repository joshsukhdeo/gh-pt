package ai

import (
	"bufio"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ManifestTemplate is injected into the AI prompt. The AI fills in simple
// directives — it never writes JSON. A deterministic Go parser constructs
// the structured payload from these directives.
const ManifestTemplate = `
OUTPUT FORMAT — follow EXACTLY. Do NOT output JSON.

DEPENDENCY MANIFEST (output as a single code block with language tag "ghpt-manifest"):
` + "```ghpt-manifest" + `
toolchain <primary build tool, e.g. gcc, cargo, go, make>
dep <package-name> <resolver: apt|dnf|pacman|mise|uv|cargo|vcpkg>
dep <package-name> <resolver>
` + "```" + `

COMPILATION SCRIPT (output as a single bash code block):
` + "```bash" + `
#!/bin/bash
set -euo pipefail
# Your build commands here
` + "```" + `

Rules:
- Each "dep" line has exactly two arguments: the package name and the resolver.
- The "toolchain" line has exactly one argument.
- Do NOT wrap the manifest in JSON. Use the directive format above.
- Do NOT add comments or extra text inside the ghpt-manifest block.
`

var (
	// ErrMissingManifest indicates that no ghpt-manifest code block was found.
	ErrMissingManifest = errors.New("missing dependency manifest (ghpt-manifest code block)")
	// ErrMissingScript indicates that no Bash/sh compilation script code block was found.
	ErrMissingScript = errors.New("missing compilation script (bash/sh code block)")
	// ErrMalformedDirective indicates a directive line could not be parsed.
	ErrMalformedDirective = errors.New("malformed directive in manifest block")

	// Aliases for backward compatibility.
	ErrMissingJSONBlock = ErrMissingManifest
	ErrMissingBashBlock = ErrMissingScript
	// Keep for callers that previously caught this.
	ErrMalformedJSON = ErrMalformedDirective
)

// Dependency represents a package dependency and the resolver required to install it.
type Dependency struct {
	Name     string `json:"name"`
	Resolver string `json:"resolver"`
}

// CompilePayload represents the parsed 2-stage AI output containing dependencies,
// toolchain, and the compilation script.
type CompilePayload struct {
	Dependencies []Dependency `json:"dependencies"`
	Toolchain    string       `json:"toolchain"`
	Script       string       `json:"script"`
}

var (
	manifestBlockRegex = regexp.MustCompile("(?is)```ghpt-manifest\\b[^\\r\\n]*\\r?\\n?(.*?)```")
	bashBlockRegex     = regexp.MustCompile("(?is)```(?:bash|sh)\\b[^\\r\\n]*\\r?\\n?(.*?)```")
)

// parseDirectiveBlock deterministically constructs a CompilePayload from
// simple "toolchain" and "dep" directive lines. The AI never writes JSON.
func parseDirectiveBlock(block string) ([]Dependency, string, error) {
	var deps []Dependency
	var toolchain string

	scanner := bufio.NewScanner(strings.NewReader(block))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		directive := strings.ToLower(fields[0])

		switch directive {
		case "toolchain":
			if len(fields) < 2 {
				return nil, "", fmt.Errorf("%w: toolchain requires an argument: %q", ErrMalformedDirective, line)
			}
			toolchain = fields[1]
		case "dep":
			if len(fields) < 3 {
				return nil, "", fmt.Errorf("%w: dep requires <name> <resolver>: %q", ErrMalformedDirective, line)
			}
			deps = append(deps, Dependency{Name: fields[1], Resolver: fields[2]})
		default:
			return nil, "", fmt.Errorf("%w: unknown directive %q: %q", ErrMalformedDirective, directive, line)
		}
	}

	if deps == nil {
		deps = []Dependency{}
	}

	return deps, toolchain, nil
}

// ParseAIOutput extracts the dependency manifest (ghpt-manifest directives) and
// compilation script (Bash/sh) from AI-generated response text. The AI outputs
// simple directives — this function deterministically constructs the payload.
func ParseAIOutput(raw string) (*CompilePayload, error) {
	manifestMatch := manifestBlockRegex.FindStringSubmatch(raw)
	if len(manifestMatch) < 2 {
		return nil, ErrMissingManifest
	}

	bashMatch := bashBlockRegex.FindStringSubmatch(raw)
	if len(bashMatch) < 2 {
		return nil, ErrMissingScript
	}

	manifestStr := strings.TrimSpace(manifestMatch[1])
	if manifestStr == "" {
		return nil, fmt.Errorf("%w: empty manifest block", ErrMalformedDirective)
	}

	deps, toolchain, err := parseDirectiveBlock(manifestStr)
	if err != nil {
		return nil, err
	}

	script := strings.TrimSpace(bashMatch[1])
	if script == "" {
		return nil, ErrMissingScript
	}

	return &CompilePayload{
		Dependencies: deps,
		Toolchain:    toolchain,
		Script:       script,
	}, nil
}
