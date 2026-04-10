#!/bin/sh
set -e

DURATION=${DURATION:-30}
PARALLEL=${PARALLEL:-10}

echo "==> TCP load test: ${PARALLEL} parallel stream(s) for ${DURATION}s"
echo "    connecting to lazy-tcp-proxy on localhost:5201 -> iperf3-tcp:5201"
echo ""

docker run --rm --network host networkstatic/iperf3 \
  -c localhost -p 5201 -t "$DURATION" -P "$PARALLEL"
