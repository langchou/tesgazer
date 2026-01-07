package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/langchou/tesgazer/internal/api/tesla"
	"github.com/langchou/tesgazer/internal/config"
	"github.com/langchou/tesgazer/internal/models"
	"github.com/langchou/tesgazer/internal/repository"
	"github.com/langchou/tesgazer/internal/state"
	"github.com/langchou/tesgazer/pkg/ws"
)

// VehicleService 车辆服务
type VehicleService struct {
	cfg          *config.Config
	logger       *zap.Logger
	teslaClient  *tesla.Client
	carRepo      *repository.CarRepository
	posRepo      *repository.PositionRepository
	driveRepo    *repository.DriveRepository
	chargeRepo   *repository.ChargeRepository
	stateManager *state.Manager
	wsHub        *ws.Hub // WebSocket Hub

	mu          sync.RWMutex
	stopCh      chan struct{}
	wg          sync.WaitGroup
	subscribers []chan *state.VehicleState
	running     bool // 标记服务是否正在运行

	// 指数退避相关状态 (per vehicle)
	pollIntervals  map[int64]time.Duration // 每辆车当前的轮询间隔
	lastPollTimes  map[int64]time.Time     // 每辆车上次轮询时间
	lastUsedTimes  map[int64]time.Time     // 每辆车最后活跃时间 (用于自动休眠)

	// Tesla Streaming API 客户端 (双链路架构)
	streamingClients map[int64]*tesla.StreamingClient // 每辆车的 Streaming 客户端
	streamingCtx     context.Context                  // Streaming 上下文
	streamingCancel  context.CancelFunc               // 取消函数
}

// NewVehicleService 创建车辆服务
func NewVehicleService(
	cfg *config.Config,
	logger *zap.Logger,
	teslaClient *tesla.Client,
	carRepo *repository.CarRepository,
	posRepo *repository.PositionRepository,
	driveRepo *repository.DriveRepository,
	chargeRepo *repository.ChargeRepository,
	wsHub *ws.Hub,
) *VehicleService {
	svc := &VehicleService{
		cfg:              cfg,
		logger:           logger,
		teslaClient:      teslaClient,
		carRepo:          carRepo,
		posRepo:          posRepo,
		driveRepo:        driveRepo,
		chargeRepo:       chargeRepo,
		wsHub:            wsHub,
		stopCh:           make(chan struct{}),
		pollIntervals:    make(map[int64]time.Duration),
		lastPollTimes:    make(map[int64]time.Time),
		lastUsedTimes:    make(map[int64]time.Time),
		streamingClients: make(map[int64]*tesla.StreamingClient),
	}

	// 创建状态管理器
	svc.stateManager = state.NewManager(svc.onStateChange)

	return svc
}

// Start 启动服务
func (s *VehicleService) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		s.logger.Info("Vehicle service already running, skipping start")
		return nil
	}
	// 重新初始化 stopCh（防止重复启动问题）
	s.stopCh = make(chan struct{})
	s.running = true
	s.mu.Unlock()

	s.logger.Info("Starting vehicle service")

	// 同步车辆列表
	if err := s.syncVehicles(ctx); err != nil {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		return fmt.Errorf("sync vehicles: %w", err)
	}

	// 启动轮询
	s.wg.Add(1)
	go s.pollLoop(ctx)

	// 启动 Streaming API（双链路架构）
	if s.cfg.UseStreamingAPI {
		s.startAllStreaming(ctx)
	}

	s.logger.Info("Vehicle service started, polling loop running")
	return nil
}

// Stop 停止服务
func (s *VehicleService) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	s.logger.Info("Stopping vehicle service")

	// 停止 Streaming
	s.stopAllStreaming()

	close(s.stopCh)
	s.wg.Wait()
	s.logger.Info("Vehicle service stopped")
}

// Subscribe 订阅状态更新
func (s *VehicleService) Subscribe() <-chan *state.VehicleState {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan *state.VehicleState, 10)
	s.subscribers = append(s.subscribers, ch)
	return ch
}

// GetState 获取车辆状态
func (s *VehicleService) GetState(carID int64) (*state.VehicleState, bool) {
	machine, ok := s.stateManager.Get(carID)
	if !ok {
		return nil, false
	}
	return machine.GetState(), true
}

// GetAllStates 获取所有车辆状态
func (s *VehicleService) GetAllStates() map[int64]*state.VehicleState {
	return s.stateManager.GetAllStates()
}

// syncVehicles 同步车辆列表
func (s *VehicleService) syncVehicles(ctx context.Context) error {
	vehicles, err := s.teslaClient.ListVehicles(ctx)
	if err != nil {
		return fmt.Errorf("list vehicles from tesla: %w", err)
	}

	for _, v := range vehicles {
		car := &models.Car{
			TeslaID:        v.ID,
			TeslaVehicleID: v.VehicleID,
			VIN:            v.VIN,
			Name:           v.DisplayName,
		}

		if err := s.carRepo.Upsert(ctx, car); err != nil {
			s.logger.Error("Failed to upsert car", zap.Error(err), zap.Int64("tesla_id", v.ID))
			continue
		}

		// 初始化状态机
		s.stateManager.GetOrCreate(car.ID, v.State)
		s.logger.Info("Synced vehicle", zap.String("name", car.Name), zap.String("vin", car.VIN), zap.String("state", v.State))
	}

	return nil
}

// pollLoop 轮询循环 - 实现指数退避策略 (参考 TeslaMate)
// 策略说明:
// - online 状态: 使用 PollIntervalOnline (默认 15s)
// - driving 状态: 使用 PollIntervalDriving (默认 3s)
// - charging 状态: 使用 PollIntervalCharging (默认 5s)
// - asleep/offline 状态: 使用指数退避，从 PollBackoffInitial 开始，每次翻倍直到 PollBackoffMax
func (s *VehicleService) pollLoop(ctx context.Context) {
	defer s.wg.Done()

	// 启动时立即执行一次轮询
	s.logger.Info("Performing initial poll...")
	s.pollAllVehicles(ctx)

	// 使用最小间隔作为基础 ticker
	baseTicker := time.NewTicker(s.cfg.PollBackoffInitial)
	defer baseTicker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		case <-baseTicker.C:
			s.pollAllVehiclesWithBackoff(ctx)
		}
	}
}

