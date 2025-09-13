#!/bin/bash

# This script creates a mock future leases JSON file based on the latest leases*.json file in the mocking_data directory.
# It advances the timestamps for each namespace, simulating future heartbeats for demo purposes.

# Directory containing the leases*.json files
DATA_DIR="mocking_data"

# Find the latest leases*.json file (excluding manifest) in the mocking_data directory
latest=$(ls "$DATA_DIR"/leases*.json | grep -v manifest | sort | tail -n 1)
echo "Latest file: $latest"

# Set output filename with current timestamp (in mocking_data directory)
now=$(date +"%Y%m%dT%H%M%S")
out="$DATA_DIR/leases${now}.json"

# Number of heartbeats to generate for each namespace
NUM=22
# Interval between heartbeats in milliseconds
INTERVAL=10000

# Use jq to process the latest file and generate new future data
jq --argjson num "$NUM" --argjson interval "$INTERVAL" '
  to_entries
  | map({
      key: .key, # namespace
      value: (
        .value as $points
        | ($points[-1].y) as $last # last timestamp for this namespace
        | [range(0; $num)]
        | map({
            x: .,
            y: ($last + $interval * (. + 1)) # advance timestamp for each heartbeat
          })
      )
    })
  | from_entries
' "$latest" > "$out"

echo "Mock future leases JSON written to $out"