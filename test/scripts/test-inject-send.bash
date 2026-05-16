#!/usr/bin/env bash
set -euo pipefail

scriptdir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$scriptdir"

cleanup() {
  docker compose -f docker/docker-compose.yml down 2>/dev/null || true
}
trap cleanup EXIT

EXECUTABLE=${EXECUTABLE:-"../dist/dingopie_linux_amd64_v1/dingopie"}
test_type="${1:-primary}"

# Receiver always starts first so it's intercepting when the sender begins.
case "$test_type" in
  primary)
    recv_role="server"
    recv_container="outstation"
    recv_args="receive --file results/out.txt"
    send_role="client"
    send_container="master"
    send_args="send --file results/in.txt"
    ;;
  secondary)
    recv_role="client"
    recv_container="master"
    recv_args="receive --file results/out.txt"
    send_role="server"
    send_container="outstation"
    send_args="send --file results/in.txt"
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

KEY="$(cat results/key.txt)"

echo "--> Starting receiver ($recv_role inject) in background"
docker exec "$recv_container" sh -c \
  "dingopie $recv_role inject $recv_args --key $KEY --server-ip 192.168.0.10 --client-ip 192.168.0.5" > "results/$recv_role.log" 2>&1 &
recv_pid=$!
sleep 1

echo "--> Starting sender ($send_role inject)"
docker exec "$send_container" sh -c \
  "dingopie $send_role inject $send_args --key $KEY --server-ip 192.168.0.10 --client-ip 192.168.0.5" | tee "results/$send_role.log"
sleep 1

if kill -0 "$recv_pid" 2>/dev/null; then
  echo "--> Receiver still running, will be stopped by docker compose down (unexpected)"
else
  echo "--> Receiver already stopped on its own (expected)"
fi

echo "--> Receiver log:"
cat "results/$recv_role.log"
docker exec outstation chown -R "$(id -u):$(id -g)" /usr/local/bin/results 2>/dev/null || true

echo "--> Stopping Docker containers"
cleanup

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
