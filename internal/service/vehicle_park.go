package service

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/langchou/tesgazer/internal/api/tesla"
	"github.com/langchou/tesgazer/internal/models"
)

// startParking 开始停车记录
func (s *VehicleService) startParking(ctx context.Context, car *models.Car, data *tesla.VehicleData) {
	parking := &models.Parking{
		CarID:     car.ID,
		StartTime: time.Now(),
	}

	// 位置
	if data.DriveState != nil {
		parking.Latitude = data.DriveState.Latitude
		parking.Longitude = data.DriveState.Longitude
	}

	// 电量
	if data.ChargeState != nil {
		parking.StartBatteryLevel = data.ChargeState.BatteryLevel
		parking.StartRangeKm = tesla.MilesToKm(data.ChargeState.EstBatteryRange)
	}

	// 里程表
	if data.VehicleState != nil {
		parking.StartOdometer = tesla.MilesToKm(data.VehicleState.Odometer)
		parking.StartLocked = data.VehicleState.Locked
		parking.StartSentryMode = data.VehicleState.SentryMode
		parking.StartIsUserPresent = data.VehicleState.IsUserPresent
		// 门状态
		parking.StartDoorsOpen = data.VehicleState.DriverDoorOpen != 0 ||
			data.VehicleState.PassengerDoorOpen != 0 ||
			data.VehicleState.DriverRearDoorOpen != 0 ||
			data.VehicleState.PassengerRearDoorOpen != 0
		// 窗户状态
		parking.StartWindowsOpen = data.VehicleState.DriverWindowOpen != 0 ||
			data.VehicleState.PassengerWindowOpen != 0 ||
			data.VehicleState.DriverRearWindowOpen != 0 ||
			data.VehicleState.PassengerRearWindowOpen != 0
		parking.StartFrunkOpen = data.VehicleState.FrunkOpen != 0
		parking.StartTrunkOpen = data.VehicleState.TrunkOpen != 0
		// 胎压
		parking.StartTpmsPressureFL = data.VehicleState.TpmsPressureFL
		parking.StartTpmsPressureFR = data.VehicleState.TpmsPressureFR
		parking.StartTpmsPressureRL = data.VehicleState.TpmsPressureRL
		parking.StartTpmsPressureRR = data.VehicleState.TpmsPressureRR
		// 软件版本
		parking.CarVersion = data.VehicleState.CarVersion
	}

	// 温度
	if data.ClimateState != nil {
		temp := data.ClimateState.InsideTemp
		parking.StartInsideTemp = &temp
		outTemp := data.ClimateState.OutsideTemp
		parking.StartOutsideTemp = &outTemp
		parking.StartIsClimateOn = data.ClimateState.IsClimateOn
	}

	if err := s.parkingRepo.Create(ctx, parking); err != nil {
		s.logger.Error("Failed to create parking", zap.Error(err))
	} else {
		s.logger.Info("Started parking", zap.Int64("parking_id", parking.ID))
	}

	// 初始化停车期间的累计数据
	s.mu.Lock()
	s.parkingClimateUsage[car.ID] = 0
	s.parkingSentryUsage[car.ID] = 0
	s.parkingLastCheck[car.ID] = time.Now()
	s.parkingTempSamples[car.ID] = []tempSample{}
	// 记录初始温度采样
	if data.ClimateState != nil {
		temp := data.ClimateState.InsideTemp
		outTemp := data.ClimateState.OutsideTemp
		s.parkingTempSamples[car.ID] = append(s.parkingTempSamples[car.ID], tempSample{
			insideTemp:  &temp,
			outsideTemp: &outTemp,
		})
	}
	s.mu.Unlock()
}

