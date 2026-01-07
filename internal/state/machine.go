package state

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/looplab/fsm"
)

// 车辆状态常量
const (
	StateOnline    = "online"
	StateAsleep    = "asleep"
	StateOffline   = "offline"
	StateDriving   = "driving"
	StateCharging  = "charging"
	StateUpdating  = "updating"
	StateSuspended = "suspended" // 暂停日志记录，等待车辆休眠
)

// 事件常量
const (
	EventWakeUp        = "wake_up"
	EventFallAsleep    = "fall_asleep"
	EventGoOffline     = "go_offline"
	EventStartDriving  = "start_driving"
	EventStopDriving   = "stop_driving"
	EventStartCharging = "start_charging"
	EventStopCharging  = "stop_charging"
	EventStartUpdating = "start_updating"
	EventStopUpdating  = "stop_updating"
	EventSuspend       = "suspend"        // 暂停日志
	EventResume        = "resume"         // 恢复日志
)

// VehicleState 车辆状态
type VehicleState struct {
	CarID         int64     `json:"car_id"`
	CurrentState  string    `json:"state"`
	Since         time.Time `json:"since"`
	LastUsed      time.Time `json:"last_used"`      // 最后活跃时间 (用于自动休眠判断)
	BatteryLevel  int       `json:"battery_level"`
	RangeKm       float64   `json:"range_km"`
	Latitude      float64   `json:"latitude"`
	Longitude     float64   `json:"longitude"`
	Speed         *int      `json:"speed"`
	Power         int       `json:"power"`
	InsideTemp    *float64  `json:"inside_temp"`
	OutsideTemp   *float64  `json:"outside_temp"`
	Locked        bool      `json:"locked"`
	SentryMode    bool      `json:"sentry_mode"`
	PluggedIn     bool      `json:"plugged_in"`
	ChargingState string    `json:"charging_state"`
	ChargerPower  int       `json:"charger_power"`
	// TPMS 胎压数据 (bar)
	TpmsPressureFL *float64 `json:"tpms_pressure_fl,omitempty"` // 左前
	TpmsPressureFR *float64 `json:"tpms_pressure_fr,omitempty"` // 右前
	TpmsPressureRL *float64 `json:"tpms_pressure_rl,omitempty"` // 左后
	TpmsPressureRR *float64 `json:"tpms_pressure_rr,omitempty"` // 右后
	// 新增字段
	Odometer           float64 `json:"odometer_km"`            // 里程 (km)
	CarVersion         string  `json:"car_version"`            // 软件版本
	Heading            int     `json:"heading"`                // 航向角
	DoorsOpen          bool    `json:"doors_open"`             // 是否有门打开
	WindowsOpen        bool    `json:"windows_open"`           // 是否有窗打开
	FrunkOpen          bool    `json:"frunk_open"`             // 前备箱状态
	TrunkOpen          bool    `json:"trunk_open"`             // 后备箱状态
	IsUserPresent      bool    `json:"is_user_present"`        // 用户在场
	IsClimateOn        bool    `json:"is_climate_on"`          // 空调开启
	IsPreconditioning  bool    `json:"is_preconditioning"`     // 预热/预冷中
	ChargeLimitSoc     int     `json:"charge_limit_soc"`       // 充电限制百分比
	TimeToFullCharge   float64 `json:"time_to_full_charge"`    // 充满所需时间 (小时)
	ChargerVoltage     int     `json:"charger_voltage"`        // 充电电压
	ChargerCurrent     int     `json:"charger_current"`        // 充电电流
	UsableBatteryLevel int     `json:"usable_battery_level"`   // 可用电量
	IdealRangeKm       float64 `json:"ideal_range_km"`         // 理想续航 (km)
}

// Machine 车辆状态机
type Machine struct {
	mu            sync.RWMutex
	carID         int64
	fsm           *fsm.FSM
	state         *VehicleState
	onStateChange func(carID int64, from, to string)
}

