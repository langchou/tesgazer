package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/langchou/tesgazer/internal/api/handlers"
	"github.com/langchou/tesgazer/internal/api/tesla"
	"github.com/langchou/tesgazer/internal/config"
	"github.com/langchou/tesgazer/internal/repository"
	"github.com/langchou/tesgazer/internal/service"
	"github.com/langchou/tesgazer/pkg/ws"
)

func main() {
	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	logger := initLogger(cfg.Debug)
	defer logger.Sync()

	logger.Info("Starting Tesgazer", zap.String("port", cfg.ServerPort))

	// 创建 context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 连接数据库
	db, err := repository.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("Failed to connect database", zap.Error(err))
	}
	defer db.Close()

	// 执行数据库迁移
	if err := db.Migrate(ctx); err != nil {
		logger.Fatal("Failed to migrate database", zap.Error(err))
	}
	logger.Info("Database migrated successfully")

	// 创建 Repository
	carRepo := repository.NewCarRepository(db)
	posRepo := repository.NewPositionRepository(db)
	driveRepo := repository.NewDriveRepository(db)
	chargeRepo := repository.NewChargeRepository(db)

	// 创建 Tesla API 客户端
	teslaClient := tesla.NewClient(
		cfg.TeslaAuthHost,
		cfg.TeslaAPIHost,
		cfg.TeslaClientID,
		cfg.TeslaRedirectURI,
	)

	// 加载 Token（如果存在）
	if err := loadToken(cfg.TokenFile, teslaClient); err != nil {
		logger.Warn("No existing token found, please authenticate", zap.Error(err))
	}

	// 创建 WebSocket Hub
	wsHub := ws.NewHub(logger)
	go wsHub.Run()

	// 创建车辆服务
	vehicleService := service.NewVehicleService(
		cfg,
		logger,
		teslaClient,
		carRepo,
		posRepo,
		driveRepo,
		chargeRepo,
	)

	// 启动车辆服务（如果已认证）
	if teslaClient.GetToken() != nil {
		if err := vehicleService.Start(ctx); err != nil {
			logger.Error("Failed to start vehicle service", zap.Error(err))
		}

		// 订阅状态更新并广播到 WebSocket
		go func() {
			stateCh := vehicleService.Subscribe()
			for state := range stateCh {
				data, _ := json.Marshal(map[string]interface{}{
					"type": "state_update",
					"data": state,
				})
				wsHub.BroadcastToCarSubscribers(state.CarID, data)
			}
		}()
	}

	// 创建 HTTP 处理器
	handler := handlers.NewHandler(
		logger,
		carRepo,
		driveRepo,
		chargeRepo,
		posRepo,
		vehicleService,
		wsHub,
	)

	// 设置 Gin 模式
	if !cfg.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	// 创建路由
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())

	// 注册路由
	handler.RegisterRoutes(router)

	// 添加认证路由
	router.POST("/api/auth/token", func(c *gin.Context) {
		var req struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		}
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		token := &tesla.Token{
			AccessToken:  req.AccessToken,
			RefreshToken: req.RefreshToken,
			CreatedAt:    time.Now(),
			ExpiresIn:    3600 * 8, // 8 小时
		}
		teslaClient.SetToken(token)

		// 保存 token
		if err := saveToken(cfg.TokenFile, token); err != nil {
			logger.Error("Failed to save token", zap.Error(err))
		}

		// 启动服务
		if err := vehicleService.Start(ctx); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start service"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 启动 HTTP 服务器
	server := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: router,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	logger.Info("Server started", zap.String("addr", server.Addr))

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// 停止服务
	vehicleService.Stop()

	// 保存 token
	if token := teslaClient.GetToken(); token != nil {
		saveToken(cfg.TokenFile, token)
	}

	// 优雅关闭
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}

// initLogger 初始化日志
func initLogger(debug bool) *zap.Logger {
	var config zap.Config
	if debug {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config = zap.NewProductionConfig()
	}

	logger, _ := config.Build()
	return logger
}

// corsMiddleware CORS 中间件
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// loadToken 加载 token
func loadToken(filename string, client *tesla.Client) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	var token tesla.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return err
	}

	client.SetToken(&token)
	return nil
}

// saveToken 保存 token
func saveToken(filename string, token *tesla.Token) error {
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0600)
}