// pollAllVehiclesWithBackoff 根据每辆车的状态使用不同的轮询间隔
func (s *VehicleService) pollAllVehiclesWithBackoff(ctx context.Context) {
	cars, err := s.carRepo.List(ctx)
	if err != nil {
		s.logger.Error("Failed to list cars", zap.Error(err))
		return
	}

	now := time.Now()

	for _, car := range cars {
		// 检查该车辆是否应该被轮询
		if !s.shouldPollVehicle(car.ID) {
			continue
		}

		// 获取当前状态，决定使用轻量轮询还是完整轮询
		machine, ok := s.stateManager.Get(car.ID)
		var currentState string
		if ok {
			currentState = machine.CurrentState()
		}

		s.logger.Debug("Polling vehicle with backoff",
			zap.Int64("car_id", car.ID),
			zap.String("name", car.Name),
			zap.String("state", currentState),
			zap.Duration("interval", s.getPollInterval(car.ID)))

		var pollErr error
		// 根据状态选择轮询方式
		// suspended/asleep/offline 状态使用轻量轮询（只查状态，不唤醒）
		if currentState == state.StateSuspended || currentState == state.StateAsleep || currentState == state.StateOffline {
			pollErr = s.pollVehicleLightweight(ctx, car)
		} else {
			pollErr = s.pollVehicle(ctx, car)
		}

		if pollErr != nil {
			s.logger.Error("Failed to poll vehicle", zap.Error(pollErr), zap.Int64("car_id", car.ID))
			// 轮询失败时也应用退避策略
			s.applyBackoff(car.ID)
		}

		// 更新下次轮询时间
		s.updateNextPollTime(car.ID, now)
	}
}

// shouldPollVehicle 检查是否应该轮询该车辆
func (s *VehicleService) shouldPollVehicle(carID int64) bool {
	s.mu.RLock()
	interval, intervalExists := s.pollIntervals[carID]
	lastPoll, lastPollExists := s.lastPollTimes[carID]
	s.mu.RUnlock()

	if !intervalExists || !lastPollExists {
		// 首次轮询
		return true
	}

	// 检查自上次轮询以来是否已过足够时间
	return time.Since(lastPoll) >= interval
}

// getPollInterval 获取车辆当前的轮询间隔
func (s *VehicleService) getPollInterval(carID int64) time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if interval, exists := s.pollIntervals[carID]; exists {
		return interval
	}
	return s.cfg.PollIntervalOnline
}

// updateNextPollTime 根据车辆状态更新轮询间隔
func (s *VehicleService) updateNextPollTime(carID int64, now time.Time) {
	machine, ok := s.stateManager.Get(carID)
	if !ok {
		return
	}

	currentState := machine.CurrentState()
	var newInterval time.Duration

	switch currentState {
	case state.StateDriving:
		// 驾驶中：高频轮询
		newInterval = s.cfg.PollIntervalDriving
		s.logger.Debug("Vehicle driving, using driving interval",
			zap.Int64("car_id", carID),
			zap.Duration("interval", newInterval))

	case state.StateCharging:
		// 充电中：中频轮询
		newInterval = s.cfg.PollIntervalCharging
		s.logger.Debug("Vehicle charging, using charging interval",
			zap.Int64("car_id", carID),
			zap.Duration("interval", newInterval))

	case state.StateSuspended:
		// 暂停日志状态：使用较长的轮询间隔，让车辆有机会休眠
		// 参考 TeslaMate: 默认 21 分钟
		newInterval = s.cfg.SuspendPollInterval
		s.logger.Debug("Vehicle suspended, using suspend poll interval",
			zap.Int64("car_id", carID),
			zap.Duration("interval", newInterval))

	case state.StateAsleep, state.StateOffline:
		// 睡眠/离线：使用指数退避
		newInterval = s.calculateBackoffInterval(carID)
		s.logger.Debug("Vehicle asleep/offline, using backoff interval",
			zap.Int64("car_id", carID),
			zap.Duration("interval", newInterval))

	default:
		// 在线：重置为正常间隔
		newInterval = s.cfg.PollIntervalOnline
		s.logger.Debug("Vehicle online, using online interval",
			zap.Int64("car_id", carID),
			zap.Duration("interval", newInterval))
	}

	s.mu.Lock()
	s.pollIntervals[carID] = newInterval
	s.lastPollTimes[carID] = now
	s.mu.Unlock()
}

// calculateBackoffInterval 计算退避间隔（不修改状态）
func (s *VehicleService) calculateBackoffInterval(carID int64) time.Duration {
	s.mu.RLock()
	currentInterval, exists := s.pollIntervals[carID]
	s.mu.RUnlock()

	if !exists || currentInterval < s.cfg.PollBackoffInitial {
		return s.cfg.PollBackoffInitial
	}

	// 指数退避: interval * factor, 但不超过最大值
	newInterval := time.Duration(float64(currentInterval) * s.cfg.PollBackoffFactor)
	if newInterval > s.cfg.PollBackoffMax {
		newInterval = s.cfg.PollBackoffMax
	}

	return newInterval
}

// applyBackoff 应用指数退避策略
// 返回新的轮询间隔
func (s *VehicleService) applyBackoff(carID int64) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	currentInterval, exists := s.pollIntervals[carID]
	if !exists || currentInterval < s.cfg.PollBackoffInitial {
		currentInterval = s.cfg.PollBackoffInitial
	}

	// 指数退避: interval * factor, 但不超过最大值
	newInterval := time.Duration(float64(currentInterval) * s.cfg.PollBackoffFactor)
	if newInterval > s.cfg.PollBackoffMax {
		newInterval = s.cfg.PollBackoffMax
	}

	s.pollIntervals[carID] = newInterval

	s.logger.Info("Applied exponential backoff",
		zap.Int64("car_id", carID),
		zap.Duration("previous_interval", currentInterval),
		zap.Duration("new_interval", newInterval),
		zap.Duration("max_interval", s.cfg.PollBackoffMax))

	return newInterval
}

