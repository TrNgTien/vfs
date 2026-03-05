package dockerparser

import (
	"strings"
)

// Dockerfile instructions that produce meaningful signatures.
// RUN is intentionally excluded -- its bodies are long shell scripts
// that provide little structural value as a table of contents.
var signatureInstructions = map[string]bool{
	"FROM":        true,
	"ARG":         true,
	"ENV":         true,
	"EXPOSE":      true,
	"COPY":        true,
	"ADD":         true,
	"CMD":         true,
	"ENTRYPOINT":  true,
	"WORKDIR":     true,
	"USER":        true,
	"VOLUME":      true,
	"LABEL":       true,
	"HEALTHCHECK": true,
	"STOPSIGNAL":  true,
	"SHELL":       true,
	"ONBUILD":     true,
}

// ExtractExportedFuncs parses a Dockerfile and returns one-line signatures
// for each meaningful instruction. Dockerfiles are inherently line-oriented,
// so line-based parsing is more natural and robust than tree-sitter here.
//
// Example output:
//
//	FROM node:20-alpine AS builder
//	ARG NODE_ENV=production
//	EXPOSE 3000/tcp
//	COPY --from=builder /app/dist ./dist
//	CMD ["node", "server.js"]
func ExtractExportedFuncs(_ string, src []byte) ([]string, error) {
	lines := strings.Split(string(src), "\n")
	var sigs []string

	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		if line == "" || strings.HasPrefix(line, "#") {
			i++
			continue
		}

		// Join continuation lines (trailing backslash)
		full := line
		for strings.HasSuffix(full, "\\") && i+1 < len(lines) {
			i++
			full = strings.TrimSuffix(full, "\\")
			full = strings.TrimRight(full, " \t") + " " + strings.TrimSpace(lines[i])
		}
		i++

		instruction, rest := splitInstruction(full)
		if instruction == "" {
			continue
		}

		upper := strings.ToUpper(instruction)
		if !signatureInstructions[upper] {
			continue
		}

		if rest != "" {
			sigs = append(sigs, upper+" "+rest)
		} else {
			sigs = append(sigs, upper)
		}
	}

	return sigs, nil
}

// splitInstruction separates the Dockerfile instruction keyword from its arguments.
// Returns ("", "") for non-instruction lines.
func splitInstruction(line string) (string, string) {
	// Handle parser directives (# directive=value) -- skip them
	if strings.HasPrefix(line, "#") {
		return "", ""
	}

	idx := strings.IndexAny(line, " \t")
	if idx < 0 {
		return line, ""
	}

	keyword := line[:idx]
	// Dockerfile instructions are ASCII letters only
	for _, c := range keyword {
		if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') {
			return "", ""
		}
	}

	return keyword, strings.TrimSpace(line[idx+1:])
}
