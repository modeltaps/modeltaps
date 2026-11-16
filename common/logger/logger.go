package logger

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/common/utils"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	loggerINFO  = "INFO"
	loggerWarn  = "WARN"
	loggerError = "ERR"
	loggerDEBUG = "DEBUG"
)
const (
	RequestIdKey = "X-Modeltaps-Request-Id"
)

// LogEntry represents a single log entry in memory
type LogEntry struct {
	Timestamp time.Time
	Level     string
	Message   string
}

// LogHistory keeps the latest entries in memory, plus a longer tail of warnings and errors
// so a burst of info lines cannot push them out.
type LogHistory struct {
	entries      []LogEntry
	important    []LogEntry
	mutex        sync.RWMutex
	maxSize      int
	maxImportant int
}

// Global log history: the last 1000 entries of any level and the last 500 warnings and errors.
var logHistory = &LogHistory{
	entries:      make([]LogEntry, 0, 1000),
	maxSize:      1000,
	maxImportant: 500,
}

// AddEntry adds a new log entry to the history and to the problem groups.
func (lh *LogHistory) AddEntry(level, message string) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   strings.TrimRight(message, " \n"),
	}
	problems.add(entry)

	lh.mutex.Lock()
	defer lh.mutex.Unlock()
	lh.entries = append(lh.entries, entry)
	if len(lh.entries) > lh.maxSize {
		lh.entries = lh.entries[1:]
	}
	if l := NormalizeLevel(level); l == LevelError || l == LevelWarn {
		lh.important = append(lh.important, entry)
		if len(lh.important) > lh.maxImportant {
			lh.important = lh.important[1:]
		}
	}
}

// GetLatestEntries returns the latest n entries from the log history (all when n <= 0).
func (lh *LogHistory) GetLatestEntries(n int) []LogEntry {
	lh.mutex.RLock()
	defer lh.mutex.RUnlock()

	if n <= 0 || n > len(lh.entries) {
		n = len(lh.entries)
	}
	result := make([]LogEntry, n)
	copy(result, lh.entries[len(lh.entries)-n:])
	return result
}

// allEntries returns everything kept in memory in log order: the warnings and errors older
// than the main buffer, then the main buffer.
func (lh *LogHistory) allEntries() []LogEntry {
	lh.mutex.RLock()
	defer lh.mutex.RUnlock()

	var result []LogEntry
	if len(lh.entries) > 0 {
		oldest := lh.entries[0].Timestamp
		for _, e := range lh.important {
			if e.Timestamp.Before(oldest) {
				result = append(result, e)
			}
		}
	} else {
		result = append(result, lh.important...)
	}
	return append(result, lh.entries...)
}

var Logger *zap.Logger

var defaultLogDir = "./logs"

func SetupLogger() {
	logDir := getLogDir()
	if logDir == "" {
		return
	}

	writeSyncer := getLogWriter(logDir)

	encoder := getEncoder()

	core := zapcore.NewCore(
		encoder,
		writeSyncer,
		zap.NewAtomicLevelAt(getLogLevel()),
	)
	Logger = zap.New(core, zap.AddCaller())
}

func getEncoder() zapcore.Encoder {
	encodeConfig := zap.NewProductionEncoderConfig()

	encodeConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006/01/02 - 15:04:05")
	encodeConfig.TimeKey = "time"
	encodeConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encodeConfig.EncodeCaller = zapcore.ShortCallerEncoder

	encodeConfig.EncodeDuration = zapcore.StringDurationEncoder

	// logs.format=json writes one JSON object per line for log collectors (Loki, ELK, ...).
	if strings.EqualFold(utils.GetOrDefault("logs.format", "text"), "json") {
		encodeConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encodeConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
		return zapcore.NewJSONEncoder(encodeConfig)
	}
	return zapcore.NewConsoleEncoder(encodeConfig)
}

func getLogWriter(logDir string) zapcore.WriteSyncer {
	filename := utils.GetOrDefault("logs.filename", "modeltaps.log")
	logPath := filepath.Join(logDir, filename)

	maxsize := utils.GetOrDefault("logs.max_size", 100)
	maxAge := utils.GetOrDefault("logs.max_age", 7)
	maxBackup := utils.GetOrDefault("logs.max_backup", 10)
	compress := utils.GetOrDefault("logs.compress", false)

	lumberJackLogger := &lumberjack.Logger{
		Filename:   logPath,   // 文件位置
		MaxSize:    maxsize,   // 进行切割之前,日志文件的最大大小(MB为单位)
		MaxAge:     maxAge,    // 保留旧文件的最大天数
		MaxBackups: maxBackup, // 保留旧文件的最大个数
		Compress:   compress,  // 是否压缩/归档旧文件
	}

	return zapcore.NewMultiWriteSyncer(zapcore.AddSync(lumberJackLogger), zapcore.AddSync(os.Stderr))
}

