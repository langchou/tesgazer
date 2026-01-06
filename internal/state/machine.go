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
	StateOnline   = "online"
	StateAsleep   = "asleep"
	StateOffline  = "offline"
	StateDriving  = "driving"
	StateCharging = "charging"
	StateUpdating = "updating"
)

// 事件常量
const (
	EventWakeUp       = "wake_up"
	EventFallAsleep   = "fall_asleep"
	EventGoOffline    = "go_offline"
	EventStartDriving = "start_driving"
	EventStopDriving  = "stop_driving"
	EventStartCharging = "start_charging"
	EventStopCharging = "stop_charging"
	EventStartUpdating = "start_updating"
	EventStopUpdating = "stop_updating"
)

// VehicleState 车辆状态
type VehicleState struct {
	CarID         int64     `json:"car_id"`
	CurrentState  string    `json:"state"`
	Since         time.Time `json:"since"`
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

			// 从 driving 状态
			{Name: EventStopDriving, Src: []string{StateDriving}, Dst: StateOnline},

			// 从 charging 状态
			{Name: EventStopCharging, Src: []string{StateCharging}, Dst: StateOnline},

			// 从 updating 状态
			{Name: EventStopUpdating, Src: []string{StateUpdating}, Dst: StateOnline},
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
