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
| GET | `/api/cars/:id/drives` | Drive history |
| GET | `/api/cars/:id/charges` | Charge history |
| POST | `/api/cars/:id/suspend` | Suspend logging (allow sleep) |
| POST | `/api/cars/:id/resume` | Resume logging |
| GET | `/api/drives/:id` | Drive details |
| GET | `/api/drives/:id/positions` | Drive trajectory |
| GET | `/api/charges/:id` | Charge details |

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
| `POLL_INTERVAL_ONLINE` | Online polling | `10s` |
| `POLL_INTERVAL_ASLEEP` | Asleep polling | `60s` |
| `POLL_INTERVAL_CHARGING` | Charging polling | `30s` |

## License

[GPL-3.0](LICENSE)

---

<p align="center">
  <sub>Inspired by <a href="https://github.com/teslamate-org/teslamate">TeslaMate</a> ❤️</sub>
</p>
