#!/usr/bin/env bash
#
# install-rules.sh -- install the vfs agent rule into your AI tool.
#
# Quick start (after cloning this repo):
#   ./scripts/install-rules.sh                  # auto-detect your AI tool
#   ./scripts/install-rules.sh --agent cursor   # install for one tool
#   ./scripts/install-rules.sh --agent all      # install for every tool
#   ./scripts/install-rules.sh --project ../myapp --agent claude
#
# Modeled after `init --agent <tool>` CLI patterns -- one command wires the rule
# file into the tool the user actually runs, instead of a manual copy/paste.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
RULE_SOURCE="${REPO_ROOT}/.cursor/rules/vfs-agent-search.mdc"

PROJECT=""
PROJECT_GIVEN=0
AGENTS=()
FORCE=0
DRY_RUN=0
LIST=0

usage() {
	cat <<EOF
Usage: $(basename "$0") [options]

Install the vfs agent rule (vfs-agent-search.mdc) into your AI tool so agents
use vfs for code discovery instead of grep / reading entire files.

Options:
  --agent <name>    Install for one tool: cursor, claude, antigravity,
                    windsurf, cline, continue, aider, opencode, or 'all'
                    (default: auto-detect from the target project)
  --project <dir>   Project to install the rule into (default: this repo)
  --force           Overwrite any existing rule file
  --dry-run         Print what would be done without writing anything
  --list            List supported tools and their rule locations
  -h, --help        Show this help

Examples:
  ./scripts/install-rules.sh --agent cursor
  ./scripts/install-rules.sh --agent claude --project ../myapp
  ./scripts/install-rules.sh --agent all --force
EOF
}

list_agents() {
	cat <<'EOF'
Supported AI tools and where the rule is installed:

  agent          destination (relative to the target project)
  ------         -------------------------------------------------
  cursor         .cursor/rules/vfs.mdc
  claude         CLAUDE.md
  antigravity    GEMINI.md
  windsurf       .windsurf/rules/vfs.md
  cline          .clinerules
  continue       .continue/rules/vfs.md
  aider          .aider.conventions.md
  opencode       .opencode/rules/vfs.mdc

Use --agent <name> or --agent all. Without --agent the script auto-detects
the tool from the config files already present in the target project.
EOF
}

fail() {
	echo "error: $*" >&2
	exit 1
}

