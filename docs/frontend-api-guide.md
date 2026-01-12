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
| GET | `/api/cars/:id/footprint` | 获取足迹数据（默认90天） |

### 充电相关

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/cars/:id/charges` | 获取充电记录列表（分页） |
| GET | `/api/charges/:id` | 获取充电详情 |
| GET | `/api/charges/:id/details` | 获取充电曲线数据 |

### 停车相关

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/cars/:id/parkings` | 获取停车记录列表（分页） |
| GET | `/api/parkings/:id` | 获取停车详情 |

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
      "start_odometer_km": 12300.0,
      "end_odometer_km": 12325.5,
      "distance_km": 25.5,
      "speed_max": 120,
      "power_max": 150,
      "power_min": -50,
      "inside_temp_avg": 22.5,
      "outside_temp_avg": 15.0,
      "energy_used_kwh": 4.5,
      "energy_regen_kwh": 1.2
    }
  ],
  "pagination": {
    "page": 1,
    "per_page": 20,
    "total": 100
  }
}
```

#### Drive 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | int64 | 行程 ID |
| `car_id` | int64 | 车辆 ID |
| `start_time` | string | 开始时间 (ISO8601) |
| `end_time` | string | 结束时间 (ISO8601) |
| `duration_min` | float64 | 行程时长 (分钟) |
| `start_battery_level` | int | 起始电量 (%) |
| `end_battery_level` | int | 结束电量 (%) |
| `start_range_km` | float64 | 起始续航 (km) |
| `end_range_km` | float64 | 结束续航 (km) |
| `start_odometer_km` | float64 | 起始里程表 (km) |
| `end_odometer_km` | float64 | 结束里程表 (km) |
| `distance_km` | float64 | 行驶距离 (km) |
| `speed_max` | int | 最高速度 (km/h) |
| `power_max` | int | 最大功率 (kW，正值=耗电) |
| `power_min` | int | 最小功率 (kW，负值=回收) |
| `inside_temp_avg` | float64 | 平均车内温度 (°C) |
| `outside_temp_avg` | float64 | 平均车外温度 (°C) |
| `energy_used_kwh` | float64 | 总耗电量 (kWh) |
| `energy_regen_kwh` | float64 | 动能回收电量 (kWh) |

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

### GET /api/cars/:id/footprint

获取车辆足迹数据（所有行程的起止点）。

**查询参数**:
| 参数 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| days | int | 90 | 查询最近多少天的数据 |

**响应示例**:
```json
{
  "data": [
    {
      "drive_id": 1,
      "start_time": "2024-01-07T10:00:00Z",
      "end_time": "2024-01-07T10:30:00Z",
      "start_lat": 31.2304,
      "start_lng": 121.4737,
      "end_lat": 31.2500,
      "end_lng": 121.5000,
      "distance_km": 25.5,
      "duration_min": 30.5
    }
  ]
}
```

#### Footprint 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `drive_id` | int64 | 行程 ID |
| `start_time` | string | 开始时间 (ISO8601) |
| `end_time` | string | 结束时间 (ISO8601) |
| `start_lat` | float64 | 起点纬度 |
| `start_lng` | float64 | 起点经度 |
| `end_lat` | float64 | 终点纬度 |
| `end_lng` | float64 | 终点经度 |
| `distance_km` | float64 | 行驶距离 (km) |
| `duration_min` | float64 | 行程时长 (分钟) |

### GET /api/cars/:id/parkings

获取停车记录列表（分页）。

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
      "start_time": "2024-01-07T10:30:00Z",
      "end_time": "2024-01-07T14:00:00Z",
      "duration_min": 210.0,
      "latitude": 31.2304,
      "longitude": 121.4737,
      "start_battery_level": 70,
      "end_battery_level": 68,
      "start_range_km": 300.0,
      "end_range_km": 290.0,
      "start_odometer": 12325.5,
      "end_odometer": 12325.5,
      "energy_used_kwh": 1.5,
      "start_inside_temp": 22.5,
      "end_inside_temp": 35.0,
      "start_outside_temp": 25.0,
      "end_outside_temp": 32.0,
      "inside_temp_avg": 28.0,
      "outside_temp_avg": 28.5,
      "climate_used_min": 15.0,
      "sentry_mode_used_min": 180.0,
      "start_locked": true,
      "start_sentry_mode": true,
      "start_doors_open": false,
      "start_windows_open": false,
      "start_frunk_open": false,
      "start_trunk_open": false,
      "start_is_climate_on": false,
      "start_is_user_present": false,
      "end_locked": true,
      "end_sentry_mode": true,
      "end_doors_open": false,
      "end_windows_open": false,
      "end_frunk_open": false,
      "end_trunk_open": false,
      "end_is_climate_on": false,
      "end_is_user_present": false,
      "start_tpms_pressure_fl": 2.9,
      "start_tpms_pressure_fr": 2.9,
      "start_tpms_pressure_rl": 2.9,
      "start_tpms_pressure_rr": 2.9,
      "end_tpms_pressure_fl": 3.0,
      "end_tpms_pressure_fr": 3.0,
      "end_tpms_pressure_rl": 3.0,
      "end_tpms_pressure_rr": 3.0,
      "car_version": "2024.26.9"
    }
  ],
  "pagination": {
    "page": 1,
    "per_page": 20,
    "total": 100
  }
}
```

