#!/bin/sh
set -e

DURATION=${DURATION:-30}
PARALLEL=${PARALLEL:-10}
TARGET_HOST=host.docker.internal

echo "==> TCP load test: ${PARALLEL} parallel stream(s) for ${DURATION}s"
echo "    connecting to lazy-tcp-proxy on ${TARGET_HOST}:5201 -> iperf3-tcp:5201"
echo ""

docker run --rm --network host networkstatic/iperf3 \
  -c "$TARGET_HOST" -p 5201 -t "$DURATION" -P "$PARALLEL"
