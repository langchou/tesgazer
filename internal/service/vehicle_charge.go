package service

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/langchou/tesgazer/internal/api/tesla"
	"github.com/langchou/tesgazer/internal/models"
)

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

	// 解析地址
	if data.DriveState != nil && s.geocoder.IsConfigured() {
		addr, err := s.geocoder.ReverseGeocode(ctx, data.DriveState.Latitude, data.DriveState.Longitude)
		if err == nil {
			cp.Address = addr
		} else {
			s.logger.Warn("Failed to geocode charging address", zap.Error(err))
		}
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

// updateActiveChargingSnapshot 更新活跃充电过程的快照信息
func (s *VehicleService) updateActiveChargingSnapshot(ctx context.Context, car *models.Car, data *tesla.VehicleData) {
	// 1. 获取活跃的充电过程
	cp, err := s.chargeRepo.GetActiveProcess(ctx, car.ID)
	if err != nil {
		return // 没有活跃充电过程
	}

	// 2. 更新快照字段
	if data.ChargeState != nil {
		level := data.ChargeState.BatteryLevel
		cp.EndBatteryLevel = &level
		rangeKm := tesla.MilesToKm(data.ChargeState.EstBatteryRange)
		cp.EndRangeKm = &rangeKm
		cp.ChargeEnergyAdded = data.ChargeState.ChargeEnergyAdded

		// 更新最大功率
		currentPower := int(data.ChargeState.ChargerPower)
		if cp.ChargerPowerMax == nil || currentPower > *cp.ChargerPowerMax {
			cp.ChargerPowerMax = &currentPower
		}
	}

	// 更新时长
	now := time.Now()
	cp.DurationMin = now.Sub(cp.StartTime).Minutes()

	// 更新外部温度 (暂用当前温度代替平均温度用于显示)
	if data.ClimateState != nil {
		out := data.ClimateState.OutsideTemp
		cp.OutsideTempAvg = &out
	}

	// 3. 保存到数据库
	if err := s.chargeRepo.UpdateSnapshot(ctx, cp); err != nil {
		s.logger.Warn("Failed to update active charging snapshot", zap.Error(err))
	}
}