#### Parking 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | int64 | 停车记录 ID |
| `car_id` | int64 | 车辆 ID |
| `start_time` | string | 开始时间 (ISO8601) |
| `end_time` | string | 结束时间 (ISO8601) |
| `duration_min` | float64 | 停车时长 (分钟) |
| `latitude` | float64 | 停车位置纬度 |
| `longitude` | float64 | 停车位置经度 |
| `start_battery_level` | int | 起始电量 (%) |
| `end_battery_level` | int | 结束电量 (%) |
| `start_range_km` | float64 | 起始续航 (km) |
| `end_range_km` | float64 | 结束续航 (km) |
| `start_odometer` | float64 | 起始里程表 (km) |
| `end_odometer` | float64 | 结束里程表 (km) |
| `energy_used_kwh` | float64 | 停车期间电量消耗/吸血鬼功耗 (kWh) |
| `start_inside_temp` | float64 | 起始车内温度 (°C) |
| `end_inside_temp` | float64 | 结束车内温度 (°C) |
| `start_outside_temp` | float64 | 起始车外温度 (°C) |
| `end_outside_temp` | float64 | 结束车外温度 (°C) |
| `inside_temp_avg` | float64 | 停车期间平均车内温度 (°C) |
| `outside_temp_avg` | float64 | 停车期间平均车外温度 (°C) |
| `climate_used_min` | float64 | 空调使用时长 (分钟) |
| `sentry_mode_used_min` | float64 | 哨兵模式使用时长 (分钟) |
| `start_locked` | bool | 起始锁车状态 |
| `start_sentry_mode` | bool | 起始哨兵模式状态 |
| `start_doors_open` | bool | 起始是否有车门打开 |
| `start_windows_open` | bool | 起始是否有车窗打开 |
| `start_frunk_open` | bool | 起始前备箱是否打开 |
| `start_trunk_open` | bool | 起始后备箱是否打开 |
| `start_is_climate_on` | bool | 起始空调是否开启 |
| `start_is_user_present` | bool | 起始是否有用户在场 |
| `end_locked` | bool | 结束锁车状态 |
| `end_sentry_mode` | bool | 结束哨兵模式状态 |
| `end_doors_open` | bool | 结束是否有车门打开 |
| `end_windows_open` | bool | 结束是否有车窗打开 |
| `end_frunk_open` | bool | 结束前备箱是否打开 |
| `end_trunk_open` | bool | 结束后备箱是否打开 |
| `end_is_climate_on` | bool | 结束空调是否开启 |
| `end_is_user_present` | bool | 结束是否有用户在场 |
| `start_tpms_pressure_fl` | float64 | 起始左前胎压 (bar) |
| `start_tpms_pressure_fr` | float64 | 起始右前胎压 (bar) |
| `start_tpms_pressure_rl` | float64 | 起始左后胎压 (bar) |
| `start_tpms_pressure_rr` | float64 | 起始右后胎压 (bar) |
| `end_tpms_pressure_fl` | float64 | 结束左前胎压 (bar) |
| `end_tpms_pressure_fr` | float64 | 结束右前胎压 (bar) |
| `end_tpms_pressure_rl` | float64 | 结束左后胎压 (bar) |
| `end_tpms_pressure_rr` | float64 | 结束右后胎压 (bar) |
| `car_version` | string | 软件版本 |

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

#### 1. `init` - 初始化数据

客户端连接时立即推送，包含车辆列表和所有车辆的当前状态。

```json
{
  "type": "init",
  "data": {
    "cars": [
      {
        "id": 1,
        "tesla_id": 1234567890,
        "vin": "LRW3E7EK1NC123456",
        "name": "我的特斯拉",
        "model": "model3"
      }
    ],
    "states": {
      "1": {
        "car_id": 1,
        "state": "online",
        "battery_level": 67,
        "latitude": 31.2304,
        "longitude": 121.4737
      }
    }
  }
}
```

#### 2. `state_update` - 车辆状态更新

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

// 停车记录
interface Parking {
  id: number;
  car_id: number;
  position_id: number | null;
  geofence_id: number | null;
  start_time: string;
  end_time: string | null;
  duration_min: number;

  // 位置
  latitude: number;
  longitude: number;

  // 电量变化
  start_battery_level: number;
  end_battery_level: number | null;
  start_range_km: number;
  end_range_km: number | null;
  start_odometer: number;
  end_odometer: number | null;
  energy_used_kwh: number | null;   // 吸血鬼功耗 (kWh)