// endParking 结束停车记录
func (s *VehicleService) endParking(ctx context.Context, car *models.Car, data *tesla.VehicleData) {
	parking, err := s.parkingRepo.GetActiveParking(ctx, car.ID)
	if err != nil {
		s.logger.Debug("No active parking to end", zap.Int64("car_id", car.ID))
		return
	}

	now := time.Now()
	parking.EndTime = &now
	parking.DurationMin = now.Sub(parking.StartTime).Minutes()

	// 电量变化
	if data.ChargeState != nil {
		level := data.ChargeState.BatteryLevel
		parking.EndBatteryLevel = &level
		rangeKm := tesla.MilesToKm(data.ChargeState.EstBatteryRange)
		parking.EndRangeKm = &rangeKm

		// 计算吸血鬼功耗 (vampire drain)
		// 假设每 % 电量约等于总电池容量的 1%
		// 对于 Model 3 约 60-82 kWh，这里用一个近似值
		if parking.EndBatteryLevel != nil && parking.StartBatteryLevel > *parking.EndBatteryLevel {
			// 简单估算：假设电池容量约 75 kWh
			batteryCapacityKwh := 75.0
			energyUsed := float64(parking.StartBatteryLevel-*parking.EndBatteryLevel) / 100.0 * batteryCapacityKwh
			parking.EnergyUsedKwh = &energyUsed
		}
	}

	// 里程表
	if data.VehicleState != nil {
		endOdometer := tesla.MilesToKm(data.VehicleState.Odometer)
		parking.EndOdometer = &endOdometer
		locked := data.VehicleState.Locked
		parking.EndLocked = &locked
		sentry := data.VehicleState.SentryMode
		parking.EndSentryMode = &sentry
		userPresent := data.VehicleState.IsUserPresent
		parking.EndIsUserPresent = &userPresent
		// 门状态
		doorsOpen := data.VehicleState.DriverDoorOpen != 0 ||
			data.VehicleState.PassengerDoorOpen != 0 ||
			data.VehicleState.DriverRearDoorOpen != 0 ||
			data.VehicleState.PassengerRearDoorOpen != 0
		parking.EndDoorsOpen = &doorsOpen
		// 窗户状态
		windowsOpen := data.VehicleState.DriverWindowOpen != 0 ||
			data.VehicleState.PassengerWindowOpen != 0 ||
			data.VehicleState.DriverRearWindowOpen != 0 ||
			data.VehicleState.PassengerRearWindowOpen != 0
		parking.EndWindowsOpen = &windowsOpen
		frunkOpen := data.VehicleState.FrunkOpen != 0
		parking.EndFrunkOpen = &frunkOpen
		trunkOpen := data.VehicleState.TrunkOpen != 0
		parking.EndTrunkOpen = &trunkOpen
		// 胎压
		parking.EndTpmsPressureFL = data.VehicleState.TpmsPressureFL
		parking.EndTpmsPressureFR = data.VehicleState.TpmsPressureFR
		parking.EndTpmsPressureRL = data.VehicleState.TpmsPressureRL
		parking.EndTpmsPressureRR = data.VehicleState.TpmsPressureRR
	}

	// 温度
	if data.ClimateState != nil {
		temp := data.ClimateState.InsideTemp
		parking.EndInsideTemp = &temp
		outTemp := data.ClimateState.OutsideTemp
		parking.EndOutsideTemp = &outTemp
		climateOn := data.ClimateState.IsClimateOn
		parking.EndIsClimateOn = &climateOn
	}

	// 计算平均温度
	s.mu.RLock()
	samples := s.parkingTempSamples[car.ID]
	climateUsage := s.parkingClimateUsage[car.ID]
	sentryUsage := s.parkingSentryUsage[car.ID]
	s.mu.RUnlock()

	if len(samples) > 0 {
		var insideSum, outsideSum float64
		var insideCount, outsideCount int
		for _, sample := range samples {
			if sample.insideTemp != nil {
				insideSum += *sample.insideTemp
				insideCount++
			}
			if sample.outsideTemp != nil {
				outsideSum += *sample.outsideTemp
				outsideCount++
			}
		}
		if insideCount > 0 {
			avg := insideSum / float64(insideCount)
			parking.InsideTempAvg = &avg
		}
		if outsideCount > 0 {
			avg := outsideSum / float64(outsideCount)
			parking.OutsideTempAvg = &avg
		}
	}

	// 空调和哨兵模式使用时长
	if climateUsage > 0 {
		minutes := climateUsage.Minutes()
		parking.ClimateUsedMin = &minutes
	}
	if sentryUsage > 0 {
		minutes := sentryUsage.Minutes()
		parking.SentryModeUsedMin = &minutes
	}

	if err := s.parkingRepo.Complete(ctx, parking); err != nil {
		s.logger.Error("Failed to complete parking", zap.Error(err))
	} else {
		s.logger.Info("Completed parking",
			zap.Int64("parking_id", parking.ID),
			zap.Float64("duration_min", parking.DurationMin),
			zap.Float64p("energy_used_kwh", parking.EnergyUsedKwh))
	}

	// 清理累计数据
	s.mu.Lock()
	delete(s.parkingClimateUsage, car.ID)
	delete(s.parkingSentryUsage, car.ID)
	delete(s.parkingLastCheck, car.ID)
	delete(s.parkingTempSamples, car.ID)
	s.mu.Unlock()
}