// resetBackoff 重置退避（车辆唤醒时调用）
func (s *VehicleService) resetBackoff(carID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pollIntervals[carID] = s.cfg.PollBackoffInitial

	s.logger.Info("Reset backoff interval",
		zap.Int64("car_id", carID),
		zap.Duration("interval", s.cfg.PollBackoffInitial))
}

// pollAllVehicles 轮询所有车辆
func (s *VehicleService) pollAllVehicles(ctx context.Context) {
	cars, err := s.carRepo.List(ctx)
	if err != nil {
		s.logger.Error("Failed to list cars", zap.Error(err))
		return
	}

	s.logger.Info("Polling all vehicles", zap.Int("count", len(cars)))

	for _, car := range cars {
		if err := s.pollVehicle(ctx, car); err != nil {
			s.logger.Error("Failed to poll vehicle", zap.Error(err), zap.Int64("car_id", car.ID))
		} else {
			s.logger.Info("Successfully polled vehicle", zap.Int64("car_id", car.ID), zap.String("name", car.Name))
		}
	}
}

// pollVehicle 轮询单个车辆
func (s *VehicleService) pollVehicle(ctx context.Context, car *models.Car) error {
	machine := s.stateManager.GetOrCreate(car.ID, "")

	// 获取车辆数据
	data, err := s.teslaClient.GetVehicleData(ctx, car.TeslaID)
	if err != nil {
		if err == tesla.ErrVehicleUnavailable {
			// 车辆不可用（可能在睡眠）
			s.transitionToSleepOrOffline(machine, "asleep")
			return nil
		}
		return err
	}

	// 根据 API 返回的 state 字段更新状态机
	s.handleVehicleStateFromAPI(machine, data.State)

	// 更新车辆配置信息（如 model、exterior_color 等）
	if data.VehicleConfig != nil {
		s.updateCarConfig(ctx, car, data.VehicleConfig)
	}

	// 更新状态机数据
	s.updateMachineFromData(machine, data)

	// 记录位置（仅在线时）
	if data.State == "online" && data.DriveState != nil {
		pos := s.createPosition(car.ID, data)
		if err := s.posRepo.Create(ctx, pos); err != nil {
			s.logger.Error("Failed to create position", zap.Error(err))
		}
	}

	// 处理状态变化（驾驶、充电等）
	s.handleStateTransitions(ctx, car, machine, data)

	// 获取最新状态
	currentState := machine.GetState()

	// 通知内部订阅者
	s.notifySubscribers(currentState)

	// 广播到 WebSocket
	s.broadcastState(currentState)

	// 尝试自动暂停（只在 online 状态下检查）
	// 参考 TeslaMate: 空闲一段时间后自动暂停日志，允许车辆进入休眠
	if machine.CurrentState() == state.StateOnline {
		s.tryToSuspend(car.ID, machine, data)
	}

	return nil
}

// pollVehicleLightweight 轻量轮询 - 只检查车辆状态，不唤醒车辆
// 用于 suspended/asleep/offline 状态，避免因轮询导致车辆无法休眠
// 参考 TeslaMate: fetch_with_unreachable_assumption
func (s *VehicleService) pollVehicleLightweight(ctx context.Context, car *models.Car) error {
	machine := s.stateManager.GetOrCreate(car.ID, "")
	currentState := machine.CurrentState()

	// 使用 GetVehicle API（不会唤醒车辆）
	vehicle, err := s.teslaClient.GetVehicle(ctx, car.TeslaID)
	if err != nil {
		s.logger.Debug("Lightweight poll failed",
			zap.Int64("car_id", car.ID),
			zap.Error(err))
		return err
	}

	s.logger.Debug("Lightweight poll result",
		zap.Int64("car_id", car.ID),
		zap.String("api_state", vehicle.State),
		zap.String("current_state", currentState))

	// 根据 API 返回的状态处理
	switch vehicle.State {
	case "online":
		// 车辆自己醒来了！切换到完整轮询获取详细数据
		s.logger.Info("Vehicle woke up during lightweight poll, fetching full data",
			zap.Int64("car_id", car.ID),
			zap.String("from_state", currentState))

		// 立即获取完整数据
		return s.pollVehicle(ctx, car)

	case "asleep":
		// 车辆在睡眠中，更新状态机
		if currentState == state.StateSuspended {
			// 从 suspended 转到 asleep - 车辆成功进入休眠
			if machine.CanTransition(state.EventFallAsleep) {
				machine.Trigger(state.EventFallAsleep)
				s.logger.Info("Vehicle fell asleep (from suspended)",
					zap.Int64("car_id", car.ID))
			}
		} else if currentState != state.StateAsleep {
			s.transitionToSleepOrOffline(machine, "asleep")
		}

	case "offline":
		// 车辆离线
		if currentState == state.StateSuspended {
			// 从 suspended 转到 offline
			if machine.CanTransition(state.EventGoOffline) {
				machine.Trigger(state.EventGoOffline)
				s.logger.Info("Vehicle went offline (from suspended)",
					zap.Int64("car_id", car.ID))
			}
		} else if currentState != state.StateOffline {
			s.transitionToSleepOrOffline(machine, "offline")
		}
	}

	// 广播状态更新
	s.broadcastState(machine.GetState())

	return nil
}

// handleVehicleStateFromAPI 根据 API 返回的 state 字段更新状态机
func (s *VehicleService) handleVehicleStateFromAPI(machine *state.Machine, apiState string) {
	currentState := machine.CurrentState()

	switch apiState {
	case "asleep":
		s.transitionToSleepOrOffline(machine, "asleep")
	case "offline":
		s.transitionToSleepOrOffline(machine, "offline")
	case "online":
		// 如果之前是睡眠/离线状态，需要唤醒
		if currentState == state.StateAsleep || currentState == state.StateOffline {
			if machine.CanTransition(state.EventWakeUp) {
				machine.Trigger(state.EventWakeUp)
				// 车辆唤醒，标记为活跃状态，重置空闲计时器
				carID := machine.GetState().CarID
				s.markVehicleActive(carID)
				s.resetBackoff(carID)
				s.logger.Info("Vehicle woke up",
					zap.Int64("car_id", carID),
					zap.String("from", currentState))

				// 重启 Streaming 连接（如果之前因车辆离线而停止）
				s.restartStreamingIfNeeded(carID)
			}
		}
	}
}

