package logx

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger

func init() {
	config := zap.NewProductionEncoderConfig()
	config.EncodeTime = zapcore.ISO8601TimeEncoder
	jsonEncoder := zapcore.NewJSONEncoder(config)
	defaultLogLevel := zapcore.DebugLevel // 设置 loglevel

	// logFile, _ := os.OpenFile("./log-test-zap.json", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 06666)
	// or os.Create()
	// writer := io.MultiWriter(logFile, os.Stderr)
	writer := os.Stderr
	syncWriter := zapcore.AddSync(writer)

	logger = zap.New(
		// zapcore.NewCore(fileEncoder, writer, defaultLogLevel),
		zapcore.NewCore(jsonEncoder,
			syncWriter,
			defaultLogLevel),
		// zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	defer logger.Sync()

}

func Info(msg string, fields ...any) {
	fmt.Printf(msg+"\n", fields...)
	// logger.Info(msg)
	// for _, field := range fields {
	// 	logger.Info(field.(string))
	// }
}
func Infof(format string, others ...any) {
	fmt.Printf(format, others...)
	// logger.Info(fmt.Sprintf(format, others...))
}

func Errorf(format string, others ...any) {
	fmt.Printf(format, others...)
	// logger.Error(fmt.Sprintf(format, others...))
}

func Fatal(err error) {
	fmt.Printf("%v", err)
	// logger.Fatal(fmt.Sprintf("%v", err))
	os.Exit(1)
}
