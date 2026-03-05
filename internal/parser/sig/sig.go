package sig

import "fmt"

// Sig is a single extracted signature with its 1-based source line number.
type Sig struct {
	Line int
	Text string
}

// FormatLine returns "relPath:line: text" matching grep-style output.
func (s Sig) FormatLine(relPath string) string {
	return fmt.Sprintf("%s:%d: %s", relPath, s.Line, s.Text)
}
