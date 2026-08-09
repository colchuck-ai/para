#!/bin/sh
# install.sh installs the para CLI.
#
#   curl -sSf https://raw.githubusercontent.com/colchuck-ai/para/main/scripts/install.sh | sh
#
# To pass flags through a pipe, use `sh -s --`:
#
#   curl -sSf .../install.sh | sh -s -- --dir "$HOME/bin"
#
# It prefers a prebuilt binary from the matching release and falls back to
# building from source, so a Go toolchain (>= 1.24) is needed only when no
# prebuilt asset fits: an unreleased ref, an architecture nothing is published
# for, or --from-source.
#
# Anything that makes the fast path untrustworthy falls back rather than
# proceeding: no downloader, no way to check a SHA-256, no asset for this
# platform. The one thing that does *not* fall back is a checksum that is
# present and wrong — that is a tampered or truncated download, and building
# from source instead would hide it.
set -eu

MODULE="github.com/colchuck-ai/para/cmd/para"
REPO_URL="https://github.com/colchuck-ai/para"
REF="main"
# REF_GIVEN tracks whether --ref was passed, which is not the same question as
# whether REF is "main". The default is "main" because that is the source
# fallback's ref, but an explicit `--ref main` means "build main" — and folding
# the two made it silently install the newest release instead, which is the one
# thing the flag's own help says it does not do.
REF_GIVEN=0
DIR=""
REPO_DIR=""
BASE_URL=""
DRY_RUN=0
UNINSTALL=0
FROM_SOURCE=0
MIN_GO_MINOR=24

usage() {
	cat <<'EOF'
Usage: install.sh [options]

Options:
  --ref <ref>       Release tag (e.g. v1.2.3) to install, or a git ref to
                     build from when no release matches. Default: the latest
                     release, falling back to the main branch.
  --dir <dir>       Install directory. Default: $HOME/.local/bin.
  --from-source     Skip the prebuilt binary and build from source. Requires a
                     Go toolchain.
  --repo-dir <dir>  Build from a local checkout instead of fetching the
                     module remotely (used for local development and tests).
  --base-url <url>  Where releases are fetched from. Default: the para
                     repository on GitHub.
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

# need_value fails with the flag's name rather than letting `set -u` kill the
# script with "$2: unbound variable", which names nothing the caller typed.
need_value() {
	if [ "$2" -lt 2 ]; then
		fail "$1 needs a value (see --help)"
	fi
}

while [ $# -gt 0 ]; do
	case "$1" in
	--ref)
		need_value "$1" $#
		REF="$2"
		REF_GIVEN=1
		shift 2
		;;
	--dir)
		need_value "$1" $#
		DIR="$2"
		shift 2
		;;
	--repo-dir)
		need_value "$1" $#
		REPO_DIR="$2"
		shift 2
		;;
	--base-url)
		need_value "$1" $#
		BASE_URL="$2"
		shift 2
		;;
	--from-source)
		FROM_SOURCE=1
		shift
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
if [ -z "$BASE_URL" ]; then
	BASE_URL="$REPO_URL"
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

mkdir_target() {
	if [ "$DRY_RUN" -eq 1 ]; then
		return 0
	fi
	mkdir -p "$DIR"
}

# asset_name is the archive published for this machine, or empty where nothing
# is. The names carry no version — that is what lets the "latest" URL below be
# constructed without first asking what the latest version is.
asset_name() {
	os=$(uname -s | tr '[:upper:]' '[:lower:]')
	arch=$(uname -m)
	case "$os" in
	linux | darwin) ;;
	*) return 0 ;;
	esac
	case "$arch" in
	x86_64 | amd64) arch=amd64 ;;
	arm64 | aarch64) arch=arm64 ;;
	*) return 0 ;;
	esac
	printf 'para_%s_%s.tar.gz' "$os" "$arch"
}

# asset_base is the directory the release's files live under: the latest
# release when no tag was asked for, and that tag's release when one was.
#
# "No tag was asked for" is REF_GIVEN, not REF = "main": an explicit
# `--ref main` names a git ref to build, and answering it with the newest
# release would install something the user did not ask for.
asset_base() {
	if [ "$REF_GIVEN" -eq 0 ]; then
		printf '%s/releases/latest/download' "$BASE_URL"
	else
		printf '%s/releases/download/%s' "$BASE_URL" "$REF"
	fi
}

download() {
	if command -v curl >/dev/null 2>&1; then
		curl -sSfL -o "$2" "$1" 2>/dev/null
	elif command -v wget >/dev/null 2>&1; then
		wget -q -O "$2" "$1"
	else
		return 1
	fi
}

