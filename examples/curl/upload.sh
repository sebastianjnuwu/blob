#!/bin/sh
# Upload a file: PUT /blob (multipart form). Usage: ./upload.sh <file> [bucket]
set -eu
: "${BLOB_URL:?set BLOB_URL}"; : "${BLOB_API_KEY:?set BLOB_API_KEY}"
FILE="${1:?usage: upload.sh <file> [bucket]}"
BUCKET="${2:-demos}"
curl -s -X PUT "$BLOB_URL/blob" \
  -H "Authorization: Bearer $BLOB_API_KEY" \
  -F "file=@$FILE" \
  -F "bucket=$BUCKET" > /tmp/out.txt
cat /tmp/out.txt | jq . 2>/dev/null || cat /tmp/out.txt
