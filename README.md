<p align="center">
  <img src="assets/logo.png" alt="tesgazer logo" width="200">
</p>

<h1 align="center">tesgazer</h1>

<p align="center">
  <strong>A lightweight, self-hosted Tesla vehicle data logger written in Go</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/License-GPL--3.0-blue?style=flat-square" alt="License">
  <img src="https://img.shields.io/badge/PostgreSQL-15+-336791?style=flat-square&logo=postgresql" alt="PostgreSQL">
  <a href="docs/README_CN.md"><img src="https://img.shields.io/badge/文档-中文版-red?style=flat-square" alt="中文文档"></a>
</p>

---

## Features

- 🚗 **Vehicle Tracking** — Real-time location, battery, temperature, odometer
- 🛣️ **Drive Logging** — Automatic trip detection with distance, duration, energy
- ⚡ **Charge Sessions** — Complete charging history with power curves
- 📊 **REST API** — Full-featured RESTful endpoints
- 🔄 **WebSocket** — Real-time data push to frontend
- 🎨 **Pluggable UI** — Comes with official frontend, or bring your own
- ⚡ **Tesla Streaming API** — Sub-second wake detection via WebSocket
- 😴 **Smart Sleep** — Zero vampire drain with intelligent suspend/resume
- 🔁 **Dual-Link Architecture** — RESTful polling + Streaming for reliability

## Quick Start

```bash
git clone https://github.com/langchou/tesgazer.git
cd tesgazer
docker-compose up -d
```

Open `http://localhost:3000`, enter your Tesla token, done.

> Get token via [Tesla Auth](https://github.com/adriankumpf/tesla_auth) or similar tools.

## API Reference

### Authentication

```http
POST /api/auth/token
Content-Type: application/json

{"access_token": "...", "refresh_token": "..."}
```

### Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/cars` | List vehicles |
| GET | `/api/cars/:id` | Vehicle details |
| GET | `/api/cars/:id/state` | Real-time state |
| GET | `/api/cars/:id/stats` | Vehicle statistics |
| GET | `/api/cars/:id/drives` | Drive history |
| GET | `/api/cars/:id/charges` | Charge history |
| GET | `/api/cars/:id/parkings` | Parking history |
| GET | `/api/cars/:id/footprint` | Footprint data (90 days) |
| POST | `/api/cars/:id/suspend` | Suspend logging (allow sleep) |
| POST | `/api/cars/:id/resume` | Resume logging |
| GET | `/api/drives/:id` | Drive details |
| GET | `/api/drives/:id/positions` | Drive trajectory |
| GET | `/api/charges/:id` | Charge details |
| GET | `/api/charges/:id/details` | Charge curve data |
| GET | `/api/parkings/:id` | Parking details |

### WebSocket

```javascript
const ws = new WebSocket('ws://localhost:4000/ws')

ws.onmessage = (event) => {
  const { type, data } = JSON.parse(event.data)
  // type: 'init' | 'state_update'
}
```

## Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | `4000` |
| `DATABASE_URL` | PostgreSQL connection | — |
| `DEBUG` | Enable debug mode | `false` |

### Polling Intervals

| Variable | Description | Default |
|----------|-------------|---------|
| `POLL_INTERVAL_ONLINE` | Online polling | `15s` |
| `POLL_INTERVAL_DRIVING` | Driving polling | `3s` |
| `POLL_INTERVAL_CHARGING` | Charging polling | `5s` |
| `POLL_INTERVAL_ASLEEP` | Asleep polling | `30s` |
| `POLL_BACKOFF_INITIAL` | Initial backoff | `1s` |
| `POLL_BACKOFF_MAX` | Max backoff | `30s` |
| `POLL_BACKOFF_FACTOR` | Backoff factor | `2.0` |

### Sleep/Suspend

| Variable | Description | Default |
|----------|-------------|---------|
| `SUSPEND_AFTER_IDLE_MIN` | Idle minutes before suspend | `15` |
| `SUSPEND_POLL_INTERVAL` | Suspend polling interval | `21m` |
| `REQUIRE_NOT_UNLOCKED` | Require locked to sleep | `false` |

### Streaming API

| Variable | Description | Default |
|----------|-------------|---------|
| `USE_STREAMING_API` | Enable Streaming API | `true` |
| `STREAMING_HOST` | Streaming WebSocket URL | `wss://streaming.vn.cloud.tesla.cn/streaming/` |
| `STREAMING_RECONNECT_DELAY` | Reconnect delay | `5s` |

### Geocoding (Optional)

Reverse geocoding converts coordinates to human-readable addresses. Two providers are supported:

| Variable | Description | Default |
|----------|-------------|---------|
| `AMAP_API_KEY` | [Amap](https://lbs.amap.com/) API key (recommended for China) | — |

- **With `AMAP_API_KEY`**: Uses Amap (高德地图) for geocoding — fast and accurate in China
- **Without `AMAP_API_KEY`**: Falls back to [Nominatim](https://nominatim.openstreetmap.org/) (OpenStreetMap) — free, worldwide coverage, rate-limited to 1 req/sec

### Other

| Variable | Description | Default |
|----------|-------------|---------|
| `TOKEN_FILE` | Token storage file | `tokens.json` |

## License

[GPL-3.0](LICENSE)

---

<p align="center">
  <sub>Inspired by <a href="https://github.com/teslamate-org/teslamate">TeslaMate</a> ❤️</sub>
</p>
