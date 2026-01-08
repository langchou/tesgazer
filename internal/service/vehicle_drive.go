package service

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/langchou/tesgazer/internal/api/tesla"
	"github.com/langchou/tesgazer/internal/models"
)

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

	// 记录起始里程表
	if data.VehicleState != nil {
		drive.StartOdometerKm = tesla.MilesToKm(data.VehicleState.Odometer)
	}

	// 记录起始位置坐标
	if data.DriveState != nil {
		lat := data.DriveState.Latitude
		lng := data.DriveState.Longitude
		drive.StartLatitude = &lat
		drive.StartLongitude = &lng

		// 异步进行逆地理编码（不阻塞行程开始）
		if s.geocoder.IsConfigured() {
			go func() {
				address, err := s.geocoder.ReverseGeocode(context.Background(), lat, lng)
				if err != nil {
					s.logger.Warn("Failed to geocode start address",
						zap.Int64("drive_id", drive.ID),
						zap.Float64("lat", lat),
						zap.Float64("lng", lng),
						zap.Error(err))
					return
				}
				drive.StartAddress = address
				s.logger.Debug("Geocoded start address",
					zap.Int64("drive_id", drive.ID),
					zap.String("address", address.FormattedAddress))
			}()
		}
	}

	if err := s.driveRepo.Create(ctx, drive); err != nil {
		s.logger.Error("Failed to create drive", zap.Error(err))
	} else {
		s.logger.Info("Started drive", zap.Int64("drive_id", drive.ID), zap.Float64("start_odometer_km", drive.StartOdometerKm))
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

	// 记录结束里程表并计算行驶距离
	if data.VehicleState != nil {
		endOdometer := tesla.MilesToKm(data.VehicleState.Odometer)
		drive.EndOdometerKm = &endOdometer
		// 根据里程表计算行驶距离
		if drive.StartOdometerKm > 0 && endOdometer > drive.StartOdometerKm {
			drive.DistanceKm = endOdometer - drive.StartOdometerKm
		}
	}

	// 记录结束位置坐标并解析地址
	if data.DriveState != nil {
		lat := data.DriveState.Latitude
		lng := data.DriveState.Longitude
		drive.EndLatitude = &lat
		drive.EndLongitude = &lng

		// 逆地理编码结束地址
		if s.geocoder.IsConfigured() {
			address, err := s.geocoder.ReverseGeocode(ctx, lat, lng)
			if err != nil {
				s.logger.Warn("Failed to geocode end address",
					zap.Int64("drive_id", drive.ID),
					zap.Float64("lat", lat),
					zap.Float64("lng", lng),
					zap.Error(err))
			} else {
				drive.EndAddress = address
				s.logger.Debug("Geocoded end address",
					zap.Int64("drive_id", drive.ID),
					zap.String("address", address.FormattedAddress))
			}

			// 如果起始地址还是空的，尝试解析起始地址
			if drive.StartAddress == nil && drive.StartLatitude != nil && drive.StartLongitude != nil {
				startAddr, err := s.geocoder.ReverseGeocode(ctx, *drive.StartLatitude, *drive.StartLongitude)
				if err != nil {
					s.logger.Warn("Failed to geocode start address",
						zap.Int64("drive_id", drive.ID),
						zap.Error(err))
				} else {
					drive.StartAddress = startAddr
					s.logger.Debug("Geocoded start address (deferred)",
						zap.Int64("drive_id", drive.ID),
						zap.String("address", startAddr.FormattedAddress))
				}
			}
		}
	}

	// 从位置记录中统计行程数据
	stats, err := s.posRepo.GetDriveStats(ctx, drive.ID)
	if err == nil && stats != nil {
		drive.SpeedMax = stats.SpeedMax
		drive.PowerMax = stats.PowerMax
		drive.PowerMin = stats.PowerMin
		drive.InsideTempAvg = stats.InsideTempAvg
		drive.OutsideTempAvg = stats.OutsideTempAvg
		drive.EnergyUsedKwh = stats.EnergyUsedKwh
		drive.EnergyRegenKwh = stats.EnergyRegenKwh
	}

	if err := s.driveRepo.Complete(ctx, drive); err != nil {
		s.logger.Error("Failed to complete drive", zap.Error(err))
	} else {
		// 构建日志字段
		logFields := []zap.Field{
			zap.Int64("drive_id", drive.ID),
			zap.Float64("duration_min", drive.DurationMin),
			zap.Float64("distance_km", drive.DistanceKm),
			zap.Intp("speed_max", drive.SpeedMax),
			zap.Float64p("energy_regen_kwh", drive.EnergyRegenKwh),
		}
		if drive.StartAddress != nil {
			logFields = append(logFields, zap.String("start_address", drive.StartAddress.FormattedAddress))
		}
		if drive.EndAddress != nil {
			logFields = append(logFields, zap.String("end_address", drive.EndAddress.FormattedAddress))
		}
		s.logger.Info("Completed drive", logFields...)
	}
}
