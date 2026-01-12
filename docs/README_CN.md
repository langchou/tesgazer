<p align="center">
  <img src="../assets/logo.png" alt="tesgazer logo" width="200">
</p>

<h1 align="center">tesgazer</h1>

<p align="center">
  <strong>轻量级、自托管的 Tesla 车辆数据记录器，使用 Go 编写</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/License-GPL--3.0-blue?style=flat-square" alt="License">
  <img src="https://img.shields.io/badge/PostgreSQL-15+-336791?style=flat-square&logo=postgresql" alt="PostgreSQL">
  <a href="../README.md"><img src="https://img.shields.io/badge/Docs-English-blue?style=flat-square" alt="English"></a>
</p>

---

## 功能特性

- 🚗 **车辆追踪** — 实时位置、电量、温度、里程
- 🛣️ **行程记录** — 自动识别行程，记录距离、时长、能耗
- ⚡ **充电记录** — 完整充电历史，包含功率曲线
- 📊 **REST API** — 完整的 RESTful 接口
- 🔄 **WebSocket** — 实时数据推送到前端
- 🎨 **前端可插拔** — 自带官方前端，也可替换为自定义 UI
- ⚡ **Tesla Streaming API** — 亚秒级唤醒检测
- 😴 **智能休眠** — 零吸血鬼功耗，智能暂停/恢复
- 🔁 **双链路架构** — RESTful 轮询 + Streaming 实时推送

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
| GET | `/api/cars/:id/stats` | 车辆统计 |
| GET | `/api/cars/:id/drives` | 行程历史 |
| GET | `/api/cars/:id/charges` | 充电历史 |
| GET | `/api/cars/:id/parkings` | 停车历史 |
| GET | `/api/cars/:id/footprint` | 足迹数据（90天） |
| POST | `/api/cars/:id/suspend` | 暂停日志（允许休眠） |
| POST | `/api/cars/:id/resume` | 恢复日志 |
| GET | `/api/drives/:id` | 行程详情 |
| GET | `/api/drives/:id/positions` | 行程轨迹 |
| GET | `/api/charges/:id` | 充电详情 |
| GET | `/api/charges/:id/details` | 充电曲线数据 |
| GET | `/api/parkings/:id` | 停车详情 |

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
| `DEBUG` | 调试模式 | `false` |

### 轮询间隔

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `POLL_INTERVAL_ONLINE` | 在线轮询间隔 | `15s` |
| `POLL_INTERVAL_DRIVING` | 驾驶轮询间隔 | `3s` |
| `POLL_INTERVAL_CHARGING` | 充电轮询间隔 | `5s` |
| `POLL_INTERVAL_ASLEEP` | 睡眠轮询间隔 | `30s` |
| `POLL_BACKOFF_INITIAL` | 初始退避间隔 | `1s` |
| `POLL_BACKOFF_MAX` | 最大退避间隔 | `30s` |
| `POLL_BACKOFF_FACTOR` | 退避因子 | `2.0` |

### 休眠控制

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `SUSPEND_AFTER_IDLE_MIN` | 空闲多久后暂停（分钟） | `15` |
| `SUSPEND_POLL_INTERVAL` | 暂停状态轮询间隔 | `21m` |
| `REQUIRE_NOT_UNLOCKED` | 是否要求上锁才能休眠 | `false` |

### Streaming API

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `USE_STREAMING_API` | 启用 Streaming API | `true` |
| `STREAMING_HOST` | Streaming WebSocket 地址 | `wss://streaming.vn.cloud.tesla.cn/streaming/` |
| `STREAMING_RECONNECT_DELAY` | 重连延迟 | `5s` |

### 逆地理编码（可选）

逆地理编码用于将坐标转换为可读地址，支持两种服务：

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `AMAP_API_KEY` | [高德地图](https://lbs.amap.com/) API Key（中国区推荐） | — |

- **配置 `AMAP_API_KEY`**：使用高德地图，中国区速度快、精度高
- **不配置**：自动回退到 [Nominatim](https://nominatim.openstreetmap.org/) (OpenStreetMap)，免费、全球覆盖，限流 1 次/秒

### 其他

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `TOKEN_FILE` | Token 存储文件 | `tokens.json` |

## 许可证

[GPL-3.0](../LICENSE)

---

<p align="center">
  <sub>灵感来源于 <a href="https://github.com/teslamate-org/teslamate">TeslaMate</a> ❤️</sub>
</p>
