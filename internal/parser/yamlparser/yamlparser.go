package yamlparser

import (
	"strconv"
	"strings"

	"github.com/TrNgTien/vfs/internal/parser/sig"
)

// ExtractExportedFuncs parses a YAML file and returns structural signatures.
// It detects common DevOps formats (docker-compose, Kubernetes, GitHub Actions,
// GitLab CI, etc.) and extracts meaningful top-level structure.
//
// Example output for docker-compose.yml:
//
//	services:
//	  services.api: { image, ports, environment, volumes }
//	  services.db: { image, ports, volumes }
//	volumes:
//
// Example output for a Kubernetes manifest:
//
//	apiVersion: apps/v1
//	kind: Deployment
//	metadata.name: my-app
//	spec.replicas: 3
//
// Example output for GitHub Actions:
//
//	name: CI
//	on: [push, pull_request]
//	jobs:
//	  jobs.build: { runs-on, steps }
//	  jobs.test: { runs-on, needs, steps }
func ExtractExportedFuncs(_ string, src []byte) ([]sig.Sig, error) {
	lines := strings.Split(string(src), "\n")
	entries := parseIndentTree(lines)

	if len(entries) == 0 {
		return nil, nil
	}

	format := detectFormat(entries)
	var sigs []sig.Sig

	switch format {
	case formatKubernetes:
		sigs = extractKubernetes(entries)
	case formatCompose:
		sigs = extractCompose(entries)
	case formatGitHubActions:
		sigs = extractGitHubActions(entries)
	default:
		sigs = extractGeneric(entries)
	}

	return sigs, nil
}

type yamlFormat int

const (
	formatGeneric yamlFormat = iota
	formatKubernetes
	formatCompose
	formatGitHubActions
)

// entry represents a parsed YAML line with its indentation level and children.
type entry struct {
	lineNum  int // 1-based source line number
	indent   int
	key      string
	value    string
	children []entry
}

// parseIndentTree builds a tree from YAML lines based on indentation.
func parseIndentTree(lines []string) []entry {
	var roots []entry
	var stack []*[]entry
	var indents []int

	stack = append(stack, &roots)
	indents = append(indents, -1)

	for idx, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || trimmed == "---" || trimmed == "..." {
			continue
		}

		indent := countIndent(raw)
		key, value := splitKeyValue(trimmed)
		if key == "" {
			continue
		}

		e := entry{lineNum: idx + 1, indent: indent, key: key, value: value}

		// Pop stack until we find a parent with less indentation
		for len(indents) > 1 && indent <= indents[len(indents)-1] {
			stack = stack[:len(stack)-1]
			indents = indents[:len(indents)-1]
		}

		parent := stack[len(stack)-1]
		*parent = append(*parent, e)

		// Push this entry as potential parent
		i := len(*parent) - 1
		stack = append(stack, &(*parent)[i].children)
		indents = append(indents, indent)
	}

	return roots
}

func detectFormat(entries []entry) yamlFormat {
	keys := make(map[string]string)
	for _, e := range entries {
		keys[e.key] = e.value
	}

	if _, ok := keys["apiVersion"]; ok {
		if _, ok2 := keys["kind"]; ok2 {
			return formatKubernetes
		}
	}

	if _, ok := keys["services"]; ok {
		return formatCompose
	}

	if _, ok := keys["on"]; ok {
		if _, ok2 := keys["jobs"]; ok2 {
			return formatGitHubActions
		}
	}

	return formatGeneric
}

// extractKubernetes pulls apiVersion, kind, metadata.name, and key spec fields.
func extractKubernetes(entries []entry) []sig.Sig {
	var sigs []sig.Sig

	for _, e := range entries {
		switch e.key {
		case "apiVersion", "kind":
			sigs = append(sigs, sig.Sig{Line: e.lineNum, Text: e.key + ": " + e.value})
		case "metadata":
			for _, child := range e.children {
				if child.key == "name" || child.key == "namespace" {
					sigs = append(sigs, sig.Sig{Line: child.lineNum, Text: "metadata." + child.key + ": " + child.value})
				}
				if child.key == "labels" {
					sigs = append(sigs, sig.Sig{Line: child.lineNum, Text: "metadata.labels: {" + childKeys(child.children) + "}"})
				}
			}
		case "spec":
			sigs = append(sigs, extractSpecSummary(e)...)
		case "data", "stringData":
			sigs = append(sigs, sig.Sig{Line: e.lineNum, Text: e.key + ": {" + childKeys(e.children) + "}"})
		}
	}

	return sigs
}

func extractSpecSummary(spec entry) []sig.Sig {
	var sigs []sig.Sig
	for _, child := range spec.children {
		switch child.key {
		case "replicas":
			sigs = append(sigs, sig.Sig{Line: child.lineNum, Text: "spec.replicas: " + child.value})
		case "selector", "template":
			sigs = append(sigs, sig.Sig{Line: child.lineNum, Text: "spec." + child.key + ": { ... }"})
		case "containers":
			for _, c := range child.children {
				name := findChildValue(c.children, "name")
				image := findChildValue(c.children, "image")
				if name != "" {
					text := "spec.containers." + name
					if image != "" {
						text += ": {image: " + image + "}"
					}
					sigs = append(sigs, sig.Sig{Line: c.lineNum, Text: text})
				}
			}
		case "ports":
			sigs = append(sigs, sig.Sig{Line: child.lineNum, Text: "spec.ports: [" + summarizePorts(child.children) + "]"})
		case "rules":
			sigs = append(sigs, sig.Sig{Line: child.lineNum, Text: "spec.rules: [" + summarizeListCount(child.children) + "]"})
		case "type":
			sigs = append(sigs, sig.Sig{Line: child.lineNum, Text: "spec.type: " + child.value})
		default:
			if child.value != "" {
				sigs = append(sigs, sig.Sig{Line: child.lineNum, Text: "spec." + child.key + ": " + child.value})
			} else if len(child.children) > 0 {
				sigs = append(sigs, sig.Sig{Line: child.lineNum, Text: "spec." + child.key + ": { ... }"})
			}
		}
	}
	return sigs
}

