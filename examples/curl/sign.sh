#!/bin/sh
# Signed download/view URLs: POST /blob/:id/sign. Usage: ./sign.sh <id>
set -eu
: "${BLOB_URL:?set BLOB_URL}"; : "${BLOB_API_KEY:?set BLOB_API_KEY}"
ID="${1:?usage: sign.sh <id>}"
curl -s -X POST "$BLOB_URL/blob/$ID/sign" \
  -H "Authorization: Bearer $BLOB_API_KEY" > /tmp/out.txt
cat /tmp/out.txt | jq . 2>/dev/null || cat /tmp/out.txt