  // 温度
  start_inside_temp: number | null;
  end_inside_temp: number | null;
  start_outside_temp: number | null;
  end_outside_temp: number | null;
  inside_temp_avg: number | null;
  outside_temp_avg: number | null;

  // 使用时长统计
  climate_used_min: number | null;       // 空调使用时长 (分钟)
  sentry_mode_used_min: number | null;   // 哨兵模式使用时长 (分钟)

  // 起始状态快照
  start_locked: boolean;
  start_sentry_mode: boolean;
  start_doors_open: boolean;
  start_windows_open: boolean;
  start_frunk_open: boolean;
  start_trunk_open: boolean;
  start_is_climate_on: boolean;
  start_is_user_present: boolean;

  // 结束状态快照
  end_locked: boolean | null;
  end_sentry_mode: boolean | null;
  end_doors_open: boolean | null;
  end_windows_open: boolean | null;
  end_frunk_open: boolean | null;
  end_trunk_open: boolean | null;
  end_is_climate_on: boolean | null;
  end_is_user_present: boolean | null;

  // 胎压 (开始)
  start_tpms_pressure_fl: number | null;
  start_tpms_pressure_fr: number | null;
  start_tpms_pressure_rl: number | null;
  start_tpms_pressure_rr: number | null;

  // 胎压 (结束)
  end_tpms_pressure_fl: number | null;
  end_tpms_pressure_fr: number | null;
  end_tpms_pressure_rl: number | null;
  end_tpms_pressure_rr: number | null;

  // 软件版本
  car_version: string;
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

### 基础配置

| 参数 | 默认值 | 说明 |
|------|--------|------|
| PORT | 4000 | 服务端口 |
| DATABASE_URL | — | PostgreSQL 连接地址 |
| DEBUG | false | 调试模式 |

### 轮询间隔

| 参数 | 默认值 | 说明 |
|------|--------|------|
| POLL_INTERVAL_ONLINE | 15s | 在线状态轮询间隔 |
| POLL_INTERVAL_DRIVING | 3s | 行驶状态轮询间隔 |
| POLL_INTERVAL_CHARGING | 5s | 充电状态轮询间隔 |
| POLL_INTERVAL_ASLEEP | 30s | 睡眠状态轮询间隔 |
| POLL_BACKOFF_INITIAL | 1s | 初始退避间隔 |
| POLL_BACKOFF_MAX | 30s | 最大退避间隔 |
| POLL_BACKOFF_FACTOR | 2.0 | 退避因子 |

### 休眠控制

| 参数 | 默认值 | 说明 |
|------|--------|------|
| SUSPEND_AFTER_IDLE_MIN | 15 | 空闲多久后暂停 (分钟) |
| SUSPEND_POLL_INTERVAL | 21m | 暂停状态轮询间隔 |
| REQUIRE_NOT_UNLOCKED | false | 是否要求上锁才能休眠 |

### Streaming API

| 参数 | 默认值 | 说明 |
|------|--------|------|
| USE_STREAMING_API | true | 是否启用 Streaming API |
| STREAMING_HOST | wss://streaming.vn.cloud.tesla.cn/streaming/ | Streaming WebSocket 地址 |
| STREAMING_RECONNECT_DELAY | 5s | 重连延迟 |

### 可选配置

| 参数 | 默认值 | 说明 |
|------|--------|------|
| AMAP_API_KEY | — | 高德地图 API Key（逆地理编码） |
| TOKEN_FILE | tokens.json | Token 存储文件 |

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

- **2025-01-09**:
  - **新增 Footprint API**: `/api/cars/:id/footprint` 获取车辆足迹数据（行程起止点）
  - **配置参数完善**: 补充所有配置参数文档，包括 Streaming API、退避算法、高德地图等
  - **修正默认值**: 更正轮询间隔默认值（POLL_INTERVAL_ONLINE: 15s, POLL_INTERVAL_CHARGING: 5s）
- **2025-01-08**:
  - **Drive API 增强**:
    - 新增 `start_odometer_km` / `end_odometer_km`: 起止里程表读数
    - 新增 `speed_max`: 最高速度 (km/h)
    - 新增 `power_max` / `power_min`: 最大/最小功率 (kW)
    - 新增 `inside_temp_avg` / `outside_temp_avg`: 平均车内/外温度
    - 新增 `energy_used_kwh`: 总耗电量 (kWh)
    - 新增 `energy_regen_kwh`: 动能回收电量 (kWh)
    - 移除旧字段 `avg_speed_kmh` / `max_speed_kmh`（用 `speed_max` 替代）
  - **Position 记录关联**: 驾驶时的位置记录现在正确关联到对应的行程，`/api/drives/:id/positions` 返回完整轨迹
  - **历史数据修复**: 数据库迁移会自动修复历史行程的统计数据
- **2025-01-07**: 初始版本，包含完整 API 和 WebSocket 文档
