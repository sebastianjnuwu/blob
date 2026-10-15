#!/bin/sh
# Multipart upload: initiate -> chunks (sha256) -> complete.
# Usage: ./multipart.sh <file> [bucket]
set -eu
: "${BLOB_URL:?set BLOB_URL}"; : "${BLOB_API_KEY:?set BLOB_API_KEY}"
FILE="${1:?usage: multipart.sh <file> [bucket]}"
BUCKET="${2:-demos}"
SIZE=$(wc -c < "$FILE" | tr -d ' ')
NAME=$(basename "$FILE")

UPLOAD_ID=$(curl -s -X POST "$BLOB_URL/blob/initiate" \
  -H "Authorization: Bearer $BLOB_API_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"bucket\":\"$BUCKET\",\"filename\":\"$NAME\",\"size\":$SIZE}" | grep -o '"uploadId":"[^"]*"' | cut -d'"' -f4)
[ -z "$UPLOAD_ID" ] && { echo "initiate failed"; exit 1; }
echo "uploadId: $UPLOAD_ID"

split -b 1048576 "$FILE" /tmp/chunk_ || exit 1
i=0
for c in /tmp/chunk_*; do
  HASH=$(sha256sum "$c" | awk '{print $1}')
  curl -s -o /dev/null -w "chunk $i HTTP:%{http_code}\n" -X PUT "$BLOB_URL/blob/$UPLOAD_ID/chunk" \
    -H "Authorization: Bearer $BLOB_API_KEY" \
    -H "X-Chunk-Index: $i" -H "X-Chunk-Hash: $HASH" \
    --data-binary "@$c"
  i=$((i + 1))
done
rm -f /tmp/chunk_*

FINAL=$(sha256sum "$FILE" | awk '{print $1}')
curl -s -X POST "$BLOB_URL/blob/$UPLOAD_ID/complete" \
  -H "Authorization: Bearer $BLOB_API_KEY" \
  -H "X-Final-Hash: $FINAL" > /tmp/mp-out.txt
cat /tmp/mp-out.txt | jq . 2>/dev/null || cat /tmp/mp-out.txt
