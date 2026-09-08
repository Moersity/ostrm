#!/bin/sh
# Never remove user data. Stop a running service before package replacement/removal.
if command -v systemctl >/dev/null 2>&1; then systemctl stop ostrm || true; fi