have_downloader() {
	command -v curl >/dev/null 2>&1 || command -v wget >/dev/null 2>&1
}

# sha256_of prints the SHA-256 of a file using whichever of the three usual
# tools is present, and fails if none is.
sha256_of() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d' ' -f1
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | cut -d' ' -f1
	elif command -v openssl >/dev/null 2>&1; then
		openssl dgst -sha256 "$1" | sed 's/.*= *//'
	else
		return 1
	fi
}

# install_prebuilt downloads, verifies, and installs the release archive for
# this machine. It returns non-zero when the fast path does not apply, and
# exits outright when it applies and fails a check.
install_prebuilt() {
	asset=$(asset_name)
	if [ -z "$asset" ]; then
		log "no prebuilt binary for $(uname -s)/$(uname -m)"
		return 1
	fi
	if ! have_downloader; then
		log "neither curl nor wget is available"
		return 1
	fi
	if ! sha256_of /dev/null >/dev/null 2>&1; then
		log "no sha256 tool available to verify a download"
		return 1
	fi

	base=$(asset_base)
	if [ "$DRY_RUN" -eq 1 ]; then
		log "would download $base/$asset into $DIR"
		warn_if_off_path
		exit 0
	fi

	work=$(mktemp -d)
	# The trap is set and cleared around this function alone, so a later
	# source build does not run it. INT and TERM re-raise as an exit rather
	# than only cleaning up: a trap that returns would let an interrupted
	# download fall through to the next line.
	trap 'rm -rf "$work"' EXIT
	trap 'rm -rf "$work"; exit 130' INT
	trap 'rm -rf "$work"; exit 143' TERM

	if ! download "$base/$asset" "$work/$asset"; then
		log "no prebuilt binary at $base/$asset"
		rm -rf "$work"
		trap - EXIT INT TERM
		return 1
	fi
	if ! download "$base/checksums.txt" "$work/checksums.txt"; then
		log "no checksums.txt beside $asset"
		rm -rf "$work"
		trap - EXIT INT TERM
		return 1
	fi

	# The name is escaped because grep takes a pattern and the name has dots
	# in it: unescaped, `para_linux_amd64.tar.gz` also matches a line naming
	# `para_linux_amd64Xtar.gz`, and the wrong line's hash is the wrong answer.
	pattern=$(printf '%s' "$asset" | sed 's/[.[\*^$]/\\&/g')
	want=$(grep "  *$pattern\$" "$work/checksums.txt" | cut -d' ' -f1 || true)
	if [ -z "$want" ]; then
		fail "checksums.txt does not list $asset"
	fi
	got=$(sha256_of "$work/$asset")
	if [ "$want" != "$got" ]; then
		fail "checksum mismatch for $asset: expected $want, got $got"
	fi

	tar -xzf "$work/$asset" -C "$work" para || fail "could not unpack $asset"
	mkdir_target
	# Copied rather than moved, because the temp directory may be on another
	# filesystem — and copied to a temp name inside $DIR first, so that the
	# final step is a rename and a para running from $BIN is replaced rather
	# than truncated under itself.
	cp "$work/para" "$BIN.new" || fail "could not write to $DIR"
	chmod 0755 "$BIN.new"
	mv "$BIN.new" "$BIN" || {
		rm -f "$BIN.new"
		fail "could not install $BIN"
	}

	rm -rf "$work"
	trap - EXIT INT TERM
	log "installed $BIN from $base/$asset"
	warn_if_off_path
	return 0
}

install_from_source() {
	# The dry run reports and stops before every check, the same way the
	# prebuilt path's does: it writes nothing and runs nothing, so requiring a
	# toolchain it will not invoke would make --dry-run fail where the thing it
	# is describing is the only thing that can.
	if [ "$DRY_RUN" -eq 1 ]; then
		if [ -n "$REPO_DIR" ]; then
			log "would build $MODULE from local checkout $REPO_DIR into $DIR"
		else
			log "would install $MODULE@$REF into $DIR"
		fi
		warn_if_off_path
		exit 0
	fi

	require_go
	mkdir_target
	if [ -n "$REPO_DIR" ]; then
		(cd "$REPO_DIR" && GOBIN="$DIR" go install ./cmd/para)
	else
		GOBIN="$DIR" go install "$MODULE@$REF"
	fi
	log "installed $BIN (built from source)"
	warn_if_off_path
}

if [ "$FROM_SOURCE" -eq 0 ] && [ -z "$REPO_DIR" ]; then
	if install_prebuilt; then
		exit 0
	fi
	log "building from source instead"
fi

install_from_source
