package logx

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/ruandao/distribute-im-pkg/lib/xerr"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var _logger *zap.Logger

// 创建日志编码器
func getEncoder() zapcore.Encoder {
	// 配置日志格式选项
	encoderConfig := zap.NewProductionEncoderConfig()
	// 设置时间格式
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	// 日志级别使用大写字母
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	// 函数名和行号格式
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	// 使用控制台编码器（也可以使用JSON编码器zapcore.NewJSONEncoder）
	return zapcore.NewConsoleEncoder(encoderConfig)
	// return zapcore.NewJSONEncoder(encoderConfig)
}
// 创建文件写入器
func getLogWriter(filePath string) zapcore.WriteSyncer {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		panic(err)
	}
	// 同时输出到控制台和文件
	return zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(file))
}

var dynamicWriterMap sync.Map

func _getLogger(suffix string) (*zap.Logger, error) {
	baseName := "_log/log."
	fName := baseName + suffix
	_logger, exist := dynamicWriterMap.Load(fName)
	if exist {
		return _logger.(*zap.Logger), nil
	}


	// 创建日志文件写入器
	allWriter := getLogWriter(fName)

		// 设置日志格式
	encoder := getEncoder()

	// 设置不同级别对应的写入器
	// allCore 处理所有级别日志（Debug及以上）
	nameCore := zapcore.NewCore(encoder, allWriter, zapcore.DebugLevel)
	// 组合多个Core
	core := zapcore.NewTee(nameCore)

	// 创建Logger实例
	logger := zap.New(core, 
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	) // 添加调用者信息

	defer logger.Sync()
	return logger, nil
}

// 初始化日志
func NewLogger() (*zap.Logger, error) {

		// 定义两个日志文件的路径
	allLogPath := "_log/log.all"
	infoLogPath := "_log/log.info"
	warnLogPath := "_log/log.warn"
	errorLogPath := "_log/log.error"

	// 创建日志文件写入器
	allWriter := getLogWriter(allLogPath)
	infoWriter := getLogWriter(infoLogPath)
	warnWriter := getLogWriter(warnLogPath)
	errorWriter := getLogWriter(errorLogPath)

		// 设置日志格式
	encoder := getEncoder()

	// 设置不同级别对应的写入器
	// allCore 处理所有级别日志（Debug及以上）
	allCore := zapcore.NewCore(encoder, allWriter, zapcore.DebugLevel)
	infoCore := zapcore.NewCore(encoder, infoWriter, zapcore.InfoLevel)
	warnCore := zapcore.NewCore(encoder, warnWriter, zapcore.WarnLevel)
	// errorCore 只处理Error及以上级别日志
	errorCore := zapcore.NewCore(encoder, errorWriter, zapcore.ErrorLevel)

	// 组合多个Core
	baseCore := zapcore.NewTee(allCore, infoCore, warnCore, errorCore)

	// 创建Logger实例
	logger := zap.New(baseCore, 
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	) // 添加调用者信息

	// 测试日志输出
	logger.Debug("这是一条Debug级别的日志")
	logger.Info("这是一条Info级别的日志")
	logger.Warn("这是一条Warn级别的日志")
	logger.Error("这是一条Error级别的日志")
	// logger.Fatal("这是一条Fatal级别的日志，会导致程序退出")

	defer logger.Sync()
	return logger, nil
}

func init() {
	var err error
	_logger, err = NewLogger()
	if err != nil {
		fmt.Printf("%+v", xerr.NewXError(err))
	}
}


func X(fSuffix string, level string) func(args ...any) {
	return func(args ...any) {
		logger, err := _getLogger(fSuffix)
		if err != nil {
			fmt.Printf("获取日志管道失败 %v %v level: %v args: %v\n",fSuffix, err, level, args)
			return 
		}
		switch level {
		case "debug":
			logger.Debug(fmt.Sprint(args...))
			DebugX("")(args...)
		case "info":
			logger.Info(fmt.Sprint(args...))
			InfoX("")(args...)
		case "warn":
			logger.Warn(fmt.Sprint(args...))
			WarnX("")(args...)
		case "error":
			logger.Error(fmt.Sprint(args...))
			ErrorX("")(args...)
		default:
			ErrorX(fSuffix)("不支持日志级别", level, xerr.NewError(""))
		}
	}

}
// func DebugX(msg string) func(fields ...any) {
// 	// fmt.Printf(msg+"\n", fields...)
// 	return func(fields ...any) {
// 		sList := []string{msg}
// 		for _, f := range fields {
// 			sList = append(sList, fmt.Sprintf("%+v", f))
// 		}
// 		_logger.Debug(strings.Join(sList, " "))
// 	}
// }
func Debug(msg string, fields ...any) {
	// fmt.Printf(msg+"\n", fields...)
	_logger.Debug(msg)
	for _, field := range fields {
		_logger.Debug(field.(string))
	}
}
func Debugf(format string, others ...any) {
	// fmt.Printf(format, others...)
	_logger.Debug(fmt.Sprintf(format, others...))
}
func DebugX(tag string) func(others ...any) {
	return func(others ...any) {
		// fmt.Printf(format, others...)
		_logger.Debug(fmt.Sprintf("%+v %+v", tag, others))
	}
}

func Info(msg string, fields ...any) {
	// fmt.Printf(msg+"\n", fields...)
	_logger.Info(msg)
	for _, field := range fields {
		_logger.Info(field.(string))
	}
}
func Infof(format string, others ...any) {
	// fmt.Printf(format, others...)
	_logger.Info(fmt.Sprintf(format, others...))
}
func InfoX(tag string) func(others ...any) {
	return func(others ...any) {
		// fmt.Printf(format, others...)
		_logger.Info(fmt.Sprintf("%+v %+v", tag, others))
	}
}

func Warn(msg string, fields ...any) {
	// fmt.Printf(msg+"\n", fields...)
	_logger.Warn(msg)
	for _, field := range fields {
		_logger.Warn(field.(string))
	}
}
func Warnf(format string, others ...any) {
	// fmt.Printf(format, others...)
	_logger.Warn(fmt.Sprintf(format, others...))
}
func WarnX(tag string) func(others ...any) {
	return func(others ...any) {
		// fmt.Printf(format, others...)
		_logger.Warn(fmt.Sprintf("%+v %+v", tag, others))
	}
}

func ErrorX(msg string) func(fields ...any) {
	// fmt.Printf(msg+"\n", fields...)
	return func(fields ...any) {
		sList := []string{msg}
		for _, f := range fields {
			sList = append(sList, fmt.Sprintf("%+v", f))
		}
		_logger.Error(strings.Join(sList, " "))
	}
}
func Errorf(format string, others ...any) {
	// fmt.Printf("Error:"+format, others...)
	_logger.Error(fmt.Sprintf(format, others...))
}

type RetArgs []any

func (retArgs RetArgs) LastErr() error {
	if len(retArgs) == 0 {
		return nil
	}
	last := retArgs[len(retArgs)-1]
	if err, ok := last.(error); ok {
		return err
	}
	return nil
}

func WhenErr(msg string) func(args ...any) (retArgs RetArgs) {
	return func(args ...any) (retArgs RetArgs) {
		if args[len(args)-1] != nil {
			ErrorX(msg)(retArgs...)
		}
		return args
	}
}
func Error(format string, others ...any) {
	// Errorf(format, others...)
	_logger.Error(fmt.Sprintf(format, others...))
}

func Fatal(err error) {
	// fmt.Printf("%v", err)
	_logger.Fatal(fmt.Sprintf("%+v", err))
	os.Exit(1)
}
