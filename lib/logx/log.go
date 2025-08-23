package logx

import (
	"fmt"
	"os"
	"strings"

	"github.com/ruandao/distribute-im-pkg/lib/xerr"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var _logger *zap.Logger

// 初始化日志
func NewLogger() (*zap.Logger, error) {
	config := zap.NewProductionEncoderConfig()
	config.EncodeTime = zapcore.ISO8601TimeEncoder
	consoleEncoder := zapcore.NewConsoleEncoder(config)
	jsonEncoder := zapcore.NewJSONEncoder(config)
	_ = jsonEncoder
	defaultLogLevel := zapcore.DebugLevel // 设置 loglevel

	// logFile, _ := os.OpenFile("./log-test-zap.json", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 06666)
	// or os.Create()
	// writer := io.MultiWriter(logFile, os.Stderr)
	writer := os.Stderr
	syncWriter := zapcore.AddSync(writer)

	logger := zap.New(
		// zapcore.NewCore(fileEncoder, writer, defaultLogLevel),
		zapcore.NewCore(
			// jsonEncoder,
			consoleEncoder,
			syncWriter,
			defaultLogLevel,
		),
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	defer logger.Sync()
	return logger, nil
}

func init() {
	var err error
	_logger, err = NewLogger()
	if err != nil {
		fmt.Printf("%v", xerr.NewXError(err))
	}
}

func DebugX(msg string) func(fields ...any) {
	// fmt.Printf(msg+"\n", fields...)
	return func(fields ...any) {
		sList := []string{msg}
		for _, f := range fields {
			sList = append(sList, fmt.Sprintf("%v", f))
		}
		_logger.Debug(strings.Join(sList, " "))
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
func InfoX(tag string,) func(others...any) {
	return func(others ...any) {
		// fmt.Printf(format, others...)
		_logger.Info(fmt.Sprintf("%v %v", tag, others))
	}
}

func Errorf(format string, others ...any) {
	// fmt.Printf("Error:"+format, others...)
	_logger.Error(fmt.Sprintf(format, others...))
}
func Error(format string, others ...any) {
	// Errorf(format, others...)
	_logger.Error(fmt.Sprintf(format, others...))
}

func Fatal(err error) {
	// fmt.Printf("%v", err)
	_logger.Fatal(fmt.Sprintf("%v", err))
	os.Exit(1)
}
