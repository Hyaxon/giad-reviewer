#!/bin/sh
# Trusted image helper. Stdin is the host's sanitized tar snapshot, never a command.
set -eu
cd /workspace
tar --no-same-owner --no-same-permissions -xf -
mkdir -p /tmp/build /tmp/cache
export HOME=/tmp TMPDIR=/tmp/build GOCACHE=/tmp/cache
exec "$@"
