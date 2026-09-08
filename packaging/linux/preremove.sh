#!/bin/sh
# Preserve activation state through upgrades; removal retains user data.
if command -v systemctl >/dev/null 2>&1; then
  if systemctl is-active --quiet ostrm; then touch /var/lib/ostrm/.restart-after-upgrade; fi
  systemctl stop ostrm || true
fi
