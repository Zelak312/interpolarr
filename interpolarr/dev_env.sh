#!/bin/sh

if [ "$1" = "up" ]; then
    docker compose -f compose.dev.yml up -d
elif [ "$1" = "down" ]; then
    docker compose -f compose.dev.yml down
elif [ "$1" = "build" ]; then
    docker compose -f compose.dev.yml build
elif [ "$1" = "restart" ]; then
    docker compose -f compose.dev.yml down --remove-orphans
    docker compose -f compose.dev.yml up -d
elif [ "$1" = "log" ]; then
    docker compose -f compose.dev.yml logs -f
elif [ "$1" = "up-attach" ]; then
    docker compose -f compose.dev.yml up
else
    echo "Argument is neither up nor down"
fi