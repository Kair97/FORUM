#!/bin/bash

set -e

echo "Building Docker image..."
docker build -t forum .

echo "Stopping existing container if running..."
docker stop forum-app 2>/dev/null || true
docker rm forum-app 2>/dev/null || true

echo "Starting the container..."

# "docker stop" sends SIGTERM, which our graceful shutdown catches to finish active requests cleanly.
docker run -d \
  --name forum-app \
  -p 8080:8080 \
  -v "$(pwd -W)/volume/database:/app/volume/database" \
  -v "$(pwd -W)/volume/uploaded_imgs:/app/volume/uploaded_imgs" \
  forum

echo ""
echo "Forum is running at http://localhost:8080"
echo ""
