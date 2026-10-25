#!/bin/sh
# squad-up installer: downloads a release, checks its checksum and puts `squad`
# and `squad-up` in a folder on your PATH.
#
#   curl -fsSL https://raw.githubusercontent.com/EdrenaVosh/squad-up/main/scripts/install.sh | sh
#
# Settings, all optional:
#   SQUAD_UP_VERSION       a version such as 0.1.0 (default: the latest release)
#   SQUAD_UP_INSTALL_DIR   where to put the programs (default: ~/.local/bin)
#   SQUAD_UP_INSTALL_FROM  a release archive on this machine, to install from it
#                         instead of downloading; checksums.txt must lie beside it
set -eu

REPO="EdrenaVosh/squad-up"
DIR="${SQUAD_UP_INSTALL_DIR:-$HOME/.local/bin}"

say() { printf '%s\n' "$*"; }
fail() { printf 'squad-up install: %s\n' "$*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

os() {
  case "$(uname -s)" in
    Darwin) echo darwin ;;
    Linux) echo linux ;;
    *) fail "this script supports macOS and Linux; on Windows download the .zip from https://github.com/$REPO/releases" ;;
  esac
}

arch() {
  case "$(uname -m)" in
    x86_64 | amd64) echo amd64 ;;
    arm64 | aarch64) echo arm64 ;;
    *) fail "no release for the processor $(uname -m)" ;;
  esac
}

sha256() {
  if have sha256sum; then sha256sum "$1" | cut -d ' ' -f 1
  elif have shasum; then shasum -a 256 "$1" | cut -d ' ' -f 1
  else fail "neither sha256sum nor shasum is installed, so the download cannot be checked"
  fi
}

fetch() { # url file
  if have curl; then curl -fsSL "$1" -o "$2"
  elif have wget; then wget -q "$1" -O "$2"
  else fail "curl or wget is needed to download"
  fi
}

latest_version() {
  have curl || fail "curl is needed to find the latest release; set SQUAD_UP_VERSION to skip the lookup"
  url=$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest") || fail "could not look up the latest release"
  tag=${url##*/}
  # With no release GitHub redirects to .../releases, so the "tag" is a word.
  case "$tag" in
    v[0-9]*) echo "${tag#v}" ;;
    *) fail "there is no release yet" ;;
  esac
}

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

if [ -n "${SQUAD_UP_INSTALL_FROM:-}" ]; then
  archive=$SQUAD_UP_INSTALL_FROM
  [ -f "$archive" ] || fail "$archive does not exist"
  sums="$(dirname "$archive")/checksums.txt"
  [ -f "$sums" ] || fail "checksums.txt is not beside $archive, so it cannot be checked"
  say "Installing from $archive"
else
  os=$(os)
  arch=$(arch)
  version=${SQUAD_UP_VERSION:-$(latest_version)}
  version=${version#v}
  name="squad-up_${version}_${os}_${arch}.tar.gz"
  base="https://github.com/$REPO/releases/download/v${version}"
  say "Downloading squad-up $version for $os/$arch"
  fetch "$base/$name" "$TMP/$name" || fail "could not download $base/$name"
  fetch "$base/checksums.txt" "$TMP/checksums.txt" || fail "could not download the checksums"
  archive=$TMP/$name
  sums=$TMP/checksums.txt
fi

# The checksum is taken from checksums.txt by the name of the archive.
want=$(grep " $(basename "$archive")\$" "$sums" | cut -d ' ' -f 1 | head -n 1)
[ -n "$want" ] || fail "$(basename "$archive") is not listed in checksums.txt"
got=$(sha256 "$archive")
[ "$want" = "$got" ] || fail "the checksum of $(basename "$archive") does not match: expected $want, got $got"
say "Checksum ok"

tar -xzf "$archive" -C "$TMP" || fail "could not unpack $archive"
mkdir -p "$DIR"
for program in squad squad-up; do
  [ -f "$TMP/$program" ] || fail "$program is not in the archive"
  cp "$TMP/$program" "$DIR/$program.new"
  chmod 0755 "$DIR/$program.new"
  mv -f "$DIR/$program.new" "$DIR/$program"
done

say "Installed squad and squad-up in $DIR"
case ":$PATH:" in
  *":$DIR:"*) ;;
  *) say "Note: $DIR is not on your PATH. Add this to your shell profile:  export PATH=\"$DIR:\$PATH\"" ;;
esac
say "Next: run  squad  to see what it found in your Claude Code setup (nothing is changed)."
