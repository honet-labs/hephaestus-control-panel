package logger

import (
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Module    string                 `json:"module"`
	Message   string                 `json:"message"`
	Error     string                 `json:"error,omitempty"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

var (
	appLogger    zerolog.Logger
	errorLogger  zerolog.Logger
	subscribers  = make(map[chan LogEntry]struct{})
	subLock      sync.RWMutex
	recentBuffer []LogEntry
	bufferLock   sync.RWMutex
	maxBuffer    = 200
)

// InitLogger initializes the structured logger with console, file rotation, and in-memory pub-sub
func InitLogger(logsDir string) {
	_ = os.MkdirAll(logsDir, 0755)

	appLogPath := filepath.Join(logsDir, "app.log")
	errorLogPath := filepath.Join(logsDir, "error.log")

	appRollingFile := &lumberjack.Logger{
		Filename:   appLogPath,
		MaxSize:    50, // megabytes
		MaxBackups: 7,
		MaxAge:     14, // days
		Compress:   true,
	}

	errorRollingFile := &lumberjack.Logger{
		Filename:   errorLogPath,
		MaxSize:    50,
		MaxBackups: 7,
		MaxAge:     30,
		Compress:   true,
	}

	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "2006-01-02 15:04:05",
	}

	multiWriter := io.MultiWriter(consoleWriter, appRollingFile)

	appLogger = zerolog.New(multiWriter).With().Timestamp().Logger()
	errorLogger = zerolog.New(errorRollingFile).With().Timestamp().Logger()
}

func broadcast(entry LogEntry) {
	bufferLock.Lock()
	recentBuffer = append(recentBuffer, entry)
	if len(recentBuffer) > maxBuffer {
		recentBuffer = recentBuffer[1:]
	}
	bufferLock.Unlock()

	subLock.RLock()
	defer subLock.RUnlock()
	for ch := range subscribers {
		select {
		case ch <- entry:
		default:
		}
	}
}

// Subscribe returns a channel of live LogEntries and an unsubscribe function
func Subscribe() (<-chan LogEntry, func()) {
	ch := make(chan LogEntry, 50)
	subLock.Lock()
	subscribers[ch] = struct{}{}
	subLock.Unlock()

	unsubscribe := func() {
		subLock.Lock()
		delete(subscribers, ch)
		close(ch)
		subLock.Unlock()
	}

	return ch, unsubscribe
}

// GetRecentLogs returns the buffered recent logs
func GetRecentLogs() []LogEntry {
	bufferLock.RLock()
	defer bufferLock.RUnlock()
	copied := make([]LogEntry, len(recentBuffer))
	copy(copied, recentBuffer)
	return copied
}

func Info(module, msg string, fields ...map[string]interface{}) {
	event := appLogger.Info().Str("module", module)
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Module:    module,
		Message:   msg,
	}

	if len(fields) > 0 && fields[0] != nil {
		entry.Fields = fields[0]
		for k, v := range fields[0] {
			event = event.Interface(k, v)
		}
	}

	event.Msg(msg)
	broadcast(entry)
}

func Warn(module, msg string, fields ...map[string]interface{}) {
	event := appLogger.Warn().Str("module", module)
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     "WARN",
		Module:    module,
		Message:   msg,
	}

	if len(fields) > 0 && fields[0] != nil {
		entry.Fields = fields[0]
		for k, v := range fields[0] {
			event = event.Interface(k, v)
		}
	}

	event.Msg(msg)
	broadcast(entry)
}

func Error(module, msg string, err error, fields ...map[string]interface{}) {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	event := appLogger.Error().Str("module", module)
	errEvent := errorLogger.Error().Str("module", module)

	if err != nil {
		event = event.Err(err)
		errEvent = errEvent.Err(err)
	}

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     "ERROR",
		Module:    module,
		Message:   msg,
		Error:     errMsg,
	}

	if len(fields) > 0 && fields[0] != nil {
		entry.Fields = fields[0]
		for k, v := range fields[0] {
			event = event.Interface(k, v)
			errEvent = errEvent.Interface(k, v)
		}
	}

	event.Msg(msg)
	errEvent.Msg(msg)
	broadcast(entry)
}

func Debug(module, msg string, fields ...map[string]interface{}) {
	event := appLogger.Debug().Str("module", module)
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     "DEBUG",
		Module:    module,
		Message:   msg,
	}

	if len(fields) > 0 && fields[0] != nil {
		entry.Fields = fields[0]
		for k, v := range fields[0] {
			event = event.Interface(k, v)
		}
	}

	event.Msg(msg)
	broadcast(entry)
}

func Fatal(module, msg string, err error) {
	Error(module, msg, err)
	fmt.Fprintf(os.Stderr, "FATAL [%s]: %s - %v\n", module, msg, err)
	os.Exit(1)
}
