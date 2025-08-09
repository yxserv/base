package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/yxserv/base/model"
	"github.com/yxserv/base/pkg/logger"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	DSN             string        `json:"dsn"`
	MaxIdleConns    int           `json:"max_idle_conns"`
	MaxOpenConns    int           `json:"max_open_conns"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime"`
}

// GormLogger 实现gorm的日志接口
type GormLogger struct {
	SlowThreshold        time.Duration
	IgnoreRecordNotFound bool
	LogLevel             gormlogger.LogLevel
}

// NewGormLogger 创建GORM日志适配器
func NewGormLogger() *GormLogger {
	return &GormLogger{
		SlowThreshold:        time.Second, // 慢查询阈值
		IgnoreRecordNotFound: true,        // 是否忽略记录未找到错误
		LogLevel:             gormlogger.Info,
	}
}

// LogMode 设置日志级别
func (l *GormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

// Info 实现Info级别日志
func (l *GormLogger) Info(ctx context.Context, msg string, data ...any) {
	if l.LogLevel >= gormlogger.Info {
		logger.Info(fmt.Sprintf(msg, data...))
	}
}

// Warn 实现Warn级别日志
func (l *GormLogger) Warn(ctx context.Context, msg string, data ...any) {
	if l.LogLevel >= gormlogger.Warn {
		logger.Warn(fmt.Sprintf(msg, data...))
	}
}

// Error 实现Error级别日志
func (l *GormLogger) Error(ctx context.Context, msg string, data ...any) {
	if l.LogLevel >= gormlogger.Error {
		logger.Error(fmt.Sprintf(msg, data...))
	}
}

// Trace 实现SQL跟踪日志
func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= gormlogger.Silent {
		return
	}

	// 计算耗时
	elapsed := time.Since(begin)

	// 获取SQL和受影响的行数
	sql, rows := fc()

	// 根据不同情况记录日志
	switch {
	case err != nil && l.LogLevel >= gormlogger.Error && (!errors.Is(err, gorm.ErrRecordNotFound) || !l.IgnoreRecordNotFound):
		// 错误日志
		logger.Error("GORM错误",
			logger.String("sql", sql),
			logger.Float64("耗时", float64(elapsed.Milliseconds())),
			logger.Error2(err))
	case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= gormlogger.Warn:
		// 慢查询警告
		logger.Warn("GORM慢查询",
			logger.String("sql", sql),
			logger.Float64("耗时", float64(elapsed.Milliseconds())),
			logger.Int("影响行数", int(rows)))
	case l.LogLevel == gormlogger.Info:
		// 普通信息
		logger.Info("GORM查询",
			logger.String("sql", sql),
			logger.Float64("耗时", float64(elapsed.Milliseconds())),
			logger.Int("影响行数", int(rows)))
	}
}

// DatabaseManager 数据库管理器
type DatabaseManager struct {
	DB     *gorm.DB
	Config DatabaseConfig
}

// NewDatabaseManager 创建数据库管理器
func NewDatabaseManager(config DatabaseConfig) *DatabaseManager {
	return &DatabaseManager{
		Config: config,
	}
}

// Init 初始化数据库连接
func (dm *DatabaseManager) Init() (*gorm.DB, error) {
	var err error

	// 创建连接 - 性能优化配置
	dm.DB, err = gorm.Open(mysql.Open(dm.Config.DSN), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "",   // 表名前缀
			SingularTable: true, // 单数表名
		},
		Logger: NewGormLogger(),
		// 性能优化配置
		PrepareStmt:                              true, // 预编译SQL语句，提高重复查询性能
		DisableForeignKeyConstraintWhenMigrating: true, // 迁移时禁用外键约束检查，提升速度
		SkipDefaultTransaction:                   true, // 跳过默认事务，提高单条操作性能
	})

	if err != nil {
		return nil, fmt.Errorf("数据库连接失败: %v", err)
	}

	// 配置连接池
	sqlDB, err := dm.DB.DB()
	if err != nil {
		return nil, fmt.Errorf("数据库连接池配置失败: %v", err)
	}

	// 设置连接池参数
	dm.configureConnectionPool(sqlDB)

	// 检查连接是否成功
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("数据库连接检查失败: %v", err)
	}

	// 设置全局数据库连接
	model.SetDB(dm.DB)

	logger.Info("数据库初始化成功")
	return dm.DB, nil
}

// configureConnectionPool 配置数据库连接池
func (dm *DatabaseManager) configureConnectionPool(sqlDB any) {
	if db, ok := sqlDB.(interface {
		SetMaxIdleConns(int)
		SetMaxOpenConns(int)
		SetConnMaxLifetime(time.Duration)
	}); ok {
		// 使用配置参数或优化后的默认值
		maxIdleConns := dm.Config.MaxIdleConns
		if maxIdleConns == 0 {
			maxIdleConns = 50 // 增加空闲连接数，减少连接建立开销
		}

		maxOpenConns := dm.Config.MaxOpenConns
		if maxOpenConns == 0 {
			maxOpenConns = 200 // 增加最大连接数，支持并发操作
		}

		connMaxLifetime := dm.Config.ConnMaxLifetime
		if connMaxLifetime == 0 {
			connMaxLifetime = 10 * time.Minute // 减少连接生命周期，避免长时间占用
		}

		db.SetMaxIdleConns(maxIdleConns)
		db.SetMaxOpenConns(maxOpenConns)
		db.SetConnMaxLifetime(connMaxLifetime)

		logger.Info("数据库连接池配置完成（已优化）",
			logger.Int("max_idle_conns", maxIdleConns),
			logger.Int("max_open_conns", maxOpenConns),
			logger.String("conn_max_lifetime", connMaxLifetime.String()))
	}
}

// AutoMigrateModels 自动迁移指定的模型（性能优化版）
func (dm *DatabaseManager) AutoMigrateModels(models ...any) error {
	if len(models) == 0 {
		return nil
	}

	startTime := time.Now()
	logger.Info("🔄 开始批量迁移模型", logger.Int("模型数量", len(models)))

	// 批量迁移，GORM会优化重复的schema查询
	if err := dm.DB.AutoMigrate(models...); err != nil {
		logger.Error("❌ 批量迁移失败", logger.Error2(err))
		return fmt.Errorf("数据库迁移失败: %v", err)
	}

	duration := time.Since(startTime)
	logger.Info("✅ 批量迁移完成",
		logger.String("耗时", duration.String()),
		logger.Int("模型数量", len(models)))

	return nil
}

// MigrateShardingTables 迁移分表（性能优化版）
func (dm *DatabaseManager) MigrateShardingTables(baseTableName string, modelInstance any) error {
	startTime := time.Now()
	logger.Info("🔄 开始迁移分表", logger.String("base_table", baseTableName))

	err := model.EnsureAllShardingTables(baseTableName, modelInstance)
	if err != nil {
		logger.Error("❌ 分表迁移失败",
			logger.String("base_table", baseTableName),
			logger.Error2(err))
		return err
	}

	duration := time.Since(startTime)
	logger.Info("✅ 分表迁移完成",
		logger.String("base_table", baseTableName),
		logger.String("耗时", duration.String()))

	return nil
}

// Close 关闭数据库连接
func (dm *DatabaseManager) Close() error {
	if dm.DB == nil {
		return nil
	}

	sqlDB, err := dm.DB.DB()
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %v", err)
	}

	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("关闭数据库连接失败: %v", err)
	}

	logger.Info("数据库连接已关闭")
	return nil
}

// GetDB 获取数据库连接
func (dm *DatabaseManager) GetDB() *gorm.DB {
	return dm.DB
}
