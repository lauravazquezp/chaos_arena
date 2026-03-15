#!/bin/bash
set -e
docker compose up -d --build
sleep 5

GAME=$(curl -sf -X POST http://localhost:8080/game/start | jq -r .game_id)
echo "Game started: $GAME"

curl -sf -X POST http://localhost:8080/attack \
  -H "Content-Type: application/json" \
  -d '{"service":"service-b","attack":"kill_switch"}'

# Wait for service-b to become unhealthy (reconciler detects the stopped container)
for i in $(seq 1 15); do
  STATUS=$(curl -sf http://localhost:8080/game/state | jq -r '.services["service-b"].status')
  echo "Tick $i: service-b = $STATUS"
  if [ "$STATUS" != "healthy" ]; then break; fi
  sleep 1
done

# Wait for service-b to recover
for i in $(seq 1 30); do
  STATUS=$(curl -sf http://localhost:8080/game/state | jq -r '.services["service-b"].status')
  echo "Recovery tick $i: service-b = $STATUS"
  if [ "$STATUS" = "healthy" ]; then break; fi
  sleep 1
done

REPORT=$(curl -sf -X POST http://localhost:8080/game/stop)
MTTR=$(echo $REPORT | jq '.summary.mean_mttr_ms')
echo "Mean MTTR: $MTTR ms"

[ "$MTTR" -gt 0 ] && echo "SMOKE TEST PASSED" || (echo "SMOKE TEST FAILED" && exit 1)

docker compose down
