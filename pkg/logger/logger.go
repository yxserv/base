package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	Log      *zap.Logger
	HertzLog hlog.FullLogger
	once     sync.Once
)

// HertzLoggerAdapter 实现hertz的hlog.FullLogger接口
type HertzLoggerAdapter struct {
	*zap.Logger
	level  hlog.Level
	output io.Writer
}

// Init 初始化日志
func Init() {
	once.Do(func() {
		// 设置基本配置
		encoderConfig := zap.NewProductionEncoderConfig()
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoderConfig.TimeKey = "time"
		encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
		encoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder
		encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

		// 设置日志级别
		atomicLevel := zap.NewAtomicLevel()
		atomicLevel.SetLevel(zap.InfoLevel)

		// 创建核心 - 只输出到控制台
		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(os.Stdout),
			atomicLevel,
		)

		// 创建日志
		Log = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

		// 创建hertz适配器
		HertzLog = &HertzLoggerAdapter{
			Logger: Log,
			level:  hlog.LevelInfo,
			output: os.Stdout,
		}
	})
}

// 以下是日志记录快捷方法

func Debug(msg string, fields ...zapcore.Field) {
	Log.Debug(msg, fields...)
}

func Info(msg string, fields ...zapcore.Field) {
	Log.Info(msg, fields...)
}

func Warn(msg string, fields ...zapcore.Field) {
	Log.Warn(msg, fields...)
}

func Error(msg string, fields ...zapcore.Field) {
	Log.Error(msg, fields...)
}

func Fatal(msg string, fields ...zapcore.Field) {
	Log.Fatal(msg, fields...)
}

// Field 快捷方法
func String(key, val string) zapcore.Field {
	return zap.String(key, val)
}

func Int(key string, val int) zapcore.Field {
	return zap.Int(key, val)
}

func Float64(key string, val float64) zapcore.Field {
	return zap.Float64(key, val)
}

func Error2(err error) zapcore.Field {
	return zap.Error(err)
}

// SetLevel 实现hlog.FullLogger接口
func (l *HertzLoggerAdapter) SetLevel(level hlog.Level) {
	l.level = level
}

// SetOutput 实现hlog.FullLogger接口
func (l *HertzLoggerAdapter) SetOutput(output io.Writer) {
	l.output = output
}

// HertzLoggerAdapter 实现hlog.FullLogger接口
func (l *HertzLoggerAdapter) Trace(v ...interface{}) {
	l.Debug(fmt.Sprint(v...))
}

func (l *HertzLoggerAdapter) Debug(v ...interface{}) {
	l.Logger.Debug(fmt.Sprint(v...))
}

func (l *HertzLoggerAdapter) Info(v ...interface{}) {
	l.Logger.Info(fmt.Sprint(v...))
}

func (l *HertzLoggerAdapter) Notice(v ...interface{}) {
	l.Logger.Info(fmt.Sprint(v...))
}

func (l *HertzLoggerAdapter) Warn(v ...interface{}) {
	l.Logger.Warn(fmt.Sprint(v...))
}

func (l *HertzLoggerAdapter) Error(v ...interface{}) {
	l.Logger.Error(fmt.Sprint(v...))
}

func (l *HertzLoggerAdapter) Fatal(v ...interface{}) {
	l.Logger.Fatal(fmt.Sprint(v...))
}

func (l *HertzLoggerAdapter) Tracef(format string, v ...interface{}) {
	l.Debug(fmt.Sprintf(format, v...))
}

func (l *HertzLoggerAdapter) Debugf(format string, v ...interface{}) {
	l.Logger.Debug(fmt.Sprintf(format, v...))
}

func (l *HertzLoggerAdapter) Infof(format string, v ...interface{}) {
	l.Logger.Info(fmt.Sprintf(format, v...))
}

func (l *HertzLoggerAdapter) Noticef(format string, v ...interface{}) {
	l.Logger.Info(fmt.Sprintf(format, v...))
}

func (l *HertzLoggerAdapter) Warnf(format string, v ...interface{}) {
	l.Logger.Warn(fmt.Sprintf(format, v...))
}

func (l *HertzLoggerAdapter) Errorf(format string, v ...interface{}) {
	l.Logger.Error(fmt.Sprintf(format, v...))
}

func (l *HertzLoggerAdapter) Fatalf(format string, v ...interface{}) {
	l.Logger.Fatal(fmt.Sprintf(format, v...))
}

func (l *HertzLoggerAdapter) CtxTrace(ctx context.Context, v ...interface{}) {
	l.Debug(fmt.Sprint(v...))
}

func (l *HertzLoggerAdapter) CtxDebug(ctx context.Context, v ...interface{}) {
	l.Logger.Debug(fmt.Sprint(v...))
}

func (l *HertzLoggerAdapter) CtxInfo(ctx context.Context, v ...interface{}) {
	l.Logger.Info(fmt.Sprint(v...))
}

func (l *HertzLoggerAdapter) CtxNotice(ctx context.Context, v ...interface{}) {
	l.Logger.Info(fmt.Sprint(v...))
}

func (l *HertzLoggerAdapter) CtxWarn(ctx context.Context, v ...interface{}) {
	l.Logger.Warn(fmt.Sprint(v...))
}

func (l *HertzLoggerAdapter) CtxError(ctx context.Context, v ...interface{}) {
	l.Logger.Error(fmt.Sprint(v...))
}

func (l *HertzLoggerAdapter) CtxFatal(ctx context.Context, v ...interface{}) {
	l.Logger.Fatal(fmt.Sprint(v...))
}

func (l *HertzLoggerAdapter) CtxTracef(ctx context.Context, format string, v ...interface{}) {
	l.Debug(fmt.Sprintf(format, v...))
}

func (l *HertzLoggerAdapter) CtxDebugf(ctx context.Context, format string, v ...interface{}) {
	l.Logger.Debug(fmt.Sprintf(format, v...))
}

func (l *HertzLoggerAdapter) CtxInfof(ctx context.Context, format string, v ...interface{}) {
	l.Logger.Info(fmt.Sprintf(format, v...))
}

func (l *HertzLoggerAdapter) CtxNoticef(ctx context.Context, format string, v ...interface{}) {
	l.Logger.Info(fmt.Sprintf(format, v...))
}

func (l *HertzLoggerAdapter) CtxWarnf(ctx context.Context, format string, v ...interface{}) {
	l.Logger.Warn(fmt.Sprintf(format, v...))
}

func (l *HertzLoggerAdapter) CtxErrorf(ctx context.Context, format string, v ...interface{}) {
	l.Logger.Error(fmt.Sprintf(format, v...))
}

func (l *HertzLoggerAdapter) CtxFatalf(ctx context.Context, format string, v ...interface{}) {
	l.Logger.Fatal(fmt.Sprintf(format, v...))
}
