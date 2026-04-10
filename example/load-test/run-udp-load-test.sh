#!/bin/sh
set -e

DURATION=${DURATION:-5}
PARALLEL=${PARALLEL:-1}
BITRATE=${BITRATE:-100M}
TARGET_HOST=host.docker.internal

echo "==> UDP load test: ${PARALLEL} parallel stream(s) at ${BITRATE} for ${DURATION}s"
echo "    connecting to lazy-tcp-proxy on ${TARGET_HOST}:5202 -> iperf3-udp:5201"
echo "    (proxy handles both TCP control channel and UDP data on port 5202)"
echo ""

docker run --rm --network host networkstatic/iperf3 \
  -c "$TARGET_HOST" -p 5202 -u -b "$BITRATE" -t "$DURATION" -P "$PARALLEL"
