# REST API Reference

The `faultcraft` management daemon serves both Toxiproxy v2-compatible endpoints and extended control endpoints for telemetry and presets.

---

## 1. Proxies

### List Proxies
* **Endpoint**: `GET /proxies`
* **Response**: `200 OK`
```json
{
  "postgres_chaos": {
    "name": "postgres_chaos",
    "listen": "localhost:15432",
    "upstream": "localhost:5432",
    "enabled": true,
    "toxics": []
  }
}
```

### Create Proxy
* **Endpoint**: `POST /proxies`
* **Request**:
```json
{
  "name": "redis_chaos",
  "listen": "localhost:16379",
  "upstream": "localhost:6379",
  "enabled": true
}
```
* **Response**: `201 Created`

### Update Proxy
* **Endpoint**: `POST /proxies/{name}`
* **Request**:
```json
{
  "listen": "localhost:16379",
  "upstream": "localhost:6379",
  "enabled": false
}
```
* **Response**: `200 OK`

### Delete Proxy
* **Endpoint**: `DELETE /proxies/{name}`
* **Response**: `204 No Content`

---

## 2. Toxics

### Add Toxic
* **Endpoint**: `POST /proxies/{name}/toxics`
* **Request**:
```json
{
  "name": "my_delay",
  "type": "latency",
  "stream": "downstream",
  "toxicity": 1.0,
  "attributes": {
    "latency": 300,
    "jitter": 50
  }
}
```
* **Response**: `200 OK`

### Delete Toxic
* **Endpoint**: `DELETE /proxies/{name}/toxics/{toxic_name}`
* **Response**: `204 No Content`

### Global Reset
* **Endpoint**: `POST /reset`
* **Description**: Removes all active toxics from every registered proxy and ensures all proxies are enabled.
* **Response**: `200 OK`

---

## 3. Extended Endpoints

### List Presets
* **Endpoint**: `GET /api/v1/presets`
* **Response**: `200 OK`
```json
[
  {
    "id": "flaky_db",
    "name": "Flaky Database",
    "description": "High latency with intermittent connection resets"
  },
  {
    "id": "cellular_3g",
    "name": "Degraded 3G Mobile",
    "description": "Bandwidth throttling and packet fragmentation"
  }
]
```

### Apply Preset to Proxy
* **Endpoint**: `POST /api/v1/presets/apply`
* **Request**:
```json
{
  "proxy": "postgres_chaos",
  "preset": "flaky_db"
}
```
* **Response**: `200 OK`

### Real-Time Event Stream
* **Endpoint**: `GET /api/v1/events`
* **Headers**: `Accept: text/event-stream`
* **Response**: Server-Sent Events stream emitting metrics payloads:
```
data: {"type":"metrics","active_connections":14,"bytes_upstream":1048576,"bytes_downstream":8388608,"timestamp":1731000000}
```

### L7 Frame Inspector
* **Endpoint**: `GET /api/v1/inspector`
* **Response**: `200 OK` (Array of captured and dissected network frames)
* **Endpoint**: `DELETE /api/v1/inspector`
* **Response**: `204 No Content`

### Automated Incident Sequences
* **Endpoint**: `GET /api/v1/sequences`
* **Response**: `200 OK` (Array of multi-phase chaos sequences)
* **Endpoint**: `POST /api/v1/sequences/run`
* **Request**:
```json
{
  "proxy": "postgres_chaos",
  "sequence_id": "db_failover_drill"
}
```
* **Response**: `200 OK`
* **Endpoint**: `POST /api/v1/sequences/stop`
* **Response**: `200 OK`

### Configuration Export & Import
* **Endpoint**: `GET /api/v1/config/export`
* **Response**: `200 OK` (Downloadable JSON bundle of all proxies and toxics)
* **Endpoint**: `POST /api/v1/config/import`
* **Request**: Configuration bundle JSON
* **Response**: `200 OK` (`{"status":"imported","imported_proxies":1,"imported_toxics":2}`)
