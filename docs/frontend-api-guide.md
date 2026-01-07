# Tesgazer 前端开发指南

## 目录

1. [API 概览](#api-概览)
2. [WebSocket 实时数据](#websocket-实时数据)
3. [车辆状态机](#车辆状态机)
4. [休眠机制说明](#休眠机制说明)
5. [数据结构定义](#数据结构定义)

---

## API 概览

**Base URL**: `http://localhost:4000`

### 车辆相关

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/cars` | 获取车辆列表 |
| GET | `/api/cars/:id` | 获取车辆详情 |
| GET | `/api/cars/:id/state` | 获取车辆实时状态 |
| POST | `/api/cars/:id/suspend` | 手动暂停日志记录 |
| POST | `/api/cars/:id/resume` | 手动恢复日志记录 |
| GET | `/api/cars/:id/stats` | 获取车辆统计数据 |

### 行程相关

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/cars/:id/drives` | 获取行程列表（分页） |
| GET | `/api/drives/:id` | 获取行程详情 |
| GET | `/api/drives/:id/positions` | 获取行程轨迹点 |

### 充电相关

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/cars/:id/charges` | 获取充电记录列表（分页） |
| GET | `/api/charges/:id` | 获取充电详情 |
| GET | `/api/charges/:id/details` | 获取充电曲线数据 |

### 系统

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/health` | 健康检查 |
| GET | `/ws` | WebSocket 连接端点 |

---

## API 详细说明

### GET /api/cars

获取所有车辆列表。

**响应示例**:
```json
{
  "data": [
    {
      "id": 1,
      "tesla_id": 1234567890,
      "tesla_vehicle_id": 1126341070917908,
      "vin": "LRW3E7EK1NC123456",
      "name": "我的特斯拉",
      "model": "model3",
      "exterior_color": "DeepBlue",
      "trim_badging": "P100D",
      "wheel_type": "Pinwheel18",
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-07T12:00:00Z"
    }
  ]
}
```

### GET /api/cars/:id/state

获取车辆实时状态（内存中的最新数据）。

**响应示例**:
```json
{
  "data": {
    "car_id": 1,
    "state": "online",
    "battery_level": 67,
    "usable_battery_level": 65,
    "range_km": 320.5,
    "ideal_range_km": 350.0,
    "latitude": 31.2304,
    "longitude": 121.4737,
    "heading": 180,
    "speed": null,
    "power": 0,
    "odometer": 12345.6,
    "plugged_in": false,
    "charging_state": "Disconnected",
    "charger_power": 0,
    "charge_limit_soc": 80,
    "time_to_full_charge": 0,
    "charger_voltage": 0,
    "charger_current": 0,
    "inside_temp": 22.5,
    "outside_temp": 15.0,
    "is_climate_on": false,
    "locked": true,
    "sentry_mode": false,
    "is_user_present": false,
    "doors_open": false,
    "windows_open": false,
    "frunk_open": false,
    "trunk_open": false,
    "tpms_pressure_fl": 2.9,
    "tpms_pressure_fr": 2.9,
    "tpms_pressure_rl": 2.9,
    "tpms_pressure_rr": 2.9,
    "car_version": "2024.26.9",
    "last_updated": "2024-01-07T14:30:00Z"
  }
}
```

### POST /api/cars/:id/suspend

手动暂停日志记录，允许车辆进入休眠。

**请求**: 无需 body

**成功响应** (200):
```json
{
  "message": "Logging suspended",
  "car_id": 1
}
```

**错误响应** (400):
```json
{
  "error": "cannot suspend: vehicle is driving"
}
```

**可能的错误**:
- `cannot suspend: vehicle is driving` - 车辆正在行驶
- `cannot suspend: vehicle is charging` - 车辆正在充电
- `cannot suspend: vehicle is updating` - 车辆正在更新

### POST /api/cars/:id/resume

手动恢复日志记录。

**请求**: 无需 body

**成功响应** (200):
```json
{
  "message": "Logging resumed",
  "car_id": 1
}
```

### GET /api/cars/:id/drives

获取行程列表（分页）。

**查询参数**:
| 参数 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| page | int | 1 | 页码 |
| per_page | int | 20 | 每页数量（最大100） |

**响应示例**:
```json
{
  "data": [
    {
      "id": 1,
      "car_id": 1,
      "start_time": "2024-01-07T10:00:00Z",
      "end_time": "2024-01-07T10:30:00Z",
      "duration_min": 30.5,
      "start_battery_level": 80,
      "end_battery_level": 70,
      "start_range_km": 350.0,
      "end_range_km": 300.0,
      "distance_km": 25.5,
      "avg_speed_kmh": 50.0,
      "max_speed_kmh": 120.0
    }
  ],
  "pagination": {
    "page": 1,
    "per_page": 20,
    "total": 100
  }
}
```

### GET /api/drives/:id/positions

获取行程轨迹点。

**响应示例**:
```json
{
  "data": [
    {
      "id": 1,
      "car_id": 1,
      "drive_id": 1,
      "recorded_at": "2024-01-07T10:00:00Z",
      "latitude": 31.2304,
      "longitude": 121.4737,
      "heading": 180,
      "speed": 50,
      "power": 15,
      "battery_level": 80,
      "range_km": 350.0,
      "odometer": 12345.6,
      "inside_temp": 22.5,
      "outside_temp": 15.0,
      "tpms_pressure_fl": 2.9,
      "tpms_pressure_fr": 2.9,
      "tpms_pressure_rl": 2.9,
      "tpms_pressure_rr": 2.9
    }
  ]
}
```

---

## WebSocket 实时数据

### 连接

```javascript
const ws = new WebSocket('ws://localhost:4000/ws');
```

### 消息格式

所有消息使用 JSON 格式：

```typescript
interface WebSocketMessage {
  type: string;
  data: any;
}
```

### 消息类型

#### 1. `state_update` - 车辆状态更新

当车辆状态发生变化时推送。

```json
{
  "type": "state_update",
  "data": {
    "car_id": 1,
    "state": "online",
    "battery_level": 67,
    "usable_battery_level": 65,
    "range_km": 320.5,
    "ideal_range_km": 350.0,
    "latitude": 31.2304,
    "longitude": 121.4737,
    "heading": 180,
    "speed": null,
    "power": 0,
    "odometer": 12345.6,
    "plugged_in": false,
    "charging_state": "Disconnected",
    "charger_power": 0,
    "charge_limit_soc": 80,
    "time_to_full_charge": 0,
    "charger_voltage": 0,
    "charger_current": 0,
    "inside_temp": 22.5,
    "outside_temp": 15.0,
    "is_climate_on": false,
    "locked": true,
    "sentry_mode": false,
    "is_user_present": false,
    "doors_open": false,
    "windows_open": false,
    "frunk_open": false,
    "trunk_open": false,
    "tpms_pressure_fl": 2.9,
    "tpms_pressure_fr": 2.9,
    "tpms_pressure_rl": 2.9,
    "tpms_pressure_rr": 2.9,
    "car_version": "2024.26.9",
    "last_updated": "2024-01-07T14:30:00Z"
  }
}
```

### 推送频率

| 车辆状态 | WebSocket 推送频率 | 说明 |
|----------|-------------------|------|
| driving | ~3秒 | 高频推送，实时追踪 |
| charging | ~30秒 | 中频推送，监控充电 |
| online | ~10秒 | 常规推送 |
| suspended | 无推送 | 等待休眠 |
| asleep | 无推送 | 车辆休眠中 |

---

## 车辆状态机

### 状态定义

| 状态 | 描述 |
|------|------|
| `online` | 车辆在线，空闲状态 |
| `driving` | 正在行驶 |
| `charging` | 正在充电 |
| `suspended` | 日志暂停，等待休眠 |
| `asleep` | 车辆休眠 |
| `offline` | 车辆离线 |
| `updating` | 正在更新软件 |

### 状态流转图

```
                         ┌──────────────────────────────────────┐
                         │              online                  │
                         │         (车辆在线空闲)                │
                         └──────────────┬───────────────────────┘
                                        │
           ┌────────────────────────────┼────────────────────────┐
           │                            │                        │
           ▼                            ▼                        ▼
     ┌──────────┐              ┌──────────┐              ┌──────────┐
     │ driving  │              │ charging │              │ updating │
     │ (行驶中) │              │ (充电中) │              │ (更新中) │
     └────┬─────┘              └────┬─────┘              └────┬─────┘
          │                         │                         │
          └─────────────────────────┼─────────────────────────┘
                                    │ 停止活动
                                    ▼
                         ┌────────────────────────┐
                         │        online          │
                         │   空闲计时开始 (15min)  │
                         └───────────┬────────────┘
                                     │ 满足休眠条件
                                     ▼
                         ┌────────────────────────┐
                         │      suspended         │
                         │    (暂停日志记录)       │
                         │   等待车辆自行休眠      │
                         └───────────┬────────────┘
                                     │ 车辆进入休眠
                                     ▼
                         ┌────────────────────────┐
                         │   asleep / offline     │
                         │      (车辆休眠)         │
                         └────────────────────────┘
```

### 用户可执行的操作

| 当前状态 | 可暂停 | 可恢复 | 说明 |
|----------|--------|--------|------|
| online | ✅ | - | 可手动暂停 |
| driving | ❌ | - | 行驶中不可暂停 |
| charging | ❌ | - | 充电中不可暂停 |
| updating | ❌ | - | 更新中不可暂停 |
| suspended | - | ✅ | 可手动恢复 |
| asleep | - | ✅ | 可恢复（增加轮询频率） |
| offline | - | ✅ | 可恢复（增加轮询频率） |

---

## 休眠机制说明

### 自动休眠流程

1. **空闲检测**: 车辆在 `online` 状态下，无驾驶/充电活动
2. **等待时间**: 默认空闲 **15 分钟** 后检查休眠条件
3. **条件检查**: 检查是否满足所有休眠条件
4. **进入暂停**: 满足条件后进入 `suspended` 状态
5. **车辆休眠**: 系统停止频繁轮询，车辆自行进入休眠
6. **状态更新**: 检测到车辆休眠后更新为 `asleep`

### 休眠阻止条件

以下任一条件会**阻止**车辆进入 `suspended` 状态：

| 条件 | 代码标识 | 描述 |
|------|----------|------|
| 用户在场 | `user_present` | 检测到车主在车内 |
| 哨兵模式 | `sentry_mode` | 哨兵模式开启 |
| 预热/预冷 | `preconditioning` | 空调预热或预冷中 |
| 空调开启 | `climate_on` | 空调正在运行 |
| 车门打开 | `doors_open` | 任意车门打开 |
| 后备箱打开 | `trunk_open` | 后备箱打开 |
| 前备箱打开 | `frunk_open` | 前备箱打开 |
| 窗户打开 | `windows_open` | 任意窗户打开 |
| 未锁车 | `unlocked` | 车辆未锁定（可配置） |
| 电力消耗 | `power_usage` | 检测到电力消耗 |
| 下载更新 | `downloading_update` | 正在下载软件更新 |

---

## 数据结构定义

### TypeScript 类型定义

```typescript
// 车辆状态
type VehicleState =
  | 'online'
  | 'driving'
  | 'charging'
  | 'suspended'
  | 'asleep'
  | 'offline'
  | 'updating';

// 充电状态
type ChargingState =
  | 'Disconnected'
  | 'Charging'
  | 'Complete'
  | 'Stopped';

// 车辆信息
interface Car {
  id: number;
  tesla_id: number;
  tesla_vehicle_id: number;
  vin: string;
  name: string;
  model: string;
  exterior_color: string;
  trim_badging: string;
  wheel_type: string;
  created_at: string;
  updated_at: string;
}

// 车辆实时状态
interface VehicleStateData {
  car_id: number;
  state: VehicleState;

  // 电池
  battery_level: number;           // 电量百分比 (0-100)
  usable_battery_level: number;    // 可用电量百分比
  range_km: number;                // 预估续航 (km)
  ideal_range_km: number;          // 理想续航 (km)

  // 位置
  latitude: number;
  longitude: number;
  heading: number;                 // 航向角 (0-360)
  speed: number | null;            // 速度 (km/h)
  power: number;                   // 功率 (kW), 正=耗电, 负=充电
  odometer: number;                // 总里程 (km)

  // 充电
  plugged_in: boolean;             // 是否插电
  charging_state: ChargingState;
  charger_power: number;           // 充电功率 (kW)
  charge_limit_soc: number;        // 充电限制 (%)
  time_to_full_charge: number;     // 充满时间 (小时)
  charger_voltage: number;         // 充电电压 (V)
  charger_current: number;         // 充电电流 (A)

  // 温度
  inside_temp: number | null;      // 车内温度 (°C)
  outside_temp: number | null;     // 车外温度 (°C)
  is_climate_on: boolean;          // 空调是否开启

  // 安全
  locked: boolean;                 // 是否锁车
  sentry_mode: boolean;            // 哨兵模式
  is_user_present: boolean;        // 用户是否在场

  // 开关状态
  doors_open: boolean;             // 任意车门打开
  windows_open: boolean;           // 任意窗户打开
  frunk_open: boolean;             // 前备箱打开
  trunk_open: boolean;             // 后备箱打开

  // 胎压 (bar)
  tpms_pressure_fl: number;        // 左前
  tpms_pressure_fr: number;        // 右前
  tpms_pressure_rl: number;        // 左后
  tpms_pressure_rr: number;        // 右后

  // 系统
  car_version: string;             // 软件版本
  last_updated: string;            // 最后更新时间 (ISO 8601)
}

// 行程记录
interface Drive {
  id: number;
  car_id: number;
  start_time: string;
  end_time: string | null;
  duration_min: number;
  start_battery_level: number;
  end_battery_level: number | null;
  start_range_km: number;
  end_range_km: number | null;
  distance_km: number;
  avg_speed_kmh: number;
  max_speed_kmh: number;
}

// 轨迹点
interface Position {
  id: number;
  car_id: number;
  drive_id: number | null;
  recorded_at: string;
  latitude: number;
  longitude: number;
  heading: number;
  speed: number | null;
  power: number;
  battery_level: number;
  range_km: number;
  odometer: number;
  inside_temp: number | null;
  outside_temp: number | null;
  tpms_pressure_fl: number;
  tpms_pressure_fr: number;
  tpms_pressure_rl: number;
  tpms_pressure_rr: number;
}

// 充电记录
interface ChargingProcess {
  id: number;
  car_id: number;
  start_time: string;
  end_time: string | null;
  duration_min: number;
  start_battery_level: number;
  end_battery_level: number | null;
  start_range_km: number;
  end_range_km: number | null;
  charge_energy_added: number;     // 充入电量 (kWh)
}

// 分页响应
interface PaginatedResponse<T> {
  data: T[];
  pagination: {
    page: number;
    per_page: number;
    total: number;
  };
}

// WebSocket 消息
interface WebSocketMessage {
  type: 'state_update' | string;
  data: VehicleStateData | any;
}
```

---

## 配置参数参考

| 参数 | 默认值 | 说明 |
|------|--------|------|
| POLL_INTERVAL_ONLINE | 10s | 在线状态轮询间隔 |
| POLL_INTERVAL_DRIVING | 3s | 行驶状态轮询间隔 |
| POLL_INTERVAL_CHARGING | 30s | 充电状态轮询间隔 |
| SUSPEND_AFTER_IDLE_MIN | 15 | 空闲多久后暂停 (分钟) |
| SUSPEND_POLL_INTERVAL | 21m | 暂停状态轮询间隔 |

---

## 错误处理

### HTTP 错误码

| 状态码 | 含义 |
|--------|------|
| 200 | 成功 |
| 400 | 请求参数错误 |
| 404 | 资源不存在 |
| 500 | 服务器内部错误 |

### 错误响应格式

```json
{
  "error": "错误描述信息"
}
```

---

## 更新日志

- **2024-01-07**: 初始版本，包含完整 API 和 WebSocket 文档
