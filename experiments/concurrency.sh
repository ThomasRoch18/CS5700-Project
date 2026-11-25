#!/bin/bash

CLIENTS=${1:-1}
OPERATIONS=20
TEST_FILE="testfiles/benchmark.bin"
RUN_ID=$(date +%Y%m%d_%H%M%S)_$$
RESULTS_DIR="experiments/results/$RUN_ID"
TEST_DIR="testfiles/generated/$RUN_ID"

mkdir -p "$RESULTS_DIR"
mkdir -p "$TEST_DIR"
mkdir -p bin

go build -o bin/client ./client || exit 1

for ((client=1; client<=CLIENTS; client++)); do
    for ((op=1; op<=OPERATIONS; op++)); do
        filename="${RUN_ID}_client${client}_op${op}.bin"

        cp "$TEST_FILE" "$TEST_DIR/$filename" || exit 1
    done
done

run_client() {
    local client_id=$1

    {
        echo "AUTH|user|password"

        for ((op=1; op<=OPERATIONS; op++)); do
            filename="${RUN_ID}_client${client_id}_op${op}.bin"

            echo "FILE|UPLOAD|$TEST_DIR/$filename"
        done

        echo "EXIT"

    } | ./bin/client > "$RESULTS_DIR/client${client_id}.log" 2>&1
}

echo "Starting experiment with $CLIENTS clients..."

for ((client=1; client<=CLIENTS; client++)); do
    run_client "$client" &
done

wait

echo "All clients finished."

TOTAL_OPERATIONS=$((CLIENTS * OPERATIONS))

SUCCESSFUL=$(grep -hF "ACK|UPLOAD" \
    "$RESULTS_DIR"/*.log | wc -l)

MEAN=$(awk '/^Response time:/ {
    sum += $3
    count++
}
END {
    if (count > 0)
        printf "%.3f", sum / count
    else
        printf "N/A"
}' "$RESULTS_DIR"/*.log)

echo "===================================="
echo "Concurrency Experiment Results"
echo "===================================="
echo "Concurrent clients: $CLIENTS"
echo "Operations per client: $OPERATIONS"
echo "Total operations: $TOTAL_OPERATIONS"
echo "Successful operations: $SUCCESSFUL/$TOTAL_OPERATIONS"
echo "Failed operations: $((TOTAL_OPERATIONS - SUCCESSFUL))"
echo "Mean response time (ms): $MEAN"
echo "Results directory: $RESULTS_DIR"
echo "===================================="