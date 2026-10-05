#!/usr/bin/env sh
# Install the `kit` binary from GitHub releases.
#
#   curl -fsSL https://raw.githubusercontent.com/valenciajoel/kit/main/install.sh | sh
#
# Override the destination with INSTALL_DIR, and pin a version with KIT_VERSION.
set -eu

REPO="valenciajoel/kit"
BIN="kit"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

os="$(uname -s)"
arch="$(uname -m)"

case "$os" in
	Linux) goos="linux" ;;
	Darwin) goos="darwin" ;;
	*)
		echo "kit: unsupported OS: $os (Windows: download kit-windows-amd64.exe from the releases page)" >&2
		exit 1
		;;
esac

case "$arch" in
	x86_64 | amd64) goarch="amd64" ;;
	arm64 | aarch64) goarch="arm64" ;;
	*)
		echo "kit: unsupported architecture: $arch" >&2
		exit 1
		;;
esac

if [ -n "${KIT_VERSION:-}" ]; then
	base="https://github.com/${REPO}/releases/download/${KIT_VERSION}"
else
	base="https://github.com/${REPO}/releases/latest/download"
fi
url="${base}/${BIN}-${goos}-${goarch}"

echo "kit: downloading ${url}"
mkdir -p "$INSTALL_DIR"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

if ! curl -fsSL "$url" -o "$tmp"; then
	echo "kit: download failed. Check that a release exists for ${goos}/${goarch}." >&2
	exit 1
fi

chmod +x "$tmp"
mv "$tmp" "${INSTALL_DIR}/${BIN}"
trap - EXIT

echo "kit: installed to ${INSTALL_DIR}/${BIN}"
case ":${PATH}:" in
	*":${INSTALL_DIR}:"*) ;;
	*) echo "kit: add ${INSTALL_DIR} to your PATH to run '${BIN}'." ;;
esac