// NewMachine 创建状态机
func NewMachine(carID int64, initialState string, onStateChange func(carID int64, from, to string)) *Machine {
	if initialState == "" {
		initialState = StateOffline
	}

	m := &Machine{
		carID:         carID,
		onStateChange: onStateChange,
		state: &VehicleState{
			CarID:        carID,
			CurrentState: initialState,
			Since:        time.Now(),
			LastUsed:     time.Now(),
		},
	}

	m.fsm = fsm.NewFSM(
		initialState,
		fsm.Events{
			// 从 offline 状态
			{Name: EventWakeUp, Src: []string{StateOffline, StateAsleep}, Dst: StateOnline},

			// 从 online 状态
			{Name: EventFallAsleep, Src: []string{StateOnline}, Dst: StateAsleep},
			{Name: EventGoOffline, Src: []string{StateOnline, StateAsleep}, Dst: StateOffline},
			{Name: EventStartDriving, Src: []string{StateOnline}, Dst: StateDriving},
			{Name: EventStartCharging, Src: []string{StateOnline}, Dst: StateCharging},
			{Name: EventStartUpdating, Src: []string{StateOnline}, Dst: StateUpdating},
			{Name: EventSuspend, Src: []string{StateOnline}, Dst: StateSuspended},

			// 从 driving 状态
			{Name: EventStopDriving, Src: []string{StateDriving}, Dst: StateOnline},

			// 从 charging 状态
			{Name: EventStopCharging, Src: []string{StateCharging}, Dst: StateOnline},

			// 从 updating 状态
			{Name: EventStopUpdating, Src: []string{StateUpdating}, Dst: StateOnline},

			// 从 suspended 状态
			{Name: EventResume, Src: []string{StateSuspended}, Dst: StateOnline},
			{Name: EventFallAsleep, Src: []string{StateSuspended}, Dst: StateAsleep},
			{Name: EventGoOffline, Src: []string{StateSuspended}, Dst: StateOffline},
			// suspended 状态下如果检测到驾驶/充电，需要恢复
			{Name: EventStartDriving, Src: []string{StateSuspended}, Dst: StateDriving},
			{Name: EventStartCharging, Src: []string{StateSuspended}, Dst: StateCharging},
		},
		fsm.Callbacks{
			"after_event": func(ctx context.Context, e *fsm.Event) {
				if m.onStateChange != nil && e.Src != e.Dst {
					m.onStateChange(m.carID, e.Src, e.Dst)
				}
			},
		},
	)

	return m
}

// CurrentState 获取当前状态
func (m *Machine) CurrentState() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.fsm.Current()
}

// GetState 获取完整状态
func (m *Machine) GetState() *VehicleState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// 返回副本
	stateCopy := *m.state
	stateCopy.CurrentState = m.fsm.Current()
	return &stateCopy
}

// UpdateState 更新状态数据
func (m *Machine) UpdateState(update func(s *VehicleState)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	update(m.state)
}

// Trigger 触发事件
func (m *Machine) Trigger(event string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.fsm.Event(context.Background(), event); err != nil {
		return fmt.Errorf("trigger event %s: %w", event, err)
	}

	m.state.CurrentState = m.fsm.Current()
	m.state.Since = time.Now()
	return nil
}

// CanTransition 检查是否可以转换
func (m *Machine) CanTransition(event string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.fsm.Can(event)
}

// Manager 状态机管理器
type Manager struct {
	mu       sync.RWMutex
	machines map[int64]*Machine
	onChange func(carID int64, from, to string)
}

// NewManager 创建管理器
func NewManager(onChange func(carID int64, from, to string)) *Manager {
	return &Manager{
		machines: make(map[int64]*Machine),
		onChange: onChange,
	}
}

// GetOrCreate 获取或创建状态机
func (m *Manager) GetOrCreate(carID int64, initialState string) *Machine {
	m.mu.Lock()
	defer m.mu.Unlock()

	if machine, ok := m.machines[carID]; ok {
		return machine
	}

	machine := NewMachine(carID, initialState, m.onChange)
	m.machines[carID] = machine
	return machine
}

// Get 获取状态机
func (m *Manager) Get(carID int64) (*Machine, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	machine, ok := m.machines[carID]
	return machine, ok
}

// GetAllStates 获取所有车辆状态
func (m *Manager) GetAllStates() map[int64]*VehicleState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	states := make(map[int64]*VehicleState)
	for carID, machine := range m.machines {
		states[carID] = machine.GetState()
	}
	return states
}
