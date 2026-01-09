package service

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/langchou/tesgazer/internal/api/tesla"
	"github.com/langchou/tesgazer/internal/models"
	"github.com/langchou/tesgazer/internal/state"
)

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
			speed := tesla.MphToKmh(data.Speed) // mph -> km/h
			vs.Speed = &speed
		}
		vs.Power = data.Power
		if data.Heading > 0 {
			vs.Heading = data.Heading
		}
	})

	// 核心修改：如果处于驾驶状态，将 Streaming 数据直接入库，实现高频轨迹记录
	if currentState == state.StateDriving && data.EstLat != 0 && data.EstLng != 0 {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// 获取当前关联的行程
			activeDrive, err := s.driveRepo.GetActiveDrive(ctx, carID)
			if err != nil || activeDrive == nil {
				// 可能行程刚开始还没入库，或者查询失败
				return
			}

			// 构造位置数据
			speedKmh := tesla.MphToKmh(data.Speed) // mph -> km/h
			pos := &models.Position{
				CarID:      carID,
				DriveID:    &activeDrive.ID,
				Latitude:   data.EstLat,
				Longitude:  data.EstLng,
				Heading:    data.Heading,
				Speed:      &speedKmh,
				Power:      data.Power,
				RecordedAt: time.Now(),
			}

			// 填充其他可用数据
			if data.SOC > 0 {
				pos.BatteryLevel = data.SOC
			}
			if data.Range > 0 {
				pos.RangeKm = tesla.MilesToKm(float64(data.Range))
			}
			if data.Odometer > 0 {
				pos.Odometer = tesla.MilesToKm(data.Odometer)
			}
			if data.Elevation > 0 {
				elev := int(data.Elevation)
				pos.Elevation = &elev
			}

			// 补全缺失数据（从状态机缓存取）
			// Streaming 数据包有时不包含所有字段，用缓存值填充避免数据跳变为 0
			cachedState := machine.GetState()

			if pos.BatteryLevel == 0 {
				pos.BatteryLevel = cachedState.BatteryLevel
			}
			if pos.RangeKm == 0 {
				pos.RangeKm = cachedState.RangeKm
			}
			if pos.Odometer == 0 {
				pos.Odometer = cachedState.Odometer
			}
			if pos.Heading == 0 {
				pos.Heading = cachedState.Heading
			}
			if pos.InsideTemp == nil {
				pos.InsideTemp = cachedState.InsideTemp
			}
			if pos.OutsideTemp == nil {
				pos.OutsideTemp = cachedState.OutsideTemp
			}

			// 写入数据库
			if err := s.posRepo.Create(ctx, pos); err != nil {
				s.logger.Error("Failed to persist streaming position",
					zap.Error(err),
					zap.Int64("car_id", carID))
			}
		}()
	}
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