// transitionToSleepOrOffline 转换到睡眠或离线状态
func (s *VehicleService) transitionToSleepOrOffline(machine *state.Machine, targetState string) {
	currentState := machine.CurrentState()

	// 如果已经是目标状态，不需要转换
	if currentState == targetState {
		return
	}

	var event string
	if targetState == "asleep" {
		event = state.EventFallAsleep
	} else {
		event = state.EventGoOffline
	}

	// 尝试直接转换
	if machine.CanTransition(event) {
		machine.Trigger(event)
		s.logger.Info("Vehicle state changed",
			zap.Int64("car_id", machine.GetState().CarID),
			zap.String("from", currentState),
			zap.String("to", targetState))
		return
	}

	// 如果不能直接转换（比如从 driving -> asleep），需要先回到 online
	// 这种情况不太常见，通常 API 不会直接返回 asleep 如果车辆在驾驶中
	s.logger.Warn("Cannot transition to sleep/offline directly",
		zap.Int64("car_id", machine.GetState().CarID),
		zap.String("current", currentState),
		zap.String("target", targetState))
}

// updateMachineFromData 从 API 数据更新状态机
func (s *VehicleService) updateMachineFromData(machine *state.Machine, data *tesla.VehicleData) {
	machine.UpdateState(func(vs *state.VehicleState) {
		if data.ChargeState != nil {
			vs.BatteryLevel = data.ChargeState.BatteryLevel
			vs.RangeKm = tesla.MilesToKm(data.ChargeState.EstBatteryRange)
			vs.PluggedIn = data.ChargeState.ChargingState != "Disconnected"
			vs.ChargingState = data.ChargeState.ChargingState
			vs.ChargerPower = data.ChargeState.ChargerPower
			// 新增充电相关字段
			vs.ChargeLimitSoc = data.ChargeState.ChargeLimitSoc
			vs.TimeToFullCharge = data.ChargeState.TimeToFullCharge
			vs.ChargerVoltage = data.ChargeState.ChargerVoltage
			vs.ChargerCurrent = data.ChargeState.ChargerActualCurrent
			vs.UsableBatteryLevel = data.ChargeState.UsableBatteryLevel
			vs.IdealRangeKm = tesla.MilesToKm(data.ChargeState.IdealBatteryRange)
		}
		if data.DriveState != nil {
			vs.Latitude = data.DriveState.Latitude
			vs.Longitude = data.DriveState.Longitude
			vs.Speed = data.DriveState.Speed
			vs.Power = data.DriveState.Power
			// 新增航向角
			vs.Heading = data.DriveState.Heading
		}
		if data.ClimateState != nil {
			temp := data.ClimateState.InsideTemp
			vs.InsideTemp = &temp
			outTemp := data.ClimateState.OutsideTemp
			vs.OutsideTemp = &outTemp
			// 新增空调状态
			vs.IsClimateOn = data.ClimateState.IsClimateOn
		}
		if data.VehicleState != nil {
			vs.Locked = data.VehicleState.Locked
			vs.SentryMode = data.VehicleState.SentryMode
			// TPMS 胎压数据
			vs.TpmsPressureFL = data.VehicleState.TpmsPressureFL
			vs.TpmsPressureFR = data.VehicleState.TpmsPressureFR
			vs.TpmsPressureRL = data.VehicleState.TpmsPressureRL
			vs.TpmsPressureRR = data.VehicleState.TpmsPressureRR
			// 新增字段
			vs.Odometer = tesla.MilesToKm(data.VehicleState.Odometer)
			vs.CarVersion = data.VehicleState.CarVersion
			vs.IsUserPresent = data.VehicleState.IsUserPresent
			// 门状态：任一门打开则为 true
			vs.DoorsOpen = data.VehicleState.DriverDoorOpen != 0 ||
				data.VehicleState.PassengerDoorOpen != 0 ||
				data.VehicleState.DriverRearDoorOpen != 0 ||
				data.VehicleState.PassengerRearDoorOpen != 0
			// 窗户状态：任一窗打开则为 true
			vs.WindowsOpen = data.VehicleState.DriverWindowOpen != 0 ||
				data.VehicleState.PassengerWindowOpen != 0 ||
				data.VehicleState.DriverRearWindowOpen != 0 ||
				data.VehicleState.PassengerRearWindowOpen != 0
			// 前后备箱状态
			vs.FrunkOpen = data.VehicleState.FrunkOpen != 0
			vs.TrunkOpen = data.VehicleState.TrunkOpen != 0
		}
	})
}

// createPosition 创建位置记录
func (s *VehicleService) createPosition(carID int64, data *tesla.VehicleData) *models.Position {
	pos := &models.Position{
		CarID:      carID,
		RecordedAt: time.Now(),
	}

	if data.DriveState != nil {
		pos.Latitude = data.DriveState.Latitude
		pos.Longitude = data.DriveState.Longitude
		pos.Heading = data.DriveState.Heading
		pos.Speed = data.DriveState.Speed
		pos.Power = data.DriveState.Power
	}

	if data.ChargeState != nil {
		pos.BatteryLevel = data.ChargeState.BatteryLevel
		pos.RangeKm = tesla.MilesToKm(data.ChargeState.EstBatteryRange)
	}

	if data.VehicleState != nil {
		pos.Odometer = tesla.MilesToKm(data.VehicleState.Odometer)
		// TPMS 胎压数据
		pos.TpmsPressureFL = data.VehicleState.TpmsPressureFL
		pos.TpmsPressureFR = data.VehicleState.TpmsPressureFR
		pos.TpmsPressureRL = data.VehicleState.TpmsPressureRL
		pos.TpmsPressureRR = data.VehicleState.TpmsPressureRR
	}

	if data.ClimateState != nil {
		temp := data.ClimateState.InsideTemp
		pos.InsideTemp = &temp
		outTemp := data.ClimateState.OutsideTemp
		pos.OutsideTemp = &outTemp
	}

	return pos
}

