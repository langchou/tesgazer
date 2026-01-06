# TeslaMate 实时数据推送架构分析

> 基于 TeslaMate 源码分析，用于指导 Go 版本实现

## 一、整体架构概览

TeslaMate 采用**多层实时数据推送架构**：

```
Tesla API (轮询)
    ↓
Vehicle GenStateMachine (状态机)
    ↓
Vehicle.Summary 结构体
    ↓
Phoenix.PubSub 广播
    ↓
多个订阅者：
├─ WebSocket Channel (前端实时更新)
├─ LiveView (服务端渲染)
└─ MQTT 发布器 (可选，用于 Home Assistant 等集成)
```

## 二、核心组件

### 2.1 GenStateMachine (车辆状态机)

**文件位置**: `lib/teslamate/vehicles/vehicle.ex`

**状态定义**:
- `:start` - 初始状态
- `:online` - 在线
- `:offline` - 离线
- `:asleep` - 睡眠
- `:driving` - 驾驶中
- `:charging` - 充电中
- `:updating` - 软件更新中
- `:suspended` - 暂停轮询

**动态轮询间隔**:
```elixir
驾驶中 → 2.5秒
充电中 → 5秒
在线   → 15秒 (默认)
睡眠   → 30秒
离线   → 60秒
```

### 2.2 Summary 数据结构

**文件位置**: `lib/teslamate/vehicles/vehicle/summary.ex`

包含 50+ 字段的完整车辆状态：

```elixir
defstruct ~w(
  car display_name state since healthy
  latitude longitude heading
  battery_level charging_state usable_battery_level
  ideal_battery_range_km est_battery_range_km rated_battery_range_km
  charge_energy_added speed outside_temp inside_temp
  is_climate_on is_preconditioning locked sentry_mode plugged_in
  scheduled_charging_start_time charge_limit_soc charger_power
  windows_open doors_open odometer shift_state
  charge_port_door_open time_to_full_charge
  charger_phases charger_actual_current charger_voltage
  version update_available update_version
  is_user_present geofence model trim_badging
  exterior_color wheel_type spoiler_type
  trunk_open frunk_open elevation power
  charge_current_request charge_current_request_max
  tpms_pressure_fl tpms_pressure_fr tpms_pressure_rl tpms_pressure_rr
  ...更多字段
)a
```

### 2.3 PubSub 广播

**主题格式**: `TeslaMate.Vehicles.Vehicle/summary/{car_id}`

**广播时机**:
- 每次轮询获取到新数据后
- 车辆状态转换时
- 驾驶/充电开始或结束时

**广播代码**:
```elixir
def handle_event(:internal, :broadcast_summary, state, %Data{last_response: vehicle} = data) do
  payload = Summary.into(vehicle, %{
    state: state,
    since: data.last_state_change,
    healthy?: healthy?(data.car.id),
    elevation: data.elevation,
    geofence: data.geofence,
    car: data.car
  })

  :ok = Phoenix.PubSub.broadcast(TeslaMate.PubSub, summary_topic(data.car.id), payload)
  :keep_state_and_data
end

defp summary_topic(car_id), do: "#{__MODULE__}/summary/#{car_id}"
```

## 三、WebSocket Channel 实现

### 3.1 Socket 配置

**文件位置**: `lib/teslamate_web/channels/user_socket.ex`

```elixir
defmodule TeslaMateWeb.UserSocket do
  use Phoenix.Socket

  # 频道路由：vehicle:summary:* 匹配所有车辆
  channel "vehicle:summary:*", TeslaMateWeb.DashboardChannel

  def connect(_params, socket, _connect_info) do
    {:ok, socket}  # 可添加认证逻辑
  end
end
```

### 3.2 DashboardChannel

**文件位置**: `lib/teslamate_web/channels/dashboard_channel.ex`

**加入频道流程**:
```elixir
def join("vehicle:summary:" <> car_id_str, _params, socket) do
  case Integer.parse(car_id_str) do
    {car_id, ""} ->
      # 1. 订阅 PubSub
      :ok = Vehicle.subscribe_to_summary(car_id)

      # 2. 获取当前状态
      summary = Vehicle.summary(car_id)
      formatted_summary = format_summary(summary, car_id)

      # 3. 返回当前状态给客户端
      {:ok, formatted_summary, assign(socket, :car_id, car_id)}

    _ ->
      {:error, %{reason: "invalid car_id"}}
  end
end
```

**处理 PubSub 消息并推送**:
```elixir
def handle_info(%Summary{} = summary, socket) do
  formatted_summary = format_summary(summary, socket.assigns.car_id)
  push(socket, "summary_update", formatted_summary)  # 推送事件
  {:noreply, socket}
end
```

**数据格式化**:
```elixir
defp format_summary(%Summary{} = summary, car_id) do
  %{
    car: %{
      id: summary.car.id,
      name: summary.car.name,
      vin: summary.car.vin,
      model: summary.model,
      battery_level: summary.battery_level,
      est_range_km: decimal_to_float(summary.est_battery_range_km),
      latitude: decimal_to_float(summary.latitude),
      longitude: decimal_to_float(summary.longitude),
      heading: summary.heading,
      speed: summary.speed,
      power: summary.power,
      shift_state: summary.shift_state,
      since: format_datetime(summary.since),
      healthy: summary.healthy,
      charging_state: summary.charging_state,
      # ... 更多字段
    }
  }
end
```

## 四、前端 JavaScript 实现

### 4.1 Phoenix Socket 连接

