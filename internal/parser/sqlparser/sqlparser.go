package sqlparser

import (
	"strings"
)

// DDL keywords that start a definition we want to capture.
var ddlKeywords = map[string]bool{
	"CREATE": true,
	"ALTER":  true,
	"DROP":   true,
}

// ExtractExportedFuncs parses a SQL file and returns one-line signatures
// for DDL statements and data-seeding INSERTs. It uses a line-based approach
// that handles comments and multi-line statements without full SQL parsing.
//
// Example output:
//
//	CREATE TABLE users (id, email, name, password_hash, status, created_at, updated_at)
//	CREATE UNIQUE INDEX idx_users_email ON users (email)
//	CREATE OR REPLACE VIEW active_users AS ...
//	CREATE OR REPLACE FUNCTION update_updated_at() RETURNS TRIGGER
//	CREATE TRIGGER trigger_update_users_timestamp BEFORE UPDATE ON users
//	ALTER TABLE users ADD COLUMN avatar_url TEXT
//	INSERT INTO roles
func ExtractExportedFuncs(_ string, src []byte) ([]string, error) {
	lines := strings.Split(string(src), "\n")
	var sigs []string

	i := 0
	for i < len(lines) {
		line := stripLineComment(strings.TrimSpace(lines[i]))
		i++

		if line == "" {
			continue
		}

		first := firstWord(line)
		upper := strings.ToUpper(first)

		if !ddlKeywords[upper] && upper != "INSERT" && upper != "GRANT" && upper != "REVOKE" {
			continue
		}

		// Collect the full statement (until semicolon)
		stmt := line
		for !strings.Contains(stmt, ";") && i < len(lines) {
			next := stripLineComment(strings.TrimSpace(lines[i]))
			i++
			if next == "" {
				continue
			}
			stmt += " " + next
		}

		// Trim everything after the semicolon
		if idx := strings.IndexByte(stmt, ';'); idx >= 0 {
			stmt = stmt[:idx]
		}
		stmt = strings.TrimSpace(stmt)

		if sig := formatStatement(stmt); sig != "" {
			sigs = append(sigs, sig)
		}
	}

	return sigs, nil
}

func formatStatement(stmt string) string {
	upper := strings.ToUpper(stmt)

	switch {
	case strings.HasPrefix(upper, "CREATE"):
		return formatCreate(stmt, upper)
	case strings.HasPrefix(upper, "ALTER"):
		return firstLineClean(stmt)
	case strings.HasPrefix(upper, "DROP"):
		return formatDrop(stmt, upper)
	case strings.HasPrefix(upper, "INSERT"):
		return formatInsert(stmt, upper)
	case strings.HasPrefix(upper, "GRANT"), strings.HasPrefix(upper, "REVOKE"):
		return firstLineClean(stmt)
	}
	return ""
}

func formatCreate(stmt, upper string) string {
	// Strip optional modifiers to find the object type
	// CREATE [OR REPLACE] [TEMP|TEMPORARY] [UNIQUE] [MATERIALIZED] [UNLOGGED] <TYPE> ...
	words := strings.Fields(stmt)
	if len(words) < 2 {
		return ""
	}

	idx := 1
	upperWords := strings.Fields(upper)
	for idx < len(upperWords) {
		w := upperWords[idx]
		if w == "OR" || w == "REPLACE" || w == "TEMP" || w == "TEMPORARY" ||
			w == "UNIQUE" || w == "MATERIALIZED" || w == "UNLOGGED" {
			idx++
			continue
		}
		break
	}
	if idx >= len(upperWords) {
		return ""
	}

	objType := upperWords[idx]
	idx++

	// Skip "IF NOT EXISTS"
	if idx+2 < len(upperWords) && upperWords[idx] == "IF" &&
		upperWords[idx+1] == "NOT" && upperWords[idx+2] == "EXISTS" {
		idx += 3
	}

	name := ""
	if idx < len(words) {
		name = words[idx]
	}

	// Build prefix preserving original case for name
	var prefix []string
	for i := 0; i < idx; i++ {
		prefix = append(prefix, upperWords[i])
	}
	header := strings.Join(prefix, " ") + " " + name

	switch objType {
	case "TABLE":
		return formatTable(header, stmt)
	case "FUNCTION", "PROCEDURE":
		// Strip trailing parens from name -- formatCallable re-extracts them from stmt
		if parenIdx := strings.IndexByte(name, '('); parenIdx >= 0 {
			name = name[:parenIdx]
			header = strings.Join(prefix, " ") + " " + name
		}
		return formatCallable(header, stmt)
	case "INDEX":
		return formatIndex(header, stmt, upper)
	case "TRIGGER":
		return formatTrigger(header, stmt, upper)
	case "VIEW":
		return header + " AS ..."
	default:
		return header
	}
}

// formatTable extracts column names from CREATE TABLE.
func formatTable(header, stmt string) string {
	parenStart := strings.IndexByte(stmt, '(')
	if parenStart < 0 {
		return header
	}

	parenEnd := findMatchingParen(stmt, parenStart)
	if parenEnd < 0 {
		return header + " (...)"
	}

	body := stmt[parenStart+1 : parenEnd]
	names := extractColumnNames(body)
	if len(names) == 0 {
		return header + " (...)"
	}

	return header + " (" + strings.Join(names, ", ") + ")"
}

