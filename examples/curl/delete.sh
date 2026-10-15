#!/bin/sh
# Delete a blob: DELETE /blob/:id. Usage: ./delete.sh <id>
set -eu
: "${BLOB_URL:?set BLOB_URL}"; : "${BLOB_API_KEY:?set BLOB_API_KEY}"
ID="${1:?usage: delete.sh <id>}"
curl -s -w "\nHTTP:%{http_code}\n" -X DELETE "$BLOB_URL/blob/$ID" \
  -H "Authorization: Bearer $BLOB_API_KEY"
