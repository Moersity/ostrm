#!/bin/bash
set -euo pipefail
version="$1"
arch="$2"
export OSTRM_SMOKE_DATA="$(mktemp -d)"
if [[ "$OSTYPE" == darwin* ]]; then
  sudo installer -pkg "dist/ostrm_${version}_darwin_${arch}.pkg" -target /
  node tests/packaging/smoke.mjs /Applications/OStrm.app/Contents/MacOS/ostrm
  sudo installer -pkg "dist/ostrm_${version}_darwin_${arch}.pkg" -target /
  node tests/packaging/smoke.mjs /Applications/OStrm.app/Contents/MacOS/ostrm
  sudo rm -rf /Applications/OStrm.app
  sudo pkgutil --forget io.github.moersity.ostrm
else
  sudo dpkg -i "dist/ostrm_${version}_linux_${arch}.deb"
  node tests/packaging/smoke.mjs /usr/bin/ostrm
  sudo systemctl start ostrm
  for i in {1..30}; do if curl --fail --silent http://127.0.0.1:3111/health >/dev/null; then break; fi; sleep 1; done
  curl --fail http://127.0.0.1:3111/health
  sudo systemctl stop ostrm
  sudo dpkg -i "dist/ostrm_${version}_linux_${arch}.deb"
  node tests/packaging/smoke.mjs /usr/bin/ostrm
  sudo dpkg -r ostrm
  test -d /var/lib/ostrm
  # Exercise RPM payload and scripts on the native architecture; dependency
  # resolution is intentionally excluded because this runner uses Debian.
  sudo rpm -i --nodeps "dist/ostrm_${version}_linux_${arch}.rpm"
  node tests/packaging/smoke.mjs /usr/bin/ostrm
  sudo rpm -U --replacepkgs --nodeps "dist/ostrm_${version}_linux_${arch}.rpm"
  node tests/packaging/smoke.mjs /usr/bin/ostrm
  sudo rpm -e --nodeps ostrm
  test -d /var/lib/ostrm
fi

test -f "$OSTRM_SMOKE_DATA/ostrm.db"