func getLogLevel() zapcore.Level {
	logLevel := viper.GetString("log_level")
	switch logLevel {
	case "debug":
		return zap.DebugLevel
	case "info":
		return zap.InfoLevel
	case "warn":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	case "panic":
		return zap.PanicLevel
	case "fatal":
		return zap.FatalLevel
	default:
		return zap.InfoLevel
	}

}

func getLogDir() string {
	logDir := viper.GetString("log_dir")
	if logDir == "" {
		logDir = defaultLogDir
	}

	var err error
	logDir, err = filepath.Abs(logDir)
	if err != nil {
		log.Fatal(err)
		return ""
	}

	if !utils.IsFileExist(logDir) {
		err = os.Mkdir(logDir, 0777)
		if err != nil {
			log.Fatal(err)
			return ""
		}
	}

	return logDir
}

func SysLog(s string) {
	message := "[SYS] | " + s

	// Add to in-memory log history
	logHistory.AddEntry(loggerINFO, message)

	// Logger 未初始化时兜底到标准库 log，避免 nil deref panic
	if Logger == nil {
		log.Println(message)
		return
	}

	// Write to log file
	entry := zapcore.Entry{
		Level:   zapcore.InfoLevel,
		Time:    time.Now(),
		Message: message,
	}

	// 使用 Logger 的核心来直接写入日志，绕过等级检查
	if ce := Logger.Core().With([]zapcore.Field{}); ce != nil {
		ce.Write(entry, nil)
	}
}

func SysError(s string) {
	message := "[SYS] | " + s

	// Add to in-memory log history
	logHistory.AddEntry(loggerError, message)

	// Logger 未初始化时兜底到标准库 log，避免 nil deref panic
	if Logger == nil {
		log.Println(message)
		return
	}

	// Write to log file
	Logger.Error(message)
}

func SysDebug(s string) {
	message := "[SYS] | " + s

	// Add to in-memory log history
	logHistory.AddEntry(loggerDEBUG, message)

	// Logger 未初始化时兜底到标准库 log，避免 nil deref panic
	if Logger == nil {
		log.Println(message)
		return
	}

	// Write to log file
	Logger.Debug(message)
}

func LogInfo(ctx context.Context, msg string) {
	logHelper(ctx, loggerINFO, msg)
}

func LogWarn(ctx context.Context, msg string) {
	logHelper(ctx, loggerWarn, msg)
}

func LogError(ctx context.Context, msg string) {
	logHelper(ctx, loggerError, msg)
}

func LogDebug(ctx context.Context, msg string) {
	logHelper(ctx, loggerDEBUG, msg)
}

func logHelper(ctx context.Context, level string, msg string) {
	id := "unknown"
	userId := 0
	if ctx != nil {
		if ctxId, ok := ctx.Value(RequestIdKey).(string); ok {
			id = ctxId
		}
		if ctxUserId, ok := ctx.Value("id").(int); ok {
			userId = ctxUserId
		}
	}

	var logMsg string
	if userId > 0 {
		logMsg = fmt.Sprintf("%s | user_id=%d %s \n", id, userId, msg)
	} else {
		logMsg = fmt.Sprintf("%s | %s \n", id, msg)
	}

	// Add to in-memory log history
	logHistory.AddEntry(level, logMsg)

	// Logger 未初始化时兜底到标准库 log，避免 nil deref panic
	if Logger == nil {
		log.Print(logMsg)
		return
	}

	// Write to log file
	switch level {
	case loggerINFO:
		Logger.Info(logMsg)
	case loggerWarn:
		Logger.Warn(logMsg)
	case loggerError:
		Logger.Error(logMsg)
	case loggerDEBUG:
		Logger.Debug(logMsg)
	default:
		Logger.Info(logMsg)
	}
}

func FatalLog(v ...any) {
	message := fmt.Sprintf("[FATAL] %v | %v \n", time.Now().Format("2006/01/02 - 15:04:05"), v)

	// Add to in-memory log history
	logHistory.AddEntry("FATAL", message)

	// Logger 未初始化时兜底到标准库 log，避免 nil deref panic
	if Logger == nil {
		log.Print(message)
		os.Exit(1)
	}

	// Write to log file
	Logger.Fatal(message)
	// t := time.Now()
	// _, _ = fmt.Fprintf(gin.DefaultErrorWriter, "[FATAL] %v | %v \n", t.Format("2006/01/02 - 15:04:05"), v)
	os.Exit(1)
}
