#!/bin/sh
# install.sh installs the para CLI.
#
#   curl -sSf https://raw.githubusercontent.com/colchuck-ai/para/main/scripts/install.sh | sh
#
# To pass flags through a pipe, use `sh -s --`:
#
#   curl -sSf .../install.sh | sh -s -- --dir "$HOME/bin"
#
# Requires a Go toolchain (>= 1.24): this script always builds para from
# source. A future release adds a prebuilt-binary fast path; until then, the
# Go toolchain requirement is unconditional.
set -eu

MODULE="github.com/colchuck-ai/para/cmd/para"
REF="main"
DIR=""
REPO_DIR=""
DRY_RUN=0
UNINSTALL=0
MIN_GO_MINOR=24

usage() {
	cat <<'EOF'
Usage: install.sh [options]

Options:
  --ref <ref>       Git ref (tag, branch, or commit) to install. Default: main.
                     Ignored when --repo-dir is given.
  --dir <dir>       Install directory. Default: $HOME/.local/bin.
  --repo-dir <dir>  Build from a local checkout instead of fetching the
                     module remotely (used for local development and tests).
  --uninstall       Remove the installed para binary from --dir.
  --dry-run         Print what would happen without doing it.
  -h, --help        Show this help.
EOF
}

log() {
	printf '%s\n' "$*"
}

fail() {
	printf 'install.sh: %s\n' "$*" >&2
	exit 1
}

while [ $# -gt 0 ]; do
	case "$1" in
	--ref)
		REF="$2"
		shift 2
		;;
	--dir)
		DIR="$2"
		shift 2
		;;
	--repo-dir)
		REPO_DIR="$2"
		shift 2
		;;
	--uninstall)
		UNINSTALL=1
		shift
		;;
	--dry-run)
		DRY_RUN=1
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		fail "unknown option: $1 (see --help)"
		;;
	esac
done

if [ -z "$DIR" ]; then
	DIR="$HOME/.local/bin"
fi

BIN="$DIR/para"

path_contains() {
	case ":$PATH:" in
	*":$1:"*) return 0 ;;
	*) return 1 ;;
	esac
}

warn_if_off_path() {
	if ! path_contains "$DIR"; then
		log "warning: $DIR is not on your PATH. Add it, e.g.:"
		log "  export PATH=\"$DIR:\$PATH\""
	fi
}

if [ "$UNINSTALL" -eq 1 ]; then
	if [ "$DRY_RUN" -eq 1 ]; then
		log "would remove $BIN"
		exit 0
	fi
	rm -f "$BIN"
	log "removed $BIN"
	exit 0
fi

require_go() {
	if ! command -v go >/dev/null 2>&1; then
		fail "Go toolchain not found on PATH. Install Go >= 1.$MIN_GO_MINOR from https://go.dev/dl/ and try again."
	fi

	go_version_line=$(go version)
	go_version=$(printf '%s\n' "$go_version_line" | sed -n 's/^go version go\([0-9][0-9]*\.[0-9][0-9]*\).*/\1/p')
	if [ -z "$go_version" ]; then
		fail "could not parse Go version from '$go_version_line'"
	fi

	go_major=${go_version%%.*}
	go_minor=${go_version#*.}
	if [ "$go_major" -lt 1 ] || { [ "$go_major" -eq 1 ] && [ "$go_minor" -lt "$MIN_GO_MINOR" ]; }; then
		fail "$go_version_line is too old; para requires Go >= 1.$MIN_GO_MINOR"
	fi
}

require_go

mkdir_target() {
	if [ "$DRY_RUN" -eq 1 ]; then
		return 0
	fi
	mkdir -p "$DIR"
}

if [ -n "$REPO_DIR" ]; then
	if [ "$DRY_RUN" -eq 1 ]; then
		log "would build $MODULE from local checkout $REPO_DIR into $DIR"
		warn_if_off_path
		exit 0
	fi
	mkdir_target
	(cd "$REPO_DIR" && GOBIN="$DIR" go install ./cmd/para)
else
	if [ "$DRY_RUN" -eq 1 ]; then
		log "would install $MODULE@$REF into $DIR"
		warn_if_off_path
		exit 0
	fi
	mkdir_target
	GOBIN="$DIR" go install "$MODULE@$REF"
fi

log "installed $BIN"
warn_if_off_path
