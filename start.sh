#!/bin/sh
echo "Checking for yt-dlp updates..."
pip3 install --upgrade yt-dlp --break-system-packages
echo "Starting bot..."
exec /app/bot
