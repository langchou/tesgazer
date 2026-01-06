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

	mu          sync.RWMutex
	stopCh      chan struct{}
	wg          sync.WaitGroup
	subscribers []chan *state.VehicleState
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
) *VehicleService {
	svc := &VehicleService{
		cfg:         cfg,
		logger:      logger,
		teslaClient: teslaClient,
		carRepo:     carRepo,
		posRepo:     posRepo,
		driveRepo:   driveRepo,
		chargeRepo:  chargeRepo,
		stopCh:      make(chan struct{}),
	}

	// 创建状态管理器
	svc.stateManager = state.NewManager(svc.onStateChange)

	return svc
}

// Start 启动服务
func (s *VehicleService) Start(ctx context.Context) error {
	s.logger.Info("Starting vehicle service")

	// 同步车辆列表
	if err := s.syncVehicles(ctx); err != nil {
		return fmt.Errorf("sync vehicles: %w", err)
	}

	// 启动轮询
	s.wg.Add(1)
	go s.pollLoop(ctx)

	return nil
}

// Stop 停止服务
func (s *VehicleService) Stop() {
	s.logger.Info("Stopping vehicle service")
	close(s.stopCh)
	s.wg.Wait()
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

// pollLoop 轮询循环
func (s *VehicleService) pollLoop(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.cfg.PollIntervalOnline)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.pollAllVehicles(ctx)
		}
	}
}

// pollAllVehicles 轮询所有车辆
func (s *VehicleService) pollAllVehicles(ctx context.Context) {
	cars, err := s.carRepo.List(ctx)
	if err != nil {
		s.logger.Error("Failed to list cars", zap.Error(err))
		return
	}

	for _, car := range cars {
		if err := s.pollVehicle(ctx, car); err != nil {
			s.logger.Error("Failed to poll vehicle", zap.Error(err), zap.Int64("car_id", car.ID))
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
			if machine.CanTransition(state.EventFallAsleep) {
				machine.Trigger(state.EventFallAsleep)
			}
			return nil
		}
		return err
	}

	// 更新状态机
	s.updateMachineFromData(machine, data)

	// 记录位置
	if data.DriveState != nil {
		pos := s.createPosition(car.ID, data)
		if err := s.posRepo.Create(ctx, pos); err != nil {
			s.logger.Error("Failed to create position", zap.Error(err))
		}
	}

	// 处理状态变化
	s.handleStateTransitions(ctx, car, machine, data)

	// 通知订阅者
	s.notifySubscribers(machine.GetState())

	return nil
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
		}
		if data.DriveState != nil {
			vs.Latitude = data.DriveState.Latitude
			vs.Longitude = data.DriveState.Longitude
			vs.Speed = data.DriveState.Speed
			vs.Power = data.DriveState.Power
		}
		if data.ClimateState != nil {
			temp := data.ClimateState.InsideTemp
			vs.InsideTemp = &temp
			outTemp := data.ClimateState.OutsideTemp
			vs.OutsideTemp = &outTemp
		}
		if data.VehicleState != nil {
			vs.Locked = data.VehicleState.Locked
			vs.SentryMode = data.VehicleState.SentryMode
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

// notifySubscribers 通知订阅者
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
