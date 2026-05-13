#!/bin/bash

set -e

echo "Building Docker image..."
docker build -t forum .

echo "Stopping existing container if running..."
docker stop forum-app 2>/dev/null || true 
docker rm forum-app 2>/dev/null || true 

echo "Starting the container..."

docker run -d \
  --name forum-app \
  -p 8080:8080 \
  -v "$(pwd -W)/database:/app/database" \
  -v "$(pwd -W)/web/static/uploads:/app/web/static/uploads" \
  forum

echo ""
echo "Forum is running at http://localhost:8080"
echo ""