// updateParkingStats 更新停车期间的统计数据
// 每次轮询时调用，累计空调/哨兵模式使用时间，记录温度采样
func (s *VehicleService) updateParkingStats(ctx context.Context, car *models.Car, data *tesla.VehicleData) {
	// 检查是否有活动的停车记录
	_, err := s.parkingRepo.GetActiveParking(ctx, car.ID)
	if err != nil {
		return // 没有活动的停车记录
	}

	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	lastCheck, exists := s.parkingLastCheck[car.ID]
	if !exists {
		s.parkingLastCheck[car.ID] = now
		return
	}

	interval := now.Sub(lastCheck)
	s.parkingLastCheck[car.ID] = now

	// 累计空调使用时长
	if data.ClimateState != nil && data.ClimateState.IsClimateOn {
		s.parkingClimateUsage[car.ID] += interval
	}

	// 累计哨兵模式使用时长
	if data.VehicleState != nil && data.VehicleState.SentryMode {
		s.parkingSentryUsage[car.ID] += interval
	}

	// 记录温度采样
	if data.ClimateState != nil {
		temp := data.ClimateState.InsideTemp
		outTemp := data.ClimateState.OutsideTemp
		s.parkingTempSamples[car.ID] = append(s.parkingTempSamples[car.ID], tempSample{
			insideTemp:  &temp,
			outsideTemp: &outTemp,
		})
	}
}

// updateActiveParkingSnapshot 更新活跃停车记录的快照信息
func (s *VehicleService) updateActiveParkingSnapshot(ctx context.Context, car *models.Car, data *tesla.VehicleData) {
	// 1. 获取活跃的停车记录
	parking, err := s.parkingRepo.GetActiveParking(ctx, car.ID)
	if err != nil {
		return // 没有活跃停车记录
	}

	// 2. 更新快照字段
	if data.ChargeState != nil {
		l := data.ChargeState.BatteryLevel
		parking.EndBatteryLevel = &l
		r := tesla.MilesToKm(data.ChargeState.EstBatteryRange)
		parking.EndRangeKm = &r
	}
	if data.ClimateState != nil {
		i := data.ClimateState.InsideTemp
		parking.EndInsideTemp = &i
		o := data.ClimateState.OutsideTemp
		parking.EndOutsideTemp = &o
		c := data.ClimateState.IsClimateOn
		parking.EndIsClimateOn = &c
	}
	if data.VehicleState != nil {
		sen := data.VehicleState.SentryMode
		parking.EndSentryMode = &sen
		locked := data.VehicleState.Locked
		parking.EndLocked = &locked

		// 更新门窗状态
		doorsOpen := data.VehicleState.DriverDoorOpen != 0 ||
			data.VehicleState.PassengerDoorOpen != 0 ||
			data.VehicleState.DriverRearDoorOpen != 0 ||
			data.VehicleState.PassengerRearDoorOpen != 0
		parking.EndDoorsOpen = &doorsOpen

		windowsOpen := data.VehicleState.DriverWindowOpen != 0 ||
			data.VehicleState.PassengerWindowOpen != 0 ||
			data.VehicleState.DriverRearWindowOpen != 0 ||
			data.VehicleState.PassengerRearWindowOpen != 0
		parking.EndWindowsOpen = &windowsOpen

		frunkOpen := data.VehicleState.FrunkOpen != 0
		parking.EndFrunkOpen = &frunkOpen
		trunkOpen := data.VehicleState.TrunkOpen != 0
		parking.EndTrunkOpen = &trunkOpen
	}

	// 3. 更新统计数据 (从内存累加器)
	s.mu.RLock()
	climUsage := s.parkingClimateUsage[car.ID]
	sentryUsage := s.parkingSentryUsage[car.ID]
	s.mu.RUnlock()

	climMin := climUsage.Minutes()
	sentryMin := sentryUsage.Minutes()

	parking.ClimateUsedMin = &climMin
	parking.SentryModeUsedMin = &sentryMin

	// 4. 保存到数据库
	if err := s.parkingRepo.UpdateSnapshot(ctx, parking); err != nil {
		s.logger.Warn("Failed to update active parking snapshot", zap.Error(err))
	}
}