// extractCompose pulls services with their key config fields.
func extractCompose(entries []entry) []sig.Sig {
	var sigs []sig.Sig

	for _, e := range entries {
		switch e.key {
		case "services":
			sigs = append(sigs, sig.Sig{Line: e.lineNum, Text: "services:"})
			for _, svc := range e.children {
				keys := childKeys(svc.children)
				sigs = append(sigs, sig.Sig{Line: svc.lineNum, Text: "  services." + svc.key + ": {" + keys + "}"})
			}
		case "volumes", "networks", "secrets", "configs":
			sigs = append(sigs, sig.Sig{Line: e.lineNum, Text: e.key + ":"})
			for _, child := range e.children {
				sigs = append(sigs, sig.Sig{Line: child.lineNum, Text: "  " + e.key + "." + child.key})
			}
		case "version", "name":
			if e.value != "" {
				sigs = append(sigs, sig.Sig{Line: e.lineNum, Text: e.key + ": " + e.value})
			}
		}
	}

	return sigs
}

// extractGitHubActions pulls workflow name, triggers, and job summaries.
func extractGitHubActions(entries []entry) []sig.Sig {
	var sigs []sig.Sig

	for _, e := range entries {
		switch e.key {
		case "name":
			sigs = append(sigs, sig.Sig{Line: e.lineNum, Text: "name: " + e.value})
		case "on":
			if e.value != "" {
				sigs = append(sigs, sig.Sig{Line: e.lineNum, Text: "on: " + e.value})
			} else {
				sigs = append(sigs, sig.Sig{Line: e.lineNum, Text: "on: [" + childKeys(e.children) + "]"})
			}
		case "env":
			sigs = append(sigs, sig.Sig{Line: e.lineNum, Text: "env: {" + childKeys(e.children) + "}"})
		case "permissions":
			if e.value != "" {
				sigs = append(sigs, sig.Sig{Line: e.lineNum, Text: "permissions: " + e.value})
			} else {
				sigs = append(sigs, sig.Sig{Line: e.lineNum, Text: "permissions: {" + childKeys(e.children) + "}"})
			}
		case "jobs":
			sigs = append(sigs, sig.Sig{Line: e.lineNum, Text: "jobs:"})
			for _, job := range e.children {
				keys := childKeys(job.children)
				sigs = append(sigs, sig.Sig{Line: job.lineNum, Text: "  jobs." + job.key + ": {" + keys + "}"})
			}
		}
	}

	return sigs
}

// extractGeneric returns top-level keys with scalar values or child key summaries.
func extractGeneric(entries []entry) []sig.Sig {
	var sigs []sig.Sig

	for _, e := range entries {
		if e.value != "" {
			sigs = append(sigs, sig.Sig{Line: e.lineNum, Text: e.key + ": " + e.value})
		} else if len(e.children) > 0 {
			sigs = append(sigs, sig.Sig{Line: e.lineNum, Text: e.key + ":"})
			for _, child := range e.children {
				if child.value != "" {
					sigs = append(sigs, sig.Sig{Line: child.lineNum, Text: "  " + e.key + "." + child.key + ": " + child.value})
				} else {
					sigs = append(sigs, sig.Sig{Line: child.lineNum, Text: "  " + e.key + "." + child.key + ": { ... }"})
				}
			}
		} else {
			sigs = append(sigs, sig.Sig{Line: e.lineNum, Text: e.key + ":"})
		}
	}

	return sigs
}

// --- helpers ---

func countIndent(line string) int {
	n := 0
	for _, ch := range line {
		if ch == ' ' {
			n++
		} else if ch == '\t' {
			n += 2
		} else {
			break
		}
	}
	return n
}

func splitKeyValue(line string) (string, string) {
	// Skip list items prefix "- "
	if strings.HasPrefix(line, "- ") {
		line = strings.TrimPrefix(line, "- ")
	} else if line == "-" {
		return "", ""
	}

	idx := strings.IndexByte(line, ':')
	if idx < 0 {
		return "", ""
	}

	key := strings.TrimSpace(line[:idx])
	// Keys shouldn't contain spaces (that would be a value line)
	if strings.ContainsAny(key, " \t") && !strings.HasPrefix(key, "\"") {
		return "", ""
	}
	key = strings.Trim(key, "\"'")

	value := strings.TrimSpace(line[idx+1:])
	// Strip inline comments
	if ci := strings.Index(value, " #"); ci >= 0 {
		value = strings.TrimSpace(value[:ci])
	}

	return key, value
}

func childKeys(children []entry) string {
	var keys []string
	for _, c := range children {
		keys = append(keys, c.key)
	}
	return strings.Join(keys, ", ")
}

func findChildValue(children []entry, key string) string {
	for _, c := range children {
		if c.key == key {
			return c.value
		}
	}
	return ""
}

func summarizePorts(children []entry) string {
	var ports []string
	for _, c := range children {
		if c.key == "port" && c.value != "" {
			ports = append(ports, c.value)
		} else if c.value != "" {
			ports = append(ports, c.value)
		}
	}
	if len(ports) > 0 {
		return strings.Join(ports, ", ")
	}
	return summarizeListCount(children)
}

func summarizeListCount(children []entry) string {
	if len(children) == 1 {
		return "1 item"
	}
	return strconv.Itoa(len(children)) + " items"
}
