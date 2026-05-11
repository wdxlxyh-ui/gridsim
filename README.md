# IEC 60870-5-104 Simulator

A multi-instance IEC 104 slave simulator for substation automation testing, written in Go. Simulates RTUs and bay-level devices for SCADA system development and integration testing.

## Features

- **Two operating modes**
  - **Legacy mode**: Single process = single port + single client, pure CLI
  - **Server mode** (`serve` subcommand): Multi-instance lifecycle management with a Vue 3 web UI
- **Excel-driven point configuration** — define IOA, point type, coefficient, and initial values in `.xlsx`
- **Full IEC 104 support**
  - Telemetry AI (M_ME_NC_1), teleindication DI (M_SP_NA_1), pulse PI (M_IT_NA_1)
  - Remote control DO (C_SC_NA_1), remote adjustment AO (C_SE_NC_1)
  - General interrogation (C_IC_NA_1), counter interrogation (C_CI_NA_1)
  - Spontaneous change update (COT=3)
  - Quality descriptor (QDS) simulation: invalid / not-topical / substituted / overflow / blocked
- **RESTful HTTP API** — query, update (single & batch), modify QDS
- **Web management UI** — instance CRUD, start/stop/restart, live monitoring (auto-refresh every 5s)
- **Statistics** — uptime, interrogation count, control count, spontaneous count per instance
- **Single-client enforcement** — rejects duplicate connections per instance
- **Cross-compilation** — Linux amd64 / arm64, Windows amd64, `.deb` packaging

## Tech Stack

