#!/bin/bash
echo "Setting up env variables..."
eval "$(docker-machine env default)"

# Inserts $SIPHON_ENV and other placeholders
COMPOSE_FILE="compose.yml"
TMP_COMPOSE_FILE=".tmp-compose.yml"
rm -f $TMP_COMPOSE_FILE
cat $COMPOSE_FILE \
    | sed -e 's/$SIPHON_ENV/staging/g' \
    | sed -e 's/$RABBITMQ_HOST/local.getsiphon.com/g' \
    | sed -e 's/$RABBITMQ_PORT/5672/g' > "${TMP_COMPOSE_FILE}"

# echo "Stopping any running containers..."
# docker-compose -f compose.yml stop
# docker stop $(docker ps -a -q)

echo "Building and running streamer containers..."
docker-compose -f $TMP_COMPOSE_FILE build && docker-compose -f $TMP_COMPOSE_FILE up && rm -f $TMP_COMPOSE_FILE