```javascript
import { Socket } from "phoenix";

// 创建 Socket 连接
let socket = new Socket("/socket", { params: { token: window.userToken } });
socket.connect();

// 加入车辆频道
let channel = socket.channel("vehicle:summary:123", {});

channel.join()
  .receive("ok", (initialData) => {
    console.log("已连接，初始数据:", initialData);
    updateUI(initialData);
  })
  .receive("error", (resp) => {
    console.log("连接失败:", resp);
  });

// 监听状态更新
channel.on("summary_update", (payload) => {
  console.log("状态更新:", payload);
  updateUI(payload);
});
```

## 五、MQTT 发布 (可选)

**文件位置**: `lib/teslamate/mqtt/pubsub/vehicle_subscriber.ex`

**主题格式**: `teslamate/cars/{car_id}/{field}`

**示例主题**:
```
teslamate/cars/123/state           → "driving"
teslamate/cars/123/battery_level   → "85"
teslamate/cars/123/latitude        → "40.7128"
teslamate/cars/123/longitude       → "-74.0060"
teslamate/cars/123/geofence        → "Home"
```

**智能发布优化**:
- 仅发送变化的字段（差分发布）
- 异步发布，最多 10 个并发
- 某些字段设置 MQTT retain 标志

## 六、消息流时序

```
T1: Tesla API 轮询
    Vehicle.GenStateMachine.fetch()
    ↓
T2: 状态更新
    GenStateMachine 更新内部状态
    ↓
T3: 创建 Summary
    Summary.into(vehicle, context)
    ↓
T4: 广播 Summary
    Phoenix.PubSub.broadcast("Vehicle/summary/123", %Summary{})
    ↓ (并行)
    ├─ DashboardChannel 接收
    │  → format_summary()
    │  → push(socket, "summary_update", data)
    │  → 前端 WebSocket 接收
    │
    ├─ VehicleSubscriber 接收 (MQTT)
    │  → publish_values()
    │  → MQTT Broker
    │
    └─ LiveView 组件接收
       → handle_info()
       → 自动重新渲染
```

## 七、架构图

```
┌─────────────────────────────────────────────────────────────┐
│                      前端 (Web Browser)                      │
│  ┌─────────────────────┐  ┌─────────────────────────────┐   │
│  │  LiveView 组件       │  │  WebSocket Channel          │   │
│  │  (服务端渲染)        │  │  vehicle:summary:{car_id}   │   │
│  │  - 自动重新渲染      │  │  - summary_update 事件      │   │
│  └─────────────────────┘  └─────────────────────────────┘   │
└────────────────────────────┬────────────────────────────────┘
                             │ WebSocket
                             ↓
┌─────────────────────────────────────────────────────────────┐
│                  Phoenix Framework                           │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  Endpoint.Socket                                     │    │
│  │  - /socket (WebSocket)                               │    │
│  │  - /live (LiveView)                                  │    │
│  └─────────────────────────────────────────────────────┘    │
└────────────────────────────┬────────────────────────────────┘
                             │
                             ↓
┌─────────────────────────────────────────────────────────────┐
│              Phoenix.PubSub (进程间通信)                     │
│  主题: "TeslaMate.Vehicles.Vehicle/summary/{car_id}"        │
│  消息: %Summary{...}                                         │
└────────────────────────────┬────────────────────────────────┘
                             │ 发布
                             ↓
┌─────────────────────────────────────────────────────────────┐
│          Vehicle GenStateMachine (每辆车一个进程)            │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  状态: :online / :driving / :charging / :asleep     │    │
│  │  - 定期轮询 Tesla API                                │    │
│  │  - 管理状态转换                                      │    │
│  │  - 存储位置、驾驶、充电数据                          │    │
│  │  - 计算 Summary 并广播                               │    │
│  └─────────────────────────────────────────────────────┘    │
└────────────────────────────┬────────────────────────────────┘
                             │ HTTP 轮询 (2.5s - 60s)
                             ↓
┌─────────────────────────────────────────────────────────────┐
│                      Tesla API                               │
│  /api/1/vehicles/{id}/vehicle_data                          │
└─────────────────────────────────────────────────────────────┘
```

## 八、Go 版本实现建议

### 8.1 简化版本 (当前场景：单用户自部署)

由于是单用户场景，可以简化为：

1. **WebSocket 连接后自动推送所有车辆数据**
2. **不需要订阅特定车辆**
3. **任何车辆状态更新都广播给所有客户端**

```
前端连接 WebSocket
    ↓
收到 init 消息: { type: "init", cars: [...], states: {...} }
    ↓
持续收到更新: { type: "state_update", car_id: 1, state: {...} }
```

### 8.2 完整版本 (未来多用户场景)

如果未来支持多用户：

1. **WebSocket 连接时需要认证**
2. **按用户隔离数据**
3. **可以保留按车辆订阅的机制**

### 8.3 关键实现点

| 组件 | TeslaMate (Elixir) | Go 版本 |
|------|-------------------|---------|
| 状态机 | GenStateMachine | looplab/fsm |
| 进程间通信 | Phoenix.PubSub | Go channel 或 内存 PubSub |
| WebSocket | Phoenix Channel | gorilla/websocket |
| 定时轮询 | Process.send_after | time.Ticker |
| 数据广播 | PubSub.broadcast | Hub.Broadcast |

## 九、参考文件

| 功能 | 文件路径 |
|------|---------|
| 状态机 | lib/teslamate/vehicles/vehicle.ex |
| Summary 结构 | lib/teslamate/vehicles/vehicle/summary.ex |
| WebSocket Socket | lib/teslamate_web/channels/user_socket.ex |
| Dashboard Channel | lib/teslamate_web/channels/dashboard_channel.ex |
| Endpoint 配置 | lib/teslamate_web/endpoint.ex |
| MQTT 发布 | lib/teslamate/mqtt/pubsub/vehicle_subscriber.ex |
| 前端 Socket | assets/js/socket.js |
| 前端入口 | assets/js/app.js |
