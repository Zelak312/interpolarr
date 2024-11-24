#!/bin/sh
set -e

# OVERWRITE ENV VARIABLES
# Source the .env.docker file
# Needs to be in docker
if [ -f .env.docker ]; then
  set -a
  . ./.env.docker
  set +a
fi

# Create process directory
if [ ! -d "/interpolarr/process" ]; then
  mkdir -p /interpolarr/process
fi

# If the PUID or PGID environment variables are set (non-empty)
if [ -n "$PUID" ] && [ -n "$PGID" ]; then
    # set permissions for directory
    chown -R $PUID:$PGID /interpolarr
    # If user with PUID already exists, use existing user
    if ! id -u "$PUID" >/dev/null 2>&1; then
        # Else, create new user and group with PUID and PGID
        addgroup --gid $PGID usergroup
        adduser --disabled-password --gecos "" --uid $PUID --gid $PGID user
    fi
    # Start the app under the defined user
    echo "Starting app under user: $(id -u $PUID)"
    env | gosu $PUID:$PGID ./interpolarr
else
    # Start the app as root
    chown -R $(id -u):$(id -g) /interpolarr
    env | ./interpolarr
fi