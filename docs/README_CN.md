<p align="center">
  <img src="../assets/logo.png" alt="tesgazer logo" width="200">
</p>

<h1 align="center">tesgazer</h1>

<p align="center">
  <strong>轻量级、自托管的 Tesla 车辆数据记录器，使用 Go 编写</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/License-MIT-green?style=flat-square" alt="License">
  <img src="https://img.shields.io/badge/PostgreSQL-15+-336791?style=flat-square&logo=postgresql" alt="PostgreSQL">
  <a href="../README.md"><img src="https://img.shields.io/badge/Docs-English-blue?style=flat-square" alt="English"></a>
</p>

---

## 功能特性

- 🚗 **车辆追踪** — 实时位置、电量、温度、里程
- 🛣️ **行程记录** — 自动识别行程，记录距离、时长、能耗
- ⚡ **充电记录** — 完整充电历史，包含功率曲线
- 📊 **REST API** — 完整的 RESTful 接口
- 🔄 **WebSocket** — 实时数据推送
- 🎨 **前端可插拔** — 自带官方前端，也可替换为自定义 UI

## 快速部署

```bash
git clone https://github.com/langchou/tesgazer.git
cd tesgazer
docker-compose up -d
```

打开 `http://localhost:3000`，输入 Tesla Token，完成。

> Token 获取：使用 [Tesla Auth](https://github.com/adriankumpf/tesla_auth) 等工具。

## API 接口

### 认证

```http
POST /api/auth/token
Content-Type: application/json

{"access_token": "...", "refresh_token": "..."}
```

### 接口列表

| 方法 | 接口 | 说明 |
|------|------|------|
| GET | `/api/cars` | 车辆列表 |
| GET | `/api/cars/:id` | 车辆详情 |
| GET | `/api/cars/:id/state` | 实时状态 |
| GET | `/api/cars/:id/drives` | 行程历史 |
| GET | `/api/cars/:id/charges` | 充电历史 |
| GET | `/api/drives/:id` | 行程详情 |
| GET | `/api/drives/:id/positions` | 行程轨迹 |
| GET | `/api/charges/:id` | 充电详情 |

### WebSocket

```javascript
const ws = new WebSocket('ws://localhost:4000/ws')

ws.onmessage = (event) => {
  const { type, data } = JSON.parse(event.data)
  // type: 'init' | 'state_update'
}
```

## 配置项

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `PORT` | 服务端口 | `4000` |
| `DATABASE_URL` | PostgreSQL 连接地址 | — |
| `POLL_INTERVAL_ONLINE` | 在线轮询间隔 | `10s` |
| `POLL_INTERVAL_ASLEEP` | 睡眠轮询间隔 | `60s` |
| `POLL_INTERVAL_CHARGING` | 充电轮询间隔 | `30s` |

## 许可证

[MIT](LICENSE)

---

<p align="center">
  <sub>灵感来源于 <a href="https://github.com/teslamate-org/teslamate">TeslaMate</a> ❤️</sub>
</p>
