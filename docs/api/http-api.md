# HTTP API Reference

## Overview

Conflux provides a RESTful HTTP API for key-value operations. All endpoints return JSON responses and support standard HTTP methods.

**Base URL**: `http://<node-address>:8080`

## Authentication

Currently, the API does not require authentication. This is suitable for development and trusted network environments. For production deployments, consider adding authentication via a reverse proxy or API gateway.

## Endpoints

### Health Check

Check if the node is healthy and ready to serve requests.

```http
GET /health
```

**Response** (200 OK):
```json
{
  "status": "healthy",
  "node_id": "raft-sample-0",
  "state": "leader",
  "term": 5,
  "commit_index": 42
}
```

**Fields**:
- `status`: Health status (`healthy` or `unhealthy`)
- `node_id`: Unique identifier for this node
- `state`: Raft state (`leader`, `follower`, or `candidate`)
- `term`: Current Raft term
- `commit_index`: Last committed log index

---

### Get Value

Retrieve the value for a given key.

```http
GET /kv/{key}
```

**Parameters**:
- `key` (path): The key to retrieve

**Response** (200 OK):
```json
{
  "value": "hello world"
}
```

**Response** (404 Not Found):
```json
{
  "error": "key not found"
}
```

**Example**:
```bash
curl http://localhost:8080/kv/mykey
```

---

### Set Value

Set or update the value for a given key.

```http
PUT /kv/{key}
Content-Type: application/json

{
  "value": "string value"
}
```

**Parameters**:
- `key` (path): The key to set
- `value` (body): The value to store (must be a string)

**Response** (200 OK):
```json
{
  "status": "ok"
}
```

**Response** (307 Temporary Redirect):
If the current node is not the leader, it will redirect to the leader:
```
Location: http://raft-sample-1:8080/kv/mykey
```

**Response** (503 Service Unavailable):
```json
{
  "error": "not leader",
  "leader_hint": "raft-sample-1:8080"
}
```

**Example**:
```bash
curl -X PUT http://localhost:8080/kv/mykey \
  -H "Content-Type: application/json" \
  -d '{"value":"hello world"}'
```

---

### Delete Value

Delete a key-value pair.

```http
DELETE /kv/{key}
```

**Parameters**:
- `key` (path): The key to delete

**Response** (200 OK):
```json
{
  "status": "ok"
}
```

**Response** (404 Not Found):
```json
{
  "error": "key not found"
}
```

**Response** (503 Service Unavailable):
```json
{
  "error": "not leader"
}
```

**Example**:
```bash
curl -X DELETE http://localhost:8080/kv/mykey
```

---

### Metrics

Prometheus-compatible metrics endpoint.

```http
GET /metrics
```

**Response** (200 OK):
```
# HELP raft_state Current Raft state (0=follower, 1=candidate, 2=leader)
# TYPE raft_state gauge
raft_state{node_id="raft-sample-0"} 2

# HELP raft_term Current Raft term
# TYPE raft_term counter
raft_term{node_id="raft-sample-0"} 5

# HELP raft_commit_index Current commit index
# TYPE raft_commit_index gauge
raft_commit_index{node_id="raft-sample-0"} 42
```

**Example**:
```bash
curl http://localhost:8080/metrics
```

## Error Handling

All errors follow a consistent JSON format:

```json
{
  "error": "error message",
  "details": "optional additional context"
}
```

### HTTP Status Codes

| Code | Meaning | Description |
|------|---------|-------------|
| 200 | OK | Request succeeded |
| 404 | Not Found | Key does not exist |
| 400 | Bad Request | Invalid request format |
| 503 | Service Unavailable | Node is not the leader or cluster has no leader |
| 500 | Internal Server Error | Unexpected server error |

## Leader Redirection

Write operations (PUT, DELETE) must be sent to the leader node. If you send a write to a follower:

1. **Option 1**: The follower returns `503 Service Unavailable` with a `leader_hint` field
2. **Option 2**: The follower returns `307 Temporary Redirect` with the leader's address

Clients should follow redirects or use the `leader_hint` to retry the request.

**Example with redirect**:
```bash
# Request to follower
curl -L -X PUT http://localhost:8081/kv/test \
  -H "Content-Type: application/json" \
  -d '{"value":"data"}'

# -L flag makes curl follow redirects automatically
```

## Consistency Guarantees

- **Reads**: Linearizable if performed on the leader; may be stale on followers
- **Writes**: Always linearizable (must go through leader and Raft consensus)
- **Ordering**: All operations are totally ordered via the Raft log

## Rate Limiting

Currently, there is no built-in rate limiting. For production use, implement rate limiting at the API gateway or load balancer level.

## Examples

### Complete Workflow

```bash
# 1. Check cluster health
curl http://localhost:8080/health

# 2. Write a value
curl -X PUT http://localhost:8080/kv/user:123 \
  -H "Content-Type: application/json" \
  -d '{"value":"John Doe"}'

# 3. Read the value
curl http://localhost:8080/kv/user:123

# 4. Update the value
curl -X PUT http://localhost:8080/kv/user:123 \
  -H "Content-Type: application/json" \
  -d '{"value":"Jane Doe"}'

# 5. Delete the value
curl -X DELETE http://localhost:8080/kv/user:123

# 6. Verify deletion
curl http://localhost:8080/kv/user:123
# Returns 404
```

### Using with jq

```bash
# Pretty-print JSON responses
curl http://localhost:8080/health | jq '.'

# Extract specific fields
curl http://localhost:8080/health | jq '.state'

# Check if node is leader
IS_LEADER=$(curl -s http://localhost:8080/health | jq -r '.state == "leader"')
echo "Is leader: $IS_LEADER"
```

## Client Libraries

While there are no official client libraries yet, the API is simple enough to use with any HTTP client:

**Go**:
```go
resp, err := http.Get("http://localhost:8080/kv/mykey")
```

**Python**:
```python
import requests
response = requests.get("http://localhost:8080/kv/mykey")
```

**JavaScript**:
```javascript
fetch('http://localhost:8080/kv/mykey')
  .then(response => response.json())
  .then(data => console.log(data));
```

## See Also

- [Architecture Documentation](../architecture/README.md)
- [Deployment Guide](../guides/deployment.md)
- [Operations Runbook](../operations/runbook.md)