// handleStateTransitions 处理状态转换
func (s *VehicleService) handleStateTransitions(ctx context.Context, car *models.Car, machine *state.Machine, data *tesla.VehicleData) {
	currentState := machine.CurrentState()

	// 检测驾驶状态
	isDriving := data.DriveState != nil && data.DriveState.ShiftState != nil && *data.DriveState.ShiftState != "P"
	if isDriving && currentState != state.StateDriving {
		if machine.CanTransition(state.EventStartDriving) {
			machine.Trigger(state.EventStartDriving)
			s.startDrive(ctx, car, data)
			// 标记车辆为活跃状态，重置空闲计时器
			s.markVehicleActive(car.ID)
		}
	} else if !isDriving && currentState == state.StateDriving {
		machine.Trigger(state.EventStopDriving)
		s.endDrive(ctx, car, data)
	}

	// 检测充电状态
	isCharging := data.ChargeState != nil && data.ChargeState.ChargingState == "Charging"
	if isCharging && currentState != state.StateCharging {
		if machine.CanTransition(state.EventStartCharging) {
			machine.Trigger(state.EventStartCharging)
			s.startCharging(ctx, car, data)
			// 标记车辆为活跃状态，重置空闲计时器
			s.markVehicleActive(car.ID)
		}
	} else if !isCharging && currentState == state.StateCharging {
		machine.Trigger(state.EventStopCharging)
		s.endCharging(ctx, car, data)
	}
}

// startDrive 开始行程
func (s *VehicleService) startDrive(ctx context.Context, car *models.Car, data *tesla.VehicleData) {
	drive := &models.Drive{
		CarID:     car.ID,
		StartTime: time.Now(),
	}

	if data.ChargeState != nil {
		drive.StartBatteryLevel = data.ChargeState.BatteryLevel
		drive.StartRangeKm = tesla.MilesToKm(data.ChargeState.EstBatteryRange)
	}

	if err := s.driveRepo.Create(ctx, drive); err != nil {
		s.logger.Error("Failed to create drive", zap.Error(err))
	} else {
		s.logger.Info("Started drive", zap.Int64("drive_id", drive.ID))
	}
}

// endDrive 结束行程
func (s *VehicleService) endDrive(ctx context.Context, car *models.Car, data *tesla.VehicleData) {
	drive, err := s.driveRepo.GetActiveDrive(ctx, car.ID)
	if err != nil {
		s.logger.Error("Failed to get active drive", zap.Error(err))
		return
	}

	now := time.Now()
	drive.EndTime = &now
	drive.DurationMin = now.Sub(drive.StartTime).Minutes()

	if data.ChargeState != nil {
		level := data.ChargeState.BatteryLevel
		drive.EndBatteryLevel = &level
		rangeKm := tesla.MilesToKm(data.ChargeState.EstBatteryRange)
		drive.EndRangeKm = &rangeKm
	}

	if err := s.driveRepo.Complete(ctx, drive); err != nil {
		s.logger.Error("Failed to complete drive", zap.Error(err))
	} else {
		s.logger.Info("Completed drive", zap.Int64("drive_id", drive.ID), zap.Float64("duration_min", drive.DurationMin))
	}
}

// startCharging 开始充电
func (s *VehicleService) startCharging(ctx context.Context, car *models.Car, data *tesla.VehicleData) {
	cp := &models.ChargingProcess{
		CarID:     car.ID,
		StartTime: time.Now(),
	}

	if data.ChargeState != nil {
		cp.StartBatteryLevel = data.ChargeState.BatteryLevel
		cp.StartRangeKm = tesla.MilesToKm(data.ChargeState.EstBatteryRange)
	}

	if err := s.chargeRepo.CreateProcess(ctx, cp); err != nil {
		s.logger.Error("Failed to create charging process", zap.Error(err))
	} else {
		s.logger.Info("Started charging", zap.Int64("charging_process_id", cp.ID))
	}
}

// endCharging 结束充电
func (s *VehicleService) endCharging(ctx context.Context, car *models.Car, data *tesla.VehicleData) {
	cp, err := s.chargeRepo.GetActiveProcess(ctx, car.ID)
	if err != nil {
		s.logger.Error("Failed to get active charging process", zap.Error(err))
		return
	}

	now := time.Now()
	cp.EndTime = &now
	cp.DurationMin = now.Sub(cp.StartTime).Minutes()

	if data.ChargeState != nil {
		level := data.ChargeState.BatteryLevel
		cp.EndBatteryLevel = &level
		rangeKm := tesla.MilesToKm(data.ChargeState.EstBatteryRange)
		cp.EndRangeKm = &rangeKm
		cp.ChargeEnergyAdded = data.ChargeState.ChargeEnergyAdded
	}

	if err := s.chargeRepo.CompleteProcess(ctx, cp); err != nil {
		s.logger.Error("Failed to complete charging process", zap.Error(err))
	} else {
		s.logger.Info("Completed charging", zap.Int64("charging_process_id", cp.ID), zap.Float64("energy_added", cp.ChargeEnergyAdded))
	}
}

// onStateChange 状态变化回调
func (s *VehicleService) onStateChange(carID int64, from, to string) {
	s.logger.Info("Vehicle state changed", zap.Int64("car_id", carID), zap.String("from", from), zap.String("to", to))
}

// notifySubscribers 通知订阅者（内部 channel 订阅者）
func (s *VehicleService) notifySubscribers(vs *state.VehicleState) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ch := range s.subscribers {
		select {
		case ch <- vs:
		default:
			// 跳过慢消费者
		}
	}
}

// broadcastState 广播状态到 WebSocket
func (s *VehicleService) broadcastState(vs *state.VehicleState) {
	if s.wsHub == nil {
		return
	}
	s.wsHub.BroadcastStateUpdate(vs)
	s.logger.Debug("Broadcasted state update via WebSocket", zap.Int64("car_id", vs.CarID))
}

// GetCars 获取车辆列表（用于 WebSocket 初始数据）
func (s *VehicleService) GetCars(ctx context.Context) ([]*models.Car, error) {
	return s.carRepo.List(ctx)
}

