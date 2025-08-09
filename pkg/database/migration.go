package database

import (
	"fmt"
	"sync"
	"time"

	"github.com/yxserv/base/pkg/logger"
)

// MigrationConfig 迁移配置接口
type MigrationConfig interface {
	// GetStandardModels 获取需要迁移的标准表模型
	GetStandardModels() []any

	// GetShardingModels 获取需要分表的模型配置
	GetShardingModels() []ShardingModel

	// GetCustomMigrations 获取自定义迁移函数
	GetCustomMigrations() []CustomMigration
}

// ShardingModel 分表模型配置
type ShardingModel struct {
	BaseTableName string // 基础表名，如 "user"
	ModelInstance any    // 模型实例，用于表结构迁移
}

// CustomMigration 自定义迁移函数
type CustomMigration struct {
	Name        string                       // 迁移名称
	MigrateFunc func(*DatabaseManager) error // 迁移函数
}

// MigrationManager 迁移管理器
type MigrationManager struct {
	dbManager  *DatabaseManager
	config     MigrationConfig
	tableCache map[string]bool // 表存在性缓存
	indexCache map[string]bool // 索引存在性缓存
	cacheMutex sync.RWMutex    // 缓存读写锁
	startTime  time.Time       // 迁移开始时间
}

// NewMigrationManager 创建迁移管理器
func NewMigrationManager(dbManager *DatabaseManager, config MigrationConfig) *MigrationManager {
	return &MigrationManager{
		dbManager:  dbManager,
		config:     config,
		tableCache: make(map[string]bool),
		indexCache: make(map[string]bool),
		startTime:  time.Now(),
	}
}

// RunMigrations 执行所有迁移
func (mm *MigrationManager) RunMigrations() error {
	mm.startTime = time.Now()
	logger.Info("🚀 开始执行数据库迁移（性能优化版）")

	// 1. 迁移标准表
	if err := mm.migrateStandardTables(); err != nil {
		return fmt.Errorf("标准表迁移失败: %v", err)
	}

	// 2. 迁移分表
	if err := mm.migrateShardingTables(); err != nil {
		return fmt.Errorf("分表迁移失败: %v", err)
	}

	// 3. 执行自定义迁移
	if err := mm.runCustomMigrations(); err != nil {
		return fmt.Errorf("自定义迁移失败: %v", err)
	}

	duration := time.Since(mm.startTime)
	logger.Info("✅ 数据库迁移完成",
		logger.String("总耗时", duration.String()),
		logger.Int("缓存命中_表", len(mm.tableCache)),
		logger.Int("缓存命中_索引", len(mm.indexCache)))
	return nil
}

// migrateStandardTables 迁移标准表
func (mm *MigrationManager) migrateStandardTables() error {
	models := mm.config.GetStandardModels()
	if len(models) == 0 {
		logger.Info("没有需要迁移的标准表")
		return nil
	}

	startTime := time.Now()
	logger.Info("🔄 开始迁移标准表", logger.Int("count", len(models)))

	// 批量迁移，减少单独的schema查询
	if err := mm.dbManager.AutoMigrateModels(models...); err != nil {
		logger.Error("❌ 标准表迁移失败", logger.Error2(err))
		return err
	}

	duration := time.Since(startTime)
	logger.Info("✅ 标准表迁移完成",
		logger.String("耗时", duration.String()),
		logger.Int("表数量", len(models)))
	return nil
}

// migrateShardingTables 迁移分表
func (mm *MigrationManager) migrateShardingTables() error {
	shardingModels := mm.config.GetShardingModels()
	if len(shardingModels) == 0 {
		logger.Info("没有需要迁移的分表")
		return nil
	}

	startTime := time.Now()
	logger.Info("🔄 开始迁移分表", logger.Int("count", len(shardingModels)))

	totalTables := 0
	for _, shardingModel := range shardingModels {
		tableStartTime := time.Now()
		logger.Info("🔄 迁移分表", logger.String("base_table", shardingModel.BaseTableName))

		if err := mm.dbManager.MigrateShardingTables(shardingModel.BaseTableName, shardingModel.ModelInstance); err != nil {
			logger.Error("❌ 分表迁移失败",
				logger.String("base_table", shardingModel.BaseTableName),
				logger.Error2(err))
			return err
		}

		tableDuration := time.Since(tableStartTime)
		// 假设每个基础表有8个分表
		shardCount := 8
		totalTables += shardCount
		logger.Info("✅ 分表迁移完成",
			logger.String("base_table", shardingModel.BaseTableName),
			logger.String("耗时", tableDuration.String()),
			logger.Int("分表数量", shardCount))
	}

	duration := time.Since(startTime)
	logger.Info("✅ 所有分表迁移完成",
		logger.String("总耗时", duration.String()),
		logger.Int("总表数量", totalTables))
	return nil
}

// runCustomMigrations 执行自定义迁移
func (mm *MigrationManager) runCustomMigrations() error {
	customMigrations := mm.config.GetCustomMigrations()
	if len(customMigrations) == 0 {
		logger.Info("没有需要执行的自定义迁移")
		return nil
	}

	logger.Info("开始执行自定义迁移", logger.Int("count", len(customMigrations)))

	for _, migration := range customMigrations {
		logger.Info("执行自定义迁移", logger.String("name", migration.Name))

		if err := migration.MigrateFunc(mm.dbManager); err != nil {
			logger.Error("自定义迁移失败",
				logger.String("name", migration.Name),
				logger.Error2(err))
			return err
		}
	}

	logger.Info("自定义迁移完成")
	return nil
}
