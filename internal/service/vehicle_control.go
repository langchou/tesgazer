package service

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/langchou/tesgazer/internal/api/tesla"
	"github.com/langchou/tesgazer/internal/state"
)

// SleepBlockReason 休眠阻止原因
type SleepBlockReason string

const (
	SleepBlockNone              SleepBlockReason = ""
	SleepBlockUserPresent       SleepBlockReason = "user_present"
	SleepBlockSentryMode        SleepBlockReason = "sentry_mode"
	SleepBlockPreconditioning   SleepBlockReason = "preconditioning"
	SleepBlockDoorsOpen         SleepBlockReason = "doors_open"
	SleepBlockTrunkOpen         SleepBlockReason = "trunk_open"
	SleepBlockFrunkOpen         SleepBlockReason = "frunk_open"
	SleepBlockWindowsOpen       SleepBlockReason = "windows_open"
	SleepBlockUnlocked          SleepBlockReason = "unlocked"
	SleepBlockClimateOn         SleepBlockReason = "climate_on"
	SleepBlockPowerUsage        SleepBlockReason = "power_usage"
	SleepBlockDownloadingUpdate SleepBlockReason = "downloading_update"
)

// canFallAsleep 检查车辆是否可以进入休眠
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

// tryToSuspend 尝试进入暂停状态
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
