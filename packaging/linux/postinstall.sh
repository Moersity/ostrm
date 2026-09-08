#!/bin/sh
set -eu
getent group ostrm >/dev/null || groupadd --system ostrm
id ostrm >/dev/null 2>&1 || useradd --system --gid ostrm --home-dir /var/lib/ostrm --shell /usr/sbin/nologin ostrm
install -d -m 750 -o ostrm -g ostrm /var/lib/ostrm
if command -v systemctl >/dev/null 2>&1; then systemctl daemon-reload || true; fi

if [ -f /var/lib/ostrm/.restart-after-upgrade ]; then
  rm -f /var/lib/ostrm/.restart-after-upgrade
  systemctl start ostrm || true
fi
