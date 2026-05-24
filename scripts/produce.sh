#!/usr/bin/env bash
# Publish N synthetic search events to Kafka via kafka-console-producer.
# Usage: ./scripts/produce.sh [count] [unique-queries]
#   count          — total events to publish (default 10000)
#   unique-queries — distinct query strings to cycle through (default 50)
#
# Requires `docker compose up` to be running.

set -euo pipefail

COUNT="${1:-10000}"
UNIQUE="${2:-50}"
TOPIC="${KAFKA_TOPIC:-search.events}"

queries=(
  "iphone 15" "samsung galaxy" "sneakers" "macbook pro" "headphones"
  "dress" "running shoes" "laptop" "monitor" "kettle"
  "winter coat" "wireless mouse" "gaming chair" "playstation 5" "xbox"
  "smart watch" "tablet" "camera" "drone" "vacuum cleaner"
  "coffee machine" "blender" "air fryer" "tv 55" "soundbar"
  "perfume" "backpack" "jeans" "t-shirt" "jacket"
  "ssd 1tb" "ram 32gb" "graphics card" "router" "printer"
  "book" "kindle" "ipad" "earbuds" "speaker"
  "bicycle" "treadmill" "yoga mat" "dumbbells" "protein"
  "skincare" "makeup" "hair dryer" "razor" "toothbrush"
)

if (( UNIQUE > ${#queries[@]} )); then
  UNIQUE=${#queries[@]}
fi

echo "Producing $COUNT events ($UNIQUE unique queries) to '$TOPIC'..."

ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

# Build payloads locally, pipe in one shot — much faster than one exec per event.
{
  for ((i = 0; i < COUNT; i++)); do
    q="${queries[$((RANDOM % UNIQUE))]}"
    printf '{"query":"%s","user_id":"u%d","request_id":"r%d","timestamp":"%s","source":"loadgen"}\n' \
      "$q" "$i" "$i" "$ts"
  done
} | docker compose exec -T kafka \
    /opt/kafka/bin/kafka-console-producer.sh --bootstrap-server kafka:9092 --topic "$TOPIC" >/dev/null

echo "Done."
