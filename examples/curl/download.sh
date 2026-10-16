#!/bin/sh
# Download a blob to a file: GET /blob/:id/download. Usage: ./download.sh <id> [out]
set -eu
: "${BLOB_URL:?set BLOB_URL}"; : "${BLOB_API_KEY:?set BLOB_API_KEY}"
ID="${1:?usage: download.sh <id> [out]}"
OUT="${2:-$ID.bin}"
curl -s -o "$OUT" -w "HTTP:%{http_code} %{size_download}b -> $OUT\n" "$BLOB_URL/blob/$ID/download" \
  -H "Authorization: Bearer $BLOB_API_KEY"
