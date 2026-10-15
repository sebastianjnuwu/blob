#!/bin/sh
# List blobs: GET /blob?bucket=&page=. Usage: ./list.sh [bucket] [page]
set -eu
: "${BLOB_URL:?set BLOB_URL}"; : "${BLOB_API_KEY:?set BLOB_API_KEY}"
BUCKET="${1:-}"; PAGE="${2:-1}"
curl -s -G "$BLOB_URL/blob" --data-urlencode "bucket=$BUCKET" --data-urlencode "page=$PAGE" \
  -H "Authorization: Bearer $BLOB_API_KEY" > /tmp/out.txt
cat /tmp/out.txt | jq . 2>/dev/null || cat /tmp/out.txt
