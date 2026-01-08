package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/langchou/tesgazer/internal/api/amap"
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
	geocoder     *amap.GeocoderClient // 高德逆地理编码客户端
	carRepo      *repository.CarRepository
	posRepo      *repository.PositionRepository
	driveRepo    *repository.DriveRepository
	chargeRepo   *repository.ChargeRepository
	parkingRepo  *repository.ParkingRepository
	stateManager *state.Manager
	wsHub        *ws.Hub // WebSocket Hub

	mu          sync.RWMutex
	stopCh      chan struct{}
	wg          sync.WaitGroup
	subscribers []chan *state.VehicleState
	running     bool // 标记服务是否正在运行

	// 指数退避相关状态 (per vehicle)
	pollIntervals map[int64]time.Duration // 每辆车当前的轮询间隔
	lastPollTimes map[int64]time.Time     // 每辆车上次轮询时间
	lastUsedTimes map[int64]time.Time     // 每辆车最后活跃时间 (用于自动休眠)

	// 停车期间的累计数据 (per vehicle)
	parkingClimateUsage map[int64]time.Duration // 空调使用时长累计
	parkingSentryUsage  map[int64]time.Duration // 哨兵模式使用时长累计
	parkingLastCheck    map[int64]time.Time     // 上次检查时间
	parkingTempSamples  map[int64][]tempSample  // 温度采样

	// Tesla Streaming API 客户端 (双链路架构)
	streamingClients map[int64]*tesla.StreamingClient // 每辆车的 Streaming 客户端
	streamingCtx     context.Context                  // Streaming 上下文
	streamingCancel  context.CancelFunc               // 取消函数
}

// tempSample 温度采样
type tempSample struct {
	insideTemp  *float64
	outsideTemp *float64
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
	parkingRepo *repository.ParkingRepository,
	wsHub *ws.Hub,
) *VehicleService {
	// 创建高德逆地理编码客户端
	geocoder := amap.NewGeocoderClient(cfg.AmapAPIKey, logger)
	if geocoder.IsConfigured() {
		logger.Info("Amap geocoder initialized")
	} else {
		logger.Warn("Amap API key not configured, address geocoding will be disabled")
	}

	svc := &VehicleService{
		cfg:                 cfg,
		logger:              logger,
		teslaClient:         teslaClient,
		geocoder:            geocoder,
		carRepo:             carRepo,
		posRepo:             posRepo,
		driveRepo:           driveRepo,
		chargeRepo:          chargeRepo,
		parkingRepo:         parkingRepo,
		wsHub:               wsHub,
		stopCh:              make(chan struct{}),
		pollIntervals:       make(map[int64]time.Duration),
		lastPollTimes:       make(map[int64]time.Time),
		lastUsedTimes:       make(map[int64]time.Time),
		parkingClimateUsage: make(map[int64]time.Duration),
		parkingSentryUsage:  make(map[int64]time.Duration),
		parkingLastCheck:    make(map[int64]time.Time),
		parkingTempSamples:  make(map[int64][]tempSample),
		streamingClients:    make(map[int64]*tesla.StreamingClient),
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

// pollLoop 轮询循环 - 实现指数退避策略
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
		// 暂停日志状态：使用较长的轮询间隔，让车辆有机会休眠（默认 21 分钟）
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

	// 处理状态变化（驾驶、充电等）
	// 注意：必须在记录位置之前处理状态变化，这样才能正确关联 drive_id
	s.handleStateTransitions(ctx, car, machine, data)

	// 如果当前处于停车状态 (Online 且非 Driving/Charging)，更新数据库中的停车记录状态 (哨兵、温度等)
	if machine.CurrentState() == state.StateOnline && data.State == "online" {
		s.updateActiveParkingSnapshot(ctx, car, data)
	}

	// 如果当前处于充电状态，更新活跃充电记录
	if machine.CurrentState() == state.StateCharging {
		s.updateActiveChargingSnapshot(ctx, car, data)
	}

	// 记录位置（仅在线时）
	if data.State == "online" && data.DriveState != nil {
		pos := s.createPosition(car.ID, data)

		// 如果正在驾驶，关联到当前活动的行程
		if machine.CurrentState() == state.StateDriving {
			activeDrive, err := s.driveRepo.GetActiveDrive(ctx, car.ID)
			if err == nil && activeDrive != nil {
				pos.DriveID = &activeDrive.ID
			}
		}

		if err := s.posRepo.Create(ctx, pos); err != nil {
			s.logger.Error("Failed to create position", zap.Error(err))
		}
	}

	// 获取最新状态
	currentState := machine.GetState()

	// 通知内部订阅者
	s.notifySubscribers(currentState)

	// 广播到 WebSocket
	s.broadcastState(currentState)

	// 尝试自动暂停（只在 online 状态下检查）
	// 空闲一段时间后自动暂停日志，允许车辆进入休眠
	if machine.CurrentState() == state.StateOnline {
		s.tryToSuspend(car.ID, machine, data)
	}

	return nil
}

// pollVehicleLightweight 轻量轮询 - 只检查车辆状态，不唤醒车辆
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

// handleStateTransitions 处理状态转换
func (s *VehicleService) handleStateTransitions(ctx context.Context, car *models.Car, machine *state.Machine, data *tesla.VehicleData) {
	currentState := machine.CurrentState()

	// 检测驾驶状态
	isDriving := data.DriveState != nil && data.DriveState.ShiftState != nil && *data.DriveState.ShiftState != "P"
	if isDriving && currentState != state.StateDriving {
		if machine.CanTransition(state.EventStartDriving) {
			// 结束停车记录（如果有）
			s.endParking(ctx, car, data)
			machine.Trigger(state.EventStartDriving)
			s.startDrive(ctx, car, data)
			// 标记车辆为活跃状态，重置空闲计时器
			s.markVehicleActive(car.ID)
		}
	} else if !isDriving && currentState == state.StateDriving {
		machine.Trigger(state.EventStopDriving)
		s.endDrive(ctx, car, data)
		// 开始停车记录
		s.startParking(ctx, car, data)
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

	// 如果在停车状态（online 且不在驾驶/充电），更新停车统计
	if currentState == state.StateOnline && !isDriving && !isCharging {
		s.updateParkingStats(ctx, car, data)
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
