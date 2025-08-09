package database

import (
	"fmt"
	"sync"
	"time"

	"github.com/yxserv/base/pkg/logger"
)

// PerformanceMonitor 数据库性能监控器
type PerformanceMonitor struct {
	startTime      time.Time
	queryCount     int64
	slowQueryCount int64
	totalQueryTime time.Duration
	slowThreshold  time.Duration
	mutex          sync.RWMutex
}

// NewPerformanceMonitor 创建性能监控器
func NewPerformanceMonitor() *PerformanceMonitor {
	return &PerformanceMonitor{
		startTime:     time.Now(),
		slowThreshold: 500 * time.Millisecond, // 慢查询阈值500ms
	}
}

// RecordQuery 记录查询
func (pm *PerformanceMonitor) RecordQuery(duration time.Duration) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	pm.queryCount++
	pm.totalQueryTime += duration

	if duration > pm.slowThreshold {
		pm.slowQueryCount++
	}
}

// GetStats 获取统计信息
func (pm *PerformanceMonitor) GetStats() map[string]interface{} {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	totalTime := time.Since(pm.startTime)
	avgQueryTime := time.Duration(0)
	if pm.queryCount > 0 {
		avgQueryTime = pm.totalQueryTime / time.Duration(pm.queryCount)
	}

	return map[string]interface{}{
		"total_time":       totalTime.String(),
		"query_count":      pm.queryCount,
		"slow_query_count": pm.slowQueryCount,
		"avg_query_time":   avgQueryTime.String(),
		"slow_query_rate":  float64(pm.slowQueryCount) / float64(pm.queryCount) * 100,
	}
}

// LogStats 记录统计信息到日志
func (pm *PerformanceMonitor) LogStats(operation string) {
	stats := pm.GetStats()

	logger.Info("📊 数据库性能统计",
		logger.String("操作", operation),
		logger.String("总耗时", stats["total_time"].(string)),
		logger.String("查询次数", fmt.Sprintf("%d", stats["query_count"].(int64))),
		logger.String("慢查询次数", fmt.Sprintf("%d", stats["slow_query_count"].(int64))),
		logger.String("平均查询时间", stats["avg_query_time"].(string)),
		logger.String("慢查询率", fmt.Sprintf("%.2f%%", stats["slow_query_rate"].(float64))))
}

// DatabaseOptimizer 数据库优化器
type DatabaseOptimizer struct {
	dbManager *DatabaseManager
	monitor   *PerformanceMonitor
}

// NewDatabaseOptimizer 创建数据库优化器
func NewDatabaseOptimizer(dbManager *DatabaseManager) *DatabaseOptimizer {
	return &DatabaseOptimizer{
		dbManager: dbManager,
		monitor:   NewPerformanceMonitor(),
	}
}

// OptimizeForMigration 为迁移优化数据库配置
func (do *DatabaseOptimizer) OptimizeForMigration() error {
	logger.Info("🔧 开始优化数据库配置用于迁移")

	// 获取底层SQL连接
	sqlDB, err := do.dbManager.DB.DB()
	if err != nil {
		return err
	}

	// 临时增加连接数用于迁移
	originalMaxOpen := 200
	originalMaxIdle := 50

	// 迁移期间使用更多连接
	sqlDB.SetMaxOpenConns(300)
	sqlDB.SetMaxIdleConns(100)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	logger.Info("✅ 数据库配置已优化",
		logger.Int("max_open_conns", 300),
		logger.Int("max_idle_conns", 100),
		logger.String("conn_max_lifetime", "5m"))

	// 返回恢复函数
	go func() {
		// 迁移完成后恢复原始配置
		time.Sleep(30 * time.Second) // 等待迁移完成
		sqlDB.SetMaxOpenConns(originalMaxOpen)
		sqlDB.SetMaxIdleConns(originalMaxIdle)
		sqlDB.SetConnMaxLifetime(10 * time.Minute)
		logger.Info("🔄 数据库配置已恢复为正常值")
	}()

	return nil
}

// CheckDatabaseHealth 检查数据库健康状态
func (do *DatabaseOptimizer) CheckDatabaseHealth() error {
	logger.Info("🔍 检查数据库健康状态")

	sqlDB, err := do.dbManager.DB.DB()
	if err != nil {
		return err
	}

	// 检查连接
	if err := sqlDB.Ping(); err != nil {
		logger.Error("❌ 数据库连接检查失败", logger.Error2(err))
		return err
	}

	// 获取连接池状态
	stats := sqlDB.Stats()
	logger.Info("📊 数据库连接池状态",
		logger.Int("open_connections", stats.OpenConnections),
		logger.Int("in_use", stats.InUse),
		logger.Int("idle", stats.Idle),
		logger.String("wait_count", fmt.Sprintf("%d", stats.WaitCount)),
		logger.String("wait_duration", stats.WaitDuration.String()),
		logger.String("max_idle_closed", fmt.Sprintf("%d", stats.MaxIdleClosed)),
		logger.String("max_lifetime_closed", fmt.Sprintf("%d", stats.MaxLifetimeClosed)))

	// 检查是否有连接等待
	if stats.WaitCount > 0 {
		logger.Warn("⚠️ 检测到连接等待，可能需要增加连接池大小",
			logger.String("wait_count", fmt.Sprintf("%d", stats.WaitCount)),
			logger.String("wait_duration", stats.WaitDuration.String()))
	}

	logger.Info("✅ 数据库健康状态检查完成")
	return nil
}

// GetOptimizationRecommendations 获取优化建议
func (do *DatabaseOptimizer) GetOptimizationRecommendations() []string {
	recommendations := []string{}

	sqlDB, err := do.dbManager.DB.DB()
	if err != nil {
		return recommendations
	}

	stats := sqlDB.Stats()

	// 基于连接池状态给出建议
	if stats.WaitCount > 100 {
		recommendations = append(recommendations, "建议增加MaxOpenConns，当前连接等待次数过多")
	}

	if stats.MaxIdleClosed > int64(stats.OpenConnections) {
		recommendations = append(recommendations, "建议增加MaxIdleConns，空闲连接被频繁关闭")
	}

	if stats.MaxLifetimeClosed > int64(stats.OpenConnections)*2 {
		recommendations = append(recommendations, "建议增加ConnMaxLifetime，连接生命周期过短")
	}

	// 基于性能监控给出建议
	perfStats := do.monitor.GetStats()
	if slowRate, ok := perfStats["slow_query_rate"].(float64); ok && slowRate > 10 {
		recommendations = append(recommendations, "慢查询率过高，建议检查索引和查询优化")
	}

	return recommendations
}