| Layer | Choice |
|-------|--------|
| Language | Go 1.21+ |
| IEC 104 library | [go-iecp5](https://github.com/wendy512/go-iecp5) |
| Excel parsing | [excelize](https://github.com/xuri/excelize/v2) v2 |
| Frontend | Vue 3 + TypeScript + Element Plus |
| Build tool | Vite |
| CLI flags | [pflag](https://github.com/spf13/pflag) |

## Quick Start

### Legacy Mode (single instance)

```bash
go build -o bin/iec104-sim.exe ./cmd/iec104-sim/
./bin/iec104-sim.exe -p 2404 -c samples/point.xlsx -H :8080 -l info
```

### Server Mode (multi-instance with Web UI)

```bash
# Build the frontend first (requires Node.js)
cd web && npm install && npm run build && cd ..

# Build and run
go build -o bin/iec104-sim.exe ./cmd/iec104-sim/
./bin/iec104-sim.exe serve --http :8080 --config-dir ./config --log-dir ./logs

# Open http://localhost:8080 in browser
```

### CLI Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--port` | `-p` | 2404 | IEC 104 server TCP port (legacy mode) |
| `--config` | `-c` | required | Path to `.xlsx` config (legacy mode) |
| `--http` | `-H` | `:8080` | HTTP API listen address |
| `--log` | `-l` | `info` | Log level: debug / info / warn / error |
| `--config-dir` | `-c` | `./config` | Config directory (server mode) |
| `--log-dir` | `-L` | `./logs` | Log directory (server mode) |

## Configuration (Excel Point Table)

Format: `.xlsx` with sheet name `point`.

| Column | Header | Type | Required | Description |
|--------|--------|------|----------|-------------|
| A | point-name | string | yes | Point name, e.g. "母线电压" |
| B | point-number | uint32 | yes | IOA, unique per point type |
| C | value-type | string | yes | Data type: FLOAT / DOUBLE / INT / BIT |
| D | point-type | string | yes | Point type: AI / DI / PI / DO / AO |
| E | efficient | float64 | yes | Scaling factor |
| F | base-value | float64 | yes | Initial value |
| G | alias | string | no | Alias or description |

### Point Type Mapping

| Type | Chinese | Function | IEC 104 TypeID | Data |
|------|---------|----------|----------------|------|
| AI | 遥测 YC | Analog monitor | M_ME_NC_1 (13) | float32, value = base × efficient |
| DI | 遥信 YX | Digital monitor | M_SP_NA_1 (1) | bool 0/1 |
| PI | 遥脉 YM | Pulse counter | M_IT_NA_1 (15) | int32 |
| DO | 遥控 | Remote control | C_SC_NA_1 (45) | Accepts external control, updates DI point |
| AO | 遥调 | Remote adjustment | C_SE_NC_1 (48) | Accepts external control, updates AI point |

## HTTP API

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/points` | List all points |
| `GET` | `/api/points/{ioa}` | Get a single point |
| `PUT` | `/api/points/{ioa}` | Update point value + trigger spontaneous |
| `POST` | `/api/points` | Batch update point values |
| `PUT` | `/api/points/{ioa}/qds` | Update quality descriptor |
| `GET` | `/api/status` | Server runtime status |

### Examples

```bash
# List all points
curl http://localhost:8080/api/points

# Get single point
curl http://localhost:8080/api/points/16385

# Update telemetry value (triggers spontaneous update)
curl -X PUT http://localhost:8080/api/points/16385 \
  -H 'Content-Type: application/json' \
  -d '{"value": 235.5}'

# Update digital point
curl -X PUT http://localhost:8080/api/points/5 \
  -H 'Content-Type: application/json' \
  -d '{"bool_value": true}'

# Batch update
curl -X POST http://localhost:8080/api/points \
  -H 'Content-Type: application/json' \
  -d '{"points": [{"ioa": 16385, "value": 999.99}, {"ioa": 5, "bool_value": false}]}'

# Update quality descriptor
curl -X PUT http://localhost:8080/api/points/16385/qds \
  -H 'Content-Type: application/json' \
  -d '{"invalid": true, "blocked": true}'

# Server status
curl http://localhost:8080/api/status
```

### Server Mode Management API

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/instances` | List all configured instances |
| `POST` | `/api/v1/instances` | Create a new instance config |
| `GET` | `/api/v1/instances/{id}` | Get instance details |
| `PUT` | `/api/v1/instances/{id}` | Update instance config |
| `DELETE` | `/api/v1/instances/{id}` | Delete instance config |
| `POST` | `/api/v1/instances/{id}/start` | Start an instance |
| `POST` | `/api/v1/instances/{id}/stop` | Stop an instance |
| `POST` | `/api/v1/instances/{id}/restart` | Restart an instance |
| `GET` | `/api/v1/status` | Global server status |
| `POST` | `/api/v1/upload` | Upload `.xlsx` point table file |

## Project Structure

```
├── cmd/iec104-sim/        Entrypoint (legacy mode + server mode)
├── internal/
│   ├── manager/           Multi-instance lifecycle manager (max 10)
│   ├── model/             Instance config & state data models
│   └── storage/           JSON-backed config persistence
├── pkg/
│   ├── api/               HTTP API handlers (point CRUD, status)
│   ├── config/            Excel loader (.xlsx) + Point data model
│   ├── iec104/            IEC 104 server (connect, interrogation, control, publish)
│   └── library/           Concurrent-safe in-memory point store
├── web/                   Vue 3 + Element Plus frontend
│   ├── src/views/         ConfigPage.vue, MonitorPage.vue
│   └── src/api/           Axios API client
├── scripts/               start.sh / stop.sh / restart.sh
├── config/                instances.json (runtime persistence)
└── samples/               Example point.xlsx
```

## Building

```bash
# Local development
go build -o bin/iec104-sim.exe ./cmd/iec104-sim/

# Linux amd64 (Debian, Ubuntu, Kylin)
make build-linux-amd64

# Linux arm64
make build-linux-arm64

# Windows amd64
make build-windows

# All platforms
make build-all

# Full build with Web UI
make build-full

# Debian packaging
make deb-amd64    # or deb-arm64

# UPX compress (reduce ~60% size)
make compress
```

## Typical Use Cases

### Substation telemetry simulation

```bash
# Start simulator (Substation A, port 2404)
./iec104-sim -p 2404 -c substation_a.xlsx -H :8080

# Simulate voltage change via HTTP API
curl -X PUT http://localhost:8080/api/points/1001 \
  -H 'Content-Type: application/json' \
  -d '{"value": 235.5}'
# → IEC 104 client receives spontaneous update (COT=3)
```

### Multi-instance deployment

```bash
# Process 1: 220kV substation
./iec104-sim serve --http :8080 --config-dir ./config220

# Process 2: 110kV substation
./iec104-sim serve --http :8081 --config-dir ./config110
```

## License

MIT
