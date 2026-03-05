package protoparser

import (
	"strings"
)

// ExtractExportedFuncs parses a Protocol Buffers file and returns signatures
// for top-level definitions: syntax, package, service (with rpcs), message,
// enum, option, and import statements.
//
// Example output:
//
//	syntax = "proto3"
//	package api.v1
//	import "google/protobuf/timestamp.proto"
//	service UserService { ... }
//	  rpc CreateUser(CreateUserRequest) returns (CreateUserResponse)
//	  rpc GetUser(GetUserRequest) returns (User)
//	message User { ... }
//	message CreateUserRequest { ... }
//	enum Status { ... }
func ExtractExportedFuncs(_ string, src []byte) ([]string, error) {
	lines := strings.Split(string(src), "\n")
	var sigs []string

	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		i++

		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") {
			continue
		}

		switch {
		case strings.HasPrefix(line, "syntax"):
			sigs = append(sigs, normalizeSemicolon(line))

		case strings.HasPrefix(line, "package"):
			sigs = append(sigs, normalizeSemicolon(line))

		case strings.HasPrefix(line, "import"):
			sigs = append(sigs, normalizeSemicolon(line))

		case strings.HasPrefix(line, "option"):
			sigs = append(sigs, normalizeSemicolon(line))

		case strings.HasPrefix(line, "service"):
			name := extractBlockName(line, "service")
			sigs = append(sigs, "service "+name+" { ... }")
			rpcs := collectRPCs(line, lines, &i)
			sigs = append(sigs, rpcs...)

		case strings.HasPrefix(line, "message"):
			name := extractBlockName(line, "message")
			sigs = append(sigs, "message "+name+" { ... }")

		case strings.HasPrefix(line, "enum"):
			name := extractBlockName(line, "enum")
			sigs = append(sigs, "enum "+name+" { ... }")

		case strings.HasPrefix(line, "oneof"):
			name := extractBlockName(line, "oneof")
			sigs = append(sigs, "oneof "+name+" { ... }")

		case strings.HasPrefix(line, "extend"):
			name := extractBlockName(line, "extend")
			sigs = append(sigs, "extend "+name+" { ... }")
		}
	}

	return sigs, nil
}

// collectRPCs scans inside a service block and returns indented rpc signatures.
// serviceLine is the "service Foo {" line already consumed by the caller.
func collectRPCs(serviceLine string, lines []string, idx *int) []string {
	var rpcs []string

	// Count braces starting from the service line itself
	depth := strings.Count(serviceLine, "{") - strings.Count(serviceLine, "}")

	for *idx < len(lines) && depth > 0 {
		line := strings.TrimSpace(lines[*idx])
		*idx++

		depth += strings.Count(line, "{") - strings.Count(line, "}")

		if strings.HasPrefix(line, "rpc") {
			rpcs = append(rpcs, "  "+normalizeRPC(line))
		}
	}

	return rpcs
}

// normalizeRPC cleans up an rpc line to a canonical one-liner.
// "rpc GetUser (GetUserRequest) returns (User) {}" -> "rpc GetUser(GetUserRequest) returns (User)"
func normalizeRPC(line string) string {
	// Strip trailing braces, semicolons, and options block
	if idx := strings.Index(line, "{"); idx > 0 {
		line = strings.TrimSpace(line[:idx])
	}
	line = strings.TrimRight(line, "; ")

	// Collapse "rpc Name (Req)" -> "rpc Name(Req)"
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return line
	}

	var b strings.Builder
	b.WriteString("rpc ")
	for i := 1; i < len(parts); i++ {
		token := parts[i]
		if i > 1 {
			prev := parts[i-1]
			needsSpace := true
			// Collapse "Name (" -> "Name(" but keep "returns (" -> "returns ("
			if strings.HasPrefix(token, "(") && prev != "returns" {
				needsSpace = false
			}
			if needsSpace {
				b.WriteByte(' ')
			}
		}
		b.WriteString(token)
	}
	return b.String()
}

func extractBlockName(line, keyword string) string {
	rest := strings.TrimSpace(strings.TrimPrefix(line, keyword))
	// Take up to the first '{' or end
	if idx := strings.IndexByte(rest, '{'); idx >= 0 {
		rest = strings.TrimSpace(rest[:idx])
	}
	// First token is the name
	if fields := strings.Fields(rest); len(fields) > 0 {
		return fields[0]
	}
	return ""
}

func normalizeSemicolon(line string) string {
	return strings.TrimRight(strings.TrimSpace(line), ";")
}
