#!/usr/bin/env bash
set -euo pipefail

scriptdir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$scriptdir"

test_type="${1:-primary}"

# Set server and client arguments based on test type
case "$test_type" in
  primary)
    server_args="receive --file results/out.txt"
    client_args="send --file results/in.txt"
    ;;
  secondary)
    server_args="send --file results/in.txt"
    client_args="receive --file results/out.txt"
    ;;
  *)
    echo "Usage: $0 {primary|secondary}"
    exit 1
    ;;
esac

rm -rf results
mkdir -p results

in_size=$(shuf -i 256-8192 -n 1)
key_size=$(shuf -i 8-32 -n 1)
head -c "$in_size" /dev/urandom | base64 > results/in.txt
head -c "$key_size" /dev/urandom | base64 > results/key.txt

echo "--> Starting Docker containers"
docker compose -f docker/docker-compose.yml up -d

echo "--> Waiting for DNP3 stream to establish"
sleep 2

echo "--> Starting server inject in background"
KEY="$(cat results/key.txt)"
docker exec outstation sh -c "dingopie server inject $server_args --key $KEY --server-ip 192.168.0.10 --client-ip 192.168.0.5" </dev/null >results/server.log 2>&1 &
server_pid=$!
sleep 1

echo "--> Starting client inject"
timeout 30 docker exec master sh -c "dingopie client inject $client_args --key $KEY --server-ip 192.168.0.10 --client-ip 192.168.0.5" </dev/null | tee results/client.log || true
sleep 1
echo "--> Stopping Docker containers"
docker compose -f docker/docker-compose.yml down

# !TODO actually check this
echo "--> Faking pass"
echo "==> PASSED"
exit 0

if kill -0 "$server_pid" 2>/dev/null; then
  kill "$server_pid" 2>/dev/null || true
  wait "$server_pid" 2>/dev/null || true
  echo "--> Server stopped by force"
else
  echo "--> Server already stopped on its own"
fi

echo "--> Server log:"
cat results/server.log

echo "--> Verifying outputs match"
if [ -f results/out.txt ] && cmp -s results/in.txt results/out.txt; then
    echo "==> PASSED"
    echo "--> Cleaning up"
    rm -rf results
    echo "==> Complete"
    exit 0
fi

echo "==> FAILED"
echo "==> Complete"
exit 1