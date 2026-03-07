#!/usr/bin/env bash
set -euo pipefail

# ─────────────────────────────────────────────────────────────
# vfs release script
#
# Usage:
#   ./scripts/release.sh              # release version from VERSION file
#   ./scripts/release.sh 1.0.5        # release a specific version
#   ./scripts/release.sh --dry-run    # show what would happen, change nothing
# ─────────────────────────────────────────────────────────────

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

DRY_RUN=false
VERSION=""

for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=true ;;
    -h|--help)
      echo "Usage: $0 [version] [--dry-run]"
      echo ""
      echo "  version    Semver to release (e.g. 1.0.5). Defaults to VERSION file."
      echo "  --dry-run  Show what would happen without changing anything."
      echo ""
      echo "Examples:"
      echo "  $0              # release version from VERSION file"
      echo "  $0 1.0.5        # release v1.0.5"
      echo "  $0 --dry-run    # preview the release steps"
      exit 0
      ;;
    *) VERSION="$arg" ;;
  esac
done

info()  { echo -e "${CYAN}[info]${NC}  $*"; }
ok()    { echo -e "${GREEN}[ok]${NC}    $*"; }
warn()  { echo -e "${YELLOW}[warn]${NC}  $*"; }
fail()  { echo -e "${RED}[fail]${NC}  $*"; exit 1; }
run()   {
  if $DRY_RUN; then
    echo -e "${YELLOW}[dry-run]${NC} $*"
  else
    "$@"
  fi
}

# ── Resolve version ──────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

if [ -z "$VERSION" ]; then
  [ -f VERSION ] || fail "No VERSION file found and no version argument given."
  VERSION="$(cat VERSION | tr -d '[:space:]')"
fi

TAG="v${VERSION}"

echo ""
echo "──────────────────────────────────────"
echo "  vfs release ${TAG}"
if $DRY_RUN; then
  echo "  (dry run -- nothing will be changed)"
fi
echo "──────────────────────────────────────"
echo ""

# ── Pre-flight checks ───────────────────────────────────────

info "Running pre-flight checks..."

# 1. Git repo
git rev-parse --is-inside-work-tree >/dev/null 2>&1 || fail "Not inside a git repository."
ok "Inside git repo"

# 2. On main branch
BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [ "$BRANCH" != "main" ]; then
  fail "You are on branch '${BRANCH}'. Switch to 'main' before releasing.\n       Run: git checkout main"
fi
ok "On branch 'main'"

# 3. Working tree clean
if ! git diff --quiet || ! git diff --cached --quiet; then
  fail "Working tree is dirty. Commit or stash your changes first.\n       Run: git status"
fi
if [ -n "$(git ls-files --others --exclude-standard)" ]; then
  warn "Untracked files present (ignored for release, but you may want to check)"
fi
ok "Working tree clean"

# 4. Up to date with remote
git fetch origin main --quiet 2>/dev/null || warn "Could not fetch origin/main (offline?)"
LOCAL="$(git rev-parse HEAD)"
REMOTE="$(git rev-parse origin/main 2>/dev/null || echo "")"
if [ -n "$REMOTE" ] && [ "$LOCAL" != "$REMOTE" ]; then
  AHEAD="$(git rev-list origin/main..HEAD --count)"
  BEHIND="$(git rev-list HEAD..origin/main --count)"
  if [ "$BEHIND" -gt 0 ]; then
    fail "Local is ${BEHIND} commit(s) behind origin/main. Pull first.\n       Run: git pull origin main"
  fi
  if [ "$AHEAD" -gt 0 ]; then
    info "Local is ${AHEAD} commit(s) ahead of origin/main (will be pushed)"
  fi
fi
ok "In sync with origin/main"

# 5. Tag doesn't already exist
if git rev-parse "$TAG" >/dev/null 2>&1; then
  fail "Tag '${TAG}' already exists. Bump the VERSION file or pick a different version."
fi
ok "Tag '${TAG}' is available"

# 6. Go + C compiler available
command -v go >/dev/null 2>&1 || fail "Go is not installed."
ok "Go $(go version | grep -oE 'go[0-9]+\.[0-9]+\.[0-9]+')"

# 7. Build succeeds
info "Building to verify compilation..."
if $DRY_RUN; then
  echo -e "${YELLOW}[dry-run]${NC} go build ./cmd/vfs"
else
  go build -o /dev/null ./cmd/vfs || fail "Build failed. Fix errors before releasing."
fi
ok "Build succeeded"

# 8. Tests pass
info "Running tests..."
if $DRY_RUN; then
  echo -e "${YELLOW}[dry-run]${NC} go test ./..."
else
  go test ./... || fail "Tests failed. Fix them before releasing."
fi
ok "Tests passed"

echo ""

# ── Release ──────────────────────────────────────────────────

info "Tagging ${TAG}..."
run git tag -a "$TAG" -m "Release ${TAG}"
ok "Created tag ${TAG}"

info "Pushing to origin..."
run git push origin main --tags
ok "Pushed main + ${TAG} to origin"

# ── Post-release verification ────────────────────────────────

echo ""
info "Waiting for Go module proxy to index ${TAG}..."

if $DRY_RUN; then
  echo -e "${YELLOW}[dry-run]${NC} Would poll https://proxy.golang.org/... for up to 120s"
else
  MODULE="github.com/!tr!ng!tien/vfs"
  PROXY_URL="https://proxy.golang.org/${MODULE}/@v/${TAG}.info"
  MAX_WAIT=120
  WAITED=0
  INTERVAL=10

  while [ $WAITED -lt $MAX_WAIT ]; do
    HTTP_CODE="$(curl -s -o /dev/null -w '%{http_code}' "$PROXY_URL" 2>/dev/null || echo "000")"
    if [ "$HTTP_CODE" = "200" ]; then
      ok "Go proxy has indexed ${TAG}"
      break
    fi
    sleep $INTERVAL
    WAITED=$((WAITED + INTERVAL))
    info "Still waiting... (${WAITED}s / ${MAX_WAIT}s)"
  done

  if [ $WAITED -ge $MAX_WAIT ]; then
    warn "Go proxy hasn't indexed ${TAG} yet. It usually takes 2-5 minutes."
    warn "Check manually: curl -s ${PROXY_URL}"
    warn "Users can still install via: git clone + go install ./cmd/vfs"
  fi
fi

# ── Summary ──────────────────────────────────────────────────

echo ""
echo "──────────────────────────────────────"
echo -e "  ${GREEN}Released ${TAG}${NC}"
echo "──────────────────────────────────────"
echo ""
echo "  GitHub Actions will now:"
echo "    1. Build binaries for Linux and macOS (amd64 + arm64)"
echo "    2. Create a GitHub Release with downloadable assets"
echo "    3. Check: https://github.com/TrNgTien/vfs/actions"
echo ""
echo "  Users can install with:"
echo ""
echo "    # Download pre-built binary (easiest -- no Go needed)"
echo "    curl -L https://github.com/TrNgTien/vfs/releases/latest/download/vfs-\$(uname -s | tr A-Z a-z)-\$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').tar.gz | tar xz"
echo "    sudo mv vfs /usr/local/bin/"
echo ""
echo "    # Or via Go module proxy"
echo "    go install github.com/TrNgTien/vfs/cmd/vfs@${TAG}"
echo ""
echo "    # Or from source"
echo "    git clone https://github.com/TrNgTien/vfs.git && cd vfs"
echo "    go install ./cmd/vfs"
echo ""