while [[ $# -gt 0 ]]; do
	case "$1" in
		--agent)
			[[ $# -ge 2 ]] || fail "--agent requires a value"
			shift
			while [[ $# -ge 1 ]] && [[ "$1" != -* ]]; do
				if [[ "$1" == "all" ]]; then
					AGENTS=(cursor claude antigravity windsurf cline continue aider opencode)
				else
					AGENTS+=("$1")
				fi
				shift
			done
			;;
		--project)
			[[ $# -ge 2 ]] || fail "--project requires a value"
			PROJECT="$2"
			PROJECT_GIVEN=1
			shift 2
			;;
		--force)
			FORCE=1
			shift
			;;
		--dry-run)
			DRY_RUN=1
			shift
			;;
		--list)
			LIST=1
			shift
			;;
		-h|--help)
			usage
			exit 0
			;;
		*)
			fail "unknown option '$1' (run --help for usage)"
			;;
	esac
done

[[ $LIST -eq 1 ]] && { list_agents; exit 0; }

[[ -f "$RULE_SOURCE" ]] || fail "rule source not found: $RULE_SOURCE"

PROJECT="${PROJECT:-$REPO_ROOT}"
[[ -d "$PROJECT" ]] || fail "target project does not exist: $PROJECT"
PROJECT="$(cd "$PROJECT" && pwd)"

if [[ $PROJECT_GIVEN -eq 0 ]] && [[ "$PROJECT" == "$REPO_ROOT" ]]; then
	echo "note: no --project given, so rules will be installed into the vfs repo itself."
	echo "      To install into your own project instead, pass --project <dir>."
	echo ""
fi

# strip_frontmatter removes the YAML frontmatter (--- ... ---) that Cursor
# requires but plain-markdown tools (Claude Code, Cline, Aider, Antigravity)
# would render as text. Non-frontmatter files pass through unchanged.
strip_frontmatter() {
	local file="$1"
	awk 'NR==1 && $0=="---" { skip=1; next }
	     skip && $0=="---" { skip=0; next }
	     skip { next }
	     !body && $0=="" { next }
	     { body=1; print }' "$file"
}

# install_for_agent <name>
install_for_agent() {
	local name="$1"
	local rel label strip_marker
	case "$name" in
		cursor)      rel=".cursor/rules/vfs.mdc";            label="Cursor";       strip_marker=0 ;;
		claude)      rel="CLAUDE.md";                        label="Claude Code"; strip_marker=1 ;;
		antigravity) rel="GEMINI.md";                        label="Antigravity";  strip_marker=1 ;;
		windsurf)    rel=".windsurf/rules/vfs.md";           label="Windsurf";     strip_marker=0 ;;
		cline)       rel=".clinerules";                      label="Cline / Roo";  strip_marker=1 ;;
		continue)    rel=".continue/rules/vfs.md";           label="Continue";     strip_marker=0 ;;
		aider)       rel=".aider.conventions.md";            label="Aider";        strip_marker=1 ;;
		opencode)    rel=".opencode/rules/vfs.mdc";          label="OpenCode";     strip_marker=0 ;;
		*)
			fail "unknown agent '${name}' (run --list for supported tools)"
			;;
	esac

	local dest="$PROJECT/$rel"
	local marker="## vfs: Use AST-based search before grep"

	if [[ $strip_marker -eq 1 ]]; then
		# Plain-markdown tools: append under a heading so the rule is
		# idempotent and survives existing CLAUDE.md / .clinerules content.
		if [[ -f "$dest" ]] && grep -qF "$marker" "$dest"; then
			echo "  ${label}: already installed (${rel}) -- skipping"
			return 0
		fi

		local body
		body="$(strip_frontmatter "$RULE_SOURCE")"

		if [[ $DRY_RUN -eq 1 ]]; then
			if [[ -f "$dest" ]]; then
				echo "  ${label}: would append rule to ${rel}"
			else
				echo "  ${label}: would create ${rel}"
			fi
			return 0
		fi

		if [[ -f "$dest" ]]; then
			printf '\n%s\n\n' "$marker" >>"$dest"
			printf '%s\n' "$body" >>"$dest"
			echo "  ${label}: appended rule to ${rel}"
		else
			{
				printf '%s\n\n' "$marker"
				printf '%s\n' "$body"
			} >"$dest"
			echo "  ${label}: created ${rel}"
		fi
		return 0
	fi

	# Frontmatter tools (Cursor, Windsurf, Continue, OpenCode): copy verbatim.
	if [[ -f "$dest" ]] && [[ $FORCE -eq 0 ]]; then
		if cmp -s "$RULE_SOURCE" "$dest"; then
			echo "  ${label}: already installed (${rel}) -- skipping"
		else
			echo "  ${label}: ${rel} already exists (use --force to overwrite) -- skipping"
		fi
		return 0
	fi

	if [[ $DRY_RUN -eq 1 ]]; then
		echo "  ${label}: would install rule to ${rel}"
		return 0
	fi

	mkdir -p "$(dirname "$dest")"
	cp "$RULE_SOURCE" "$dest"
	echo "  ${label}: installed to ${rel}"
}

detect_agents() {
	local found=()
	[[ -d "$PROJECT/.cursor" ]]   && found+=(cursor)
	if [[ -f "$PROJECT/CLAUDE.md" ]] || [[ -f "$PROJECT/AGENTS.md" ]]; then
		found+=(claude)
	fi
	[[ -f "$PROJECT/GEMINI.md" ]] && found+=(antigravity)
	[[ -d "$PROJECT/.windsurf" ]] && found+=(windsurf)
	[[ -f "$PROJECT/.clinerules" ]] && found+=(cline)
	[[ -d "$PROJECT/.continue" ]] && found+=(continue)
	[[ -f "$PROJECT/.aider.conventions.md" ]] && found+=(aider)
	[[ -d "$PROJECT/.opencode" ]] && found+=(opencode)
	printf '%s\n' "${found[@]}"
}

if [[ ${#AGENTS[@]} -eq 0 ]]; then
	AGENTS=()
	while IFS= read -r a; do
		[[ -n "$a" ]] && AGENTS+=("$a")
	done < <(detect_agents)

	if [[ ${#AGENTS[@]} -eq 0 ]]; then
		echo "note: no AI-tool config detected in ${PROJECT}."
		echo "      Installing for the most common tools (cursor, claude, opencode)."
		echo "      Pick one with --agent <name> (see --list)."
		echo ""
		AGENTS=(cursor claude opencode)
	fi
fi

echo "Installing vfs agent rules into ${PROJECT}:"
for a in "${AGENTS[@]}"; do
	install_for_agent "$a"
done

echo ""
echo "Done. Next steps:"
echo "  1. Install vfs itself if you haven't already (MCP server or CLI binary)."
echo "  2. Restart / reload your AI tool so it picks up the new rule."
echo "  3. The rule makes agents call 'vfs <path> -f <name>' before grep."