// formatCallable extracts parameter list and RETURNS clause.
func formatCallable(header, stmt string) string {
	parenStart := strings.IndexByte(stmt, '(')
	if parenStart < 0 {
		return header + "()"
	}

	parenEnd := findMatchingParen(stmt, parenStart)
	if parenEnd < 0 {
		return header + "(...)"
	}

	params := stmt[parenStart : parenEnd+1]
	rest := strings.TrimSpace(stmt[parenEnd+1:])
	sig := header + params

	// Capture RETURNS / RETURN clause
	restUpper := strings.ToUpper(rest)
	if strings.HasPrefix(restUpper, "RETURNS ") || strings.HasPrefix(restUpper, "RETURN ") {
		// Take up to AS, LANGUAGE, or $$ (body start)
		end := len(rest)
		for _, marker := range []string{" AS ", " LANGUAGE ", "$$", " BEGIN "} {
			if idx := strings.Index(strings.ToUpper(rest), marker); idx >= 0 && idx < end {
				end = idx
			}
		}
		sig += " " + strings.TrimSpace(rest[:end])
	}

	return sig
}

func formatIndex(header, stmt, upper string) string {
	// Find "ON table_name"
	if idx := strings.Index(upper, " ON "); idx >= 0 {
		rest := strings.TrimSpace(stmt[idx+4:])
		tableName := firstWord(rest)
		return header + " ON " + tableName
	}
	return header
}

func formatTrigger(header, stmt, upper string) string {
	// Find "BEFORE|AFTER|INSTEAD OF ... ON table_name"
	for _, timing := range []string{" BEFORE ", " AFTER ", " INSTEAD "} {
		if idx := strings.Index(upper, timing); idx >= 0 {
			rest := stmt[idx:]
			if onIdx := strings.Index(strings.ToUpper(rest), " ON "); onIdx >= 0 {
				afterOn := strings.TrimSpace(rest[onIdx+4:])
				tableName := firstWord(afterOn)
				return header + " " + strings.TrimSpace(rest[:onIdx]) + " ON " + tableName
			}
		}
	}
	return header
}

func formatDrop(stmt, upper string) string {
	words := strings.Fields(stmt)
	upperWords := strings.Fields(upper)
	if len(words) < 3 {
		return ""
	}

	idx := 1
	// Object type
	objType := upperWords[idx]
	idx++

	// Skip "IF EXISTS"
	if idx+1 < len(upperWords) && upperWords[idx] == "IF" && upperWords[idx+1] == "EXISTS" {
		idx += 2
	}

	name := ""
	if idx < len(words) {
		name = words[idx]
	}
	return "DROP " + objType + " " + name
}

func formatInsert(stmt, upper string) string {
	// INSERT INTO table_name
	if idx := strings.Index(upper, "INTO "); idx >= 0 {
		rest := strings.TrimSpace(stmt[idx+5:])
		tableName := firstWord(rest)
		return "INSERT INTO " + tableName
	}
	return ""
}

// --- helpers ---

func findMatchingParen(s string, start int) int {
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		case '\'':
			// Skip quoted strings
			i++
			for i < len(s) && s[i] != '\'' {
				i++
			}
		}
	}
	return -1
}

// extractColumnNames pulls the first word of each comma-separated item,
// skipping constraint keywords.
func extractColumnNames(body string) []string {
	var names []string
	depth := 0
	start := 0

	for i := 0; i <= len(body); i++ {
		if i < len(body) {
			switch body[i] {
			case '(':
				depth++
				continue
			case ')':
				depth--
				continue
			case ',':
				if depth > 0 {
					continue
				}
			default:
				continue
			}
		}

		col := strings.TrimSpace(body[start:i])
		start = i + 1

		if col == "" {
			continue
		}

		token := firstWord(col)
		upper := strings.ToUpper(token)

		// Skip constraints
		if upper == "PRIMARY" || upper == "UNIQUE" || upper == "CHECK" ||
			upper == "FOREIGN" || upper == "CONSTRAINT" || upper == "EXCLUDE" {
			names = append(names, upper+" KEY")
			continue
		}

		names = append(names, token)
	}

	return names
}

func firstWord(s string) string {
	s = strings.TrimSpace(s)
	for i, ch := range s {
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '(' || ch == ')' {
			return s[:i]
		}
	}
	return s
}

func firstLineClean(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return strings.TrimSpace(s[:idx])
	}
	return strings.TrimSpace(s)
}

func stripLineComment(line string) string {
	if strings.HasPrefix(line, "--") {
		return ""
	}
	// Don't strip inline -- inside quotes (simplified: just strip if not inside quotes)
	inQuote := false
	for i := 0; i < len(line)-1; i++ {
		if line[i] == '\'' {
			inQuote = !inQuote
		}
		if !inQuote && line[i] == '-' && line[i+1] == '-' {
			return strings.TrimSpace(line[:i])
		}
	}
	return line
}
