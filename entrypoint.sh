#!/bin/sh
set -e

echo "Checking for yt-dlp updates..."
pip3 install --no-cache-dir --user --upgrade --break-system-packages yt-dlp || echo "Warning: Failed to update yt-dlp, using installed version"

echo "Starting bot..."
exec /app/bot