// updateCarConfig 更新车辆配置信息
func (s *VehicleService) updateCarConfig(ctx context.Context, car *models.Car, config *tesla.VehicleConfig) {
	needUpdate := false

	if config.CarType != "" && car.Model != config.CarType {
		car.Model = config.CarType
		needUpdate = true
	}
	if config.ExteriorColor != "" && car.ExteriorColor != config.ExteriorColor {
		car.ExteriorColor = config.ExteriorColor
		needUpdate = true
	}
	if config.TrimBadging != "" && car.TrimBadging != config.TrimBadging {
		car.TrimBadging = config.TrimBadging
		needUpdate = true
	}
	if config.WheelType != "" && car.WheelType != config.WheelType {
		car.WheelType = config.WheelType
		needUpdate = true
	}

	if needUpdate {
		if err := s.carRepo.Update(ctx, car); err != nil {
			s.logger.Error("Failed to update car config", zap.Error(err), zap.Int64("car_id", car.ID))
		} else {
			s.logger.Info("Updated car config", zap.Int64("car_id", car.ID), zap.String("model", car.Model))
		}
	}
}

// ============================================================================
// TeslaMate 风格的休眠机制实现
// ============================================================================

// SleepBlockReason 休眠阻止原因
type SleepBlockReason string

const (
	SleepBlockNone             SleepBlockReason = ""
	SleepBlockUserPresent      SleepBlockReason = "user_present"
	SleepBlockSentryMode       SleepBlockReason = "sentry_mode"
	SleepBlockPreconditioning  SleepBlockReason = "preconditioning"
	SleepBlockDoorsOpen        SleepBlockReason = "doors_open"
	SleepBlockTrunkOpen        SleepBlockReason = "trunk_open"
	SleepBlockFrunkOpen        SleepBlockReason = "frunk_open"
	SleepBlockWindowsOpen      SleepBlockReason = "windows_open"
	SleepBlockUnlocked         SleepBlockReason = "unlocked"
	SleepBlockClimateOn        SleepBlockReason = "climate_on"
	SleepBlockPowerUsage       SleepBlockReason = "power_usage"
	SleepBlockDownloadingUpdate SleepBlockReason = "downloading_update"
)

// canFallAsleep 检查车辆是否可以进入休眠 (参考 TeslaMate can_fall_asleep)
// 返回空字符串表示可以休眠，否则返回阻止原因
func (s *VehicleService) canFallAsleep(data *tesla.VehicleData) SleepBlockReason {
	// 1. 用户在场
	if data.VehicleState != nil && data.VehicleState.IsUserPresent {
		return SleepBlockUserPresent
	}

	// 2. 哨兵模式开启
	if data.VehicleState != nil && data.VehicleState.SentryMode {
		return SleepBlockSentryMode
	}

	// 3. 预热/预冷中
	if data.ClimateState != nil && data.ClimateState.IsPreconditioning {
		return SleepBlockPreconditioning
	}

	// 4. 空调开启 (非预热模式下的空调使用)
	if data.ClimateState != nil && data.ClimateState.IsClimateOn {
		return SleepBlockClimateOn
	}

	// 5. 门打开
	if data.VehicleState != nil {
		if data.VehicleState.DriverDoorOpen != 0 ||
			data.VehicleState.PassengerDoorOpen != 0 ||
			data.VehicleState.DriverRearDoorOpen != 0 ||
			data.VehicleState.PassengerRearDoorOpen != 0 {
			return SleepBlockDoorsOpen
		}
	}

	// 6. 后备箱打开
	if data.VehicleState != nil && data.VehicleState.TrunkOpen != 0 {
		return SleepBlockTrunkOpen
	}

	// 7. 前备箱打开
	if data.VehicleState != nil && data.VehicleState.FrunkOpen != 0 {
		return SleepBlockFrunkOpen
	}

	// 8. 窗户打开
	if data.VehicleState != nil {
		if data.VehicleState.DriverWindowOpen != 0 ||
			data.VehicleState.PassengerWindowOpen != 0 ||
			data.VehicleState.DriverRearWindowOpen != 0 ||
			data.VehicleState.PassengerRearWindowOpen != 0 {
			return SleepBlockWindowsOpen
		}
	}

	// 9. 车辆未锁定（如果配置要求）
	if s.cfg.RequireNotUnlocked && data.VehicleState != nil && !data.VehicleState.Locked {
		return SleepBlockUnlocked
	}

	// 10. 正在消耗电力 (power > 0 表示在放电)
	if data.DriveState != nil && data.DriveState.Power > 0 {
		return SleepBlockPowerUsage
	}

	// 11. 正在下载更新
	if data.VehicleState != nil && data.VehicleState.SoftwareUpdate != nil {
		su := data.VehicleState.SoftwareUpdate
		if su.Status == "downloading" && su.DownloadPerc < 100 {
			return SleepBlockDownloadingUpdate
		}
	}

	return SleepBlockNone
}

// tryToSuspend 尝试进入暂停状态 (参考 TeslaMate try_to_suspend)
// 在 online 状态下调用，检查是否应该暂停日志以允许车辆休眠
func (s *VehicleService) tryToSuspend(carID int64, machine *state.Machine, data *tesla.VehicleData) {
	currentState := machine.CurrentState()

	// 只在 online 状态下尝试暂停
	if currentState != state.StateOnline {
		return
	}

	// 检查是否可以休眠
	blockReason := s.canFallAsleep(data)

	// 获取空闲时间
	s.mu.RLock()
	lastUsed, exists := s.lastUsedTimes[carID]
	s.mu.RUnlock()
	if !exists {
		lastUsed = time.Now()
	}

	idleMinutes := time.Since(lastUsed).Minutes()
	suspendAfterIdle := float64(s.cfg.SuspendAfterIdleMin)

	// 如果有阻止原因
	if blockReason != SleepBlockNone {
		// 如果已经空闲超过阈值，记录警告日志
		if idleMinutes >= suspendAfterIdle {
			s.logger.Info("Cannot suspend logging",
				zap.Int64("car_id", carID),
				zap.String("reason", string(blockReason)),
				zap.Float64("idle_minutes", idleMinutes))
		}
		// 更新最后活跃时间（因为有活动阻止休眠）
		s.markVehicleActive(carID)
		return
	}

	// 检查是否已空闲足够时间
	if idleMinutes < suspendAfterIdle {
		s.logger.Debug("Vehicle idle but not long enough to suspend",
			zap.Int64("car_id", carID),
			zap.Float64("idle_minutes", idleMinutes),
			zap.Float64("suspend_after", suspendAfterIdle))
		return
	}

	// 可以暂停 - 进入 suspended 状态
	if machine.CanTransition(state.EventSuspend) {
		if err := machine.Trigger(state.EventSuspend); err != nil {
			s.logger.Error("Failed to suspend logging",
				zap.Int64("car_id", carID),
				zap.Error(err))
			return
		}

		s.logger.Info("Suspending logging to allow vehicle sleep",
			zap.Int64("car_id", carID),
			zap.Float64("idle_minutes", idleMinutes))

		// 设置暂停状态的轮询间隔
		s.mu.Lock()
		s.pollIntervals[carID] = s.cfg.SuspendPollInterval
		s.mu.Unlock()
	}
}

