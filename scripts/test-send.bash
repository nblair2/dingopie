#!/usr/bin/env bash
set -euo pipefail

EXECUTABLE=${EXECUTABLE:-"dist/dingopie_linux_amd64_v1/dingopie"}
test_type="${1:-primary}"

# Set server and client arguments based on test type
case "$test_type" in
  primary)
    server_args="receive --file test/out.txt"
    client_args="send --file test/in.txt --points $(shuf -i 4-48 -n 1)"
    ;;
  secondary)
    server_args="send --file test/in.txt --points $(shuf -i 4-60 -n 1)"
    client_args="receive --file test/out.txt"
    ;;
  *)
    echo "Usage: $0 {primary|secondary}"
    exit 1
    ;;
esac

rm -rf test
mkdir -p test

in_size=$(shuf -i 256-8192 -n 1)
key_size=$(shuf -i 8-32 -n 1)
head -c "$in_size" /dev/urandom | base64 > test/in.txt
head -c "$key_size" /dev/urandom | base64 > test/key.txt

echo "--> Starting server in background"
KEY="$(cat test/key.txt)"
"$EXECUTABLE" server direct $server_args --key "$KEY" > test/server.log 2>&1 &
server_pid=$!
sleep 1

echo "--> Starting client"
wait_ms=$(shuf -i 10-500 -n 1)
"$EXECUTABLE" client direct $client_args --key "$KEY" --server-ip 127.0.0.1 --wait "${wait_ms}ms" | tee test/client.log
sleep 1

if kill -0 "$server_pid" 2>/dev/null; then
  kill "$server_pid" 2>/dev/null && echo "--> Server stopped by force (unexpected)" || true
else
  echo "--> Server already stopped on its own (expected)"
fi

echo "--> Server log:"
cat test/server.log

echo "--> Verifying outputs match"
if [ -f test/out.txt ] && cmp -s test/in.txt test/out.txt; then
    echo "==> PASSED"
    echo "--> Cleaning up"
    rm -rf test
    echo "==> Complete"
    exit 0
fi

echo "==> FAILED"
echo "==> Complete"
exit 1