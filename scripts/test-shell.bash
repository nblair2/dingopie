#!/usr/bin/env bash
set -euo pipefail

EXECUTABLE=${EXECUTABLE:-"dist/dingopie_linux_amd64_v1/dingopie"}
test_type="${1:-primary}"

rm -rf test
mkdir -p test

in_size=$(shuf -i 128-1024 -n 1)
key_size=$(shuf -i 8-32 -n 1)
head -c "$in_size" /dev/urandom | base64 > test/in.txt
head -c "$key_size" /dev/urandom | base64 > test/key.txt
KEY="$(cat test/key.txt)"

echo "--> Starting server in background"
case "$test_type" in
  primary)
    "$EXECUTABLE" server direct shell --key "$KEY" > test/server.log 2>&1 &
    ;;
  secondary)
    echo "echo 'hello dingopie'; ls test; cat test/in.txt; exit;" | timeout 10s "$EXECUTABLE" server direct connect --key "$KEY" > test/server.log 2>&1 &
    ;;
  *)
    echo "Usage: $0 {primary|secondary}"
    exit 1
    ;;
esac
server_pid=$!
sleep 1

echo "--> Starting client"
case "$test_type" in
  primary)
    echo "echo 'hello dingopie'; ls test; cat test/in.txt; exit;" | timeout 10s "$EXECUTABLE" client direct connect --key "$KEY" --server-ip 127.0.0.1 2>&1 | tee test/client.log
    ;;
  secondary)
    "$EXECUTABLE" client direct shell --key "$KEY" --server-ip 127.0.0.1 2>&1 | tee test/client.log
    ;;
esac
sleep 1

if kill -0 "$server_pid" 2>/dev/null; then
  kill "$server_pid" 2>/dev/null && echo "--> Server stopped by force (unexpected)" || true
else
  echo "--> Server already stopped on its own (expected)"
fi

echo "--> Server log:"
cat test/server.log

echo "--> Verifying output"
case "$test_type" in
  primary)
    log_file="test/client.log"
    ;;
  secondary)
    log_file="test/server.log"
    ;;
esac
if [ -f "$log_file" ] && grep -F -q -f test/in.txt "$log_file"; then
    echo "==> PASSED"
    echo "--> Cleaning up"
    rm -rf test
    echo "==> Complete"
    exit 0
fi

echo "==> FAILED"
echo "==> Complete"
exit 1