// markVehicleActive 标记车辆为活跃状态
func (s *VehicleService) markVehicleActive(carID int64) {
	s.mu.Lock()
	s.lastUsedTimes[carID] = time.Now()
	s.mu.Unlock()
}

// SuspendLogging 手动暂停日志记录 (供 API 调用)
func (s *VehicleService) SuspendLogging(carID int64) error {
	machine, ok := s.stateManager.Get(carID)
	if !ok {
		return fmt.Errorf("vehicle %d not found", carID)
	}

	currentState := machine.CurrentState()

	// 只能从 online 状态暂停
	switch currentState {
	case state.StateAsleep, state.StateOffline:
		return nil // 已经在休眠/离线，无需操作
	case state.StateSuspended:
		return nil // 已经暂停
	case state.StateDriving:
		return fmt.Errorf("cannot suspend: vehicle is driving")
	case state.StateCharging:
		return fmt.Errorf("cannot suspend: vehicle is charging")
	case state.StateUpdating:
		return fmt.Errorf("cannot suspend: vehicle is updating")
	}

	if !machine.CanTransition(state.EventSuspend) {
		return fmt.Errorf("cannot suspend from state: %s", currentState)
	}

	if err := machine.Trigger(state.EventSuspend); err != nil {
		return fmt.Errorf("failed to suspend: %w", err)
	}

	s.logger.Info("Manually suspended logging", zap.Int64("car_id", carID))

	// 设置暂停状态的轮询间隔
	s.mu.Lock()
	s.pollIntervals[carID] = s.cfg.SuspendPollInterval
	s.mu.Unlock()

	return nil
}

// ResumeLogging 手动恢复日志记录 (供 API 调用)
func (s *VehicleService) ResumeLogging(carID int64) error {
	machine, ok := s.stateManager.Get(carID)
	if !ok {
		return fmt.Errorf("vehicle %d not found", carID)
	}

	currentState := machine.CurrentState()

	// 只能从 suspended 或 asleep/offline 状态恢复
	switch currentState {
	case state.StateOnline, state.StateDriving, state.StateCharging, state.StateUpdating:
		return nil // 已经在活跃状态
	case state.StateSuspended:
		if !machine.CanTransition(state.EventResume) {
			return fmt.Errorf("cannot resume from suspended state")
		}
		if err := machine.Trigger(state.EventResume); err != nil {
			return fmt.Errorf("failed to resume: %w", err)
		}
	case state.StateAsleep, state.StateOffline:
		// 从睡眠/离线状态恢复需要唤醒车辆
		// 这里只是增加轮询频率，等待车辆自然唤醒或 API 唤醒
		s.logger.Info("Expecting imminent wakeup, increasing polling frequency",
			zap.Int64("car_id", carID))
	}

	s.logger.Info("Manually resumed logging", zap.Int64("car_id", carID))

	// 重置轮询间隔为在线间隔
	s.mu.Lock()
	s.pollIntervals[carID] = s.cfg.PollIntervalOnline
	s.lastUsedTimes[carID] = time.Now()
	s.mu.Unlock()

	return nil
}

// ============================================================================
// Tesla Streaming API 集成 (双链路架构)
// ============================================================================

// startAllStreaming 为所有车辆启动 Streaming 连接
func (s *VehicleService) startAllStreaming(ctx context.Context) {
	// 创建 Streaming 专用的 context
	s.streamingCtx, s.streamingCancel = context.WithCancel(ctx)

	cars, err := s.carRepo.List(ctx)
	if err != nil {
		s.logger.Error("Failed to list cars for streaming", zap.Error(err))
		return
	}

	for _, car := range cars {
		s.startStreaming(car)
	}

	s.logger.Info("Started streaming for all vehicles",
		zap.Int("count", len(cars)))
}

// stopAllStreaming 停止所有 Streaming 连接
func (s *VehicleService) stopAllStreaming() {
	if s.streamingCancel != nil {
		s.streamingCancel()
	}

	s.mu.Lock()
	for vehicleID, client := range s.streamingClients {
		client.Stop()
		s.logger.Debug("Stopped streaming client", zap.Int64("vehicle_id", vehicleID))
	}
	s.streamingClients = make(map[int64]*tesla.StreamingClient)
	s.mu.Unlock()

	s.logger.Info("Stopped all streaming connections")
}

// startStreaming 为单个车辆启动 Streaming 连接
func (s *VehicleService) startStreaming(car *models.Car) {
	token := s.teslaClient.GetToken()
	if token == nil {
		s.logger.Warn("No token available for streaming",
			zap.Int64("car_id", car.ID))
		return
	}

	client := tesla.NewStreamingClient(s.logger, car.TeslaVehicleID, token.AccessToken)

	// 设置自定义 host（如果配置了）
	if s.cfg.StreamingHost != "" {
		client.SetHost(s.cfg.StreamingHost)
	}

	// 设置回调
	client.SetCallbacks(tesla.StreamingCallbacks{
		OnData:           s.handleStreamData,
		OnConnect:        s.handleStreamConnect,
		OnDisconnect:     s.handleStreamDisconnect,
		OnVehicleOffline: s.handleStreamVehicleOffline,
	})

	// 保存客户端引用
	s.mu.Lock()
	s.streamingClients[car.TeslaVehicleID] = client
	s.mu.Unlock()

	// 启动自动重连
	client.StartWithReconnect(s.streamingCtx)

	s.logger.Info("Started streaming for vehicle",
		zap.Int64("car_id", car.ID),
		zap.Int64("vehicle_id", car.TeslaVehicleID))
}

// handleStreamData 处理 Streaming 数据
// 关键：实现 < 1 秒的唤醒检测
func (s *VehicleService) handleStreamData(vehicleID int64, data *tesla.StreamData) {
	// 根据 vehicle_id 找到 car_id
	carID := s.findCarIDByVehicleID(vehicleID)
	if carID == 0 {
		s.logger.Warn("Unknown vehicle in streaming data",
			zap.Int64("vehicle_id", vehicleID))
		return
	}

	machine, ok := s.stateManager.Get(carID)
	if !ok {
		return
	}

	currentState := machine.CurrentState()

	// 检测换挡 → 立即开始驾驶记录
	if data.ShiftState == "D" || data.ShiftState == "N" || data.ShiftState == "R" {
		s.logger.Info("Streaming: Driving detected via shift state",
			zap.Int64("car_id", carID),
			zap.String("shift_state", data.ShiftState),
			zap.String("from_state", currentState))

		// 标记活跃
		s.markVehicleActive(carID)

		// 如果在暂停状态，需要先恢复
		if currentState == state.StateSuspended {
			if machine.CanTransition(state.EventResume) {
				machine.Trigger(state.EventResume)
			}
		}

		// 触发驾驶状态
		if machine.CanTransition(state.EventStartDriving) {
			machine.Trigger(state.EventStartDriving)
		}

		// 立即触发完整轮询获取详细数据
		s.triggerImmediatePoll(carID)
		return
	}

	// 检测充电（负功率）
	if data.Power < 0 {
		s.logger.Info("Streaming: Charging detected via negative power",
			zap.Int64("car_id", carID),
			zap.Int("power", data.Power),
			zap.String("from_state", currentState))

		// 标记活跃
		s.markVehicleActive(carID)

		// 如果在暂停状态，需要先恢复
		if currentState == state.StateSuspended {
			if machine.CanTransition(state.EventResume) {
				machine.Trigger(state.EventResume)
			}
		}

		// 触发充电状态
		if machine.CanTransition(state.EventStartCharging) {
			machine.Trigger(state.EventStartCharging)
		}

		// 立即触发完整轮询
		s.triggerImmediatePoll(carID)
		return
	}

	// 检测耗电（正功率，如空调）
	if data.Power > 0 {
		s.logger.Debug("Streaming: Power usage detected",
			zap.Int64("car_id", carID),
			zap.Int("power", data.Power))

		// 标记活跃，重置空闲计时器
		s.markVehicleActive(carID)

		// 如果在暂停状态，恢复到 online
		if currentState == state.StateSuspended {
			if machine.CanTransition(state.EventResume) {
				machine.Trigger(state.EventResume)
				s.logger.Info("Streaming: Resumed from suspended due to power usage",
					zap.Int64("car_id", carID))
			}
		}
	}

	// 更新部分状态数据（不触发完整轮询）
	machine.UpdateState(func(vs *state.VehicleState) {
		if data.SOC > 0 {
			vs.BatteryLevel = data.SOC
		}
		if data.EstLat != 0 && data.EstLng != 0 {
			vs.Latitude = data.EstLat
			vs.Longitude = data.EstLng
		}
		if data.Speed > 0 {
			speed := data.Speed
			vs.Speed = &speed
		}
		vs.Power = data.Power
		if data.Heading > 0 {
			vs.Heading = data.Heading
		}
	})
}

// handleStreamConnect Streaming 连接成功回调
func (s *VehicleService) handleStreamConnect(vehicleID int64) {
	s.logger.Info("Streaming connected",
		zap.Int64("vehicle_id", vehicleID))
}

// handleStreamDisconnect Streaming 断开回调
func (s *VehicleService) handleStreamDisconnect(vehicleID int64, err error) {
	if err != nil {
		s.logger.Warn("Streaming disconnected with error",
			zap.Int64("vehicle_id", vehicleID),
			zap.Error(err))
	} else {
		s.logger.Debug("Streaming disconnected",
			zap.Int64("vehicle_id", vehicleID))
	}
}

// handleStreamVehicleOffline 车辆离线回调，停止 Streaming 重连
func (s *VehicleService) handleStreamVehicleOffline(vehicleID int64) {
	s.logger.Info("Streaming: Vehicle offline, will restart when vehicle comes online",
		zap.Int64("vehicle_id", vehicleID))
}

// restartStreamingIfNeeded 如果 Streaming 因车辆离线而停止，则重新启动
func (s *VehicleService) restartStreamingIfNeeded(carID int64) {
	if !s.cfg.UseStreamingAPI {
		return
	}

	// 根据 carID 找到对应的 vehicleID
	car, err := s.carRepo.GetByID(context.Background(), carID)
	if err != nil {
		return
	}

	s.mu.RLock()
	client, exists := s.streamingClients[car.TeslaVehicleID]
	s.mu.RUnlock()

	if !exists {
		// 如果没有客户端，创建新的
		s.startStreaming(car)
		return
	}

	// 如果客户端存在且车辆之前离线，重新启动
	if client.IsVehicleOffline() {
		client.ResetAndRestart(s.streamingCtx)
	}
}

// findCarIDByVehicleID 根据 Tesla vehicle_id 查找内部 car_id
func (s *VehicleService) findCarIDByVehicleID(vehicleID int64) int64 {
	ctx := context.Background()
	cars, err := s.carRepo.List(ctx)
	if err != nil {
		return 0
	}

	for _, car := range cars {
		if car.TeslaVehicleID == vehicleID {
			return car.ID
		}
	}
	return 0
}

// triggerImmediatePoll 触发立即轮询
// 当 Streaming 检测到状态变化时调用，立即获取完整数据
func (s *VehicleService) triggerImmediatePoll(carID int64) {
	s.mu.Lock()
	// 重置轮询间隔和时间，确保下一次 ticker 触发时立即轮询
	s.pollIntervals[carID] = 0
	s.lastPollTimes[carID] = time.Time{} // 零值确保立即轮询
	s.mu.Unlock()

	s.logger.Debug("Triggered immediate poll",
		zap.Int64("car_id", carID))
}
