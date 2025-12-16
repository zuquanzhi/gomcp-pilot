package logger

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// LogChan is the channel where logs are sent for TUI display.
	// Buffered to avoid blocking the application if TUI is slow.
	LogChan = make(chan LogEntry, 100)
	atom    = zap.NewAtomicLevel()
	once    sync.Once
	Global  *zap.Logger
)

type LogEntry struct {
	Level     string
	Message   string
	Timestamp time.Time
	Fields    map[string]interface{}
}

// ChannelSink implements zapcore.WriteSyncer to pipe logs to the TUI channel.
type ChannelSink struct{}

func (cs *ChannelSink) Write(p []byte) (n int, err error) {
	// Parse the JSON log entry back to struct for the TUI
	// This is a bit inefficient but keeps the TUI decoupled from Zap internals.
	// For high performance, we'd implement a custom Core.
	// For this local tool, it's fine.
	// note: p includes the newline.
	msg := string(p)
	
	// Create a simple entry. We can improve parsing if we want structured data in TUI.
	// For now, let's just send the raw text or try to make it look decent.
	LogChan <- LogEntry{
		Level:     "INFO", // Default, hard to parse back without custom encoder
		Message:   msg,
		Timestamp: time.Now(),
	}
	return len(p), nil
}

func (cs *ChannelSink) Sync() error {
	return nil
}

// InitLogger initializes the global logger.
// It writes to a log file and the LogChan.
func InitLogger() error {
	var err error
	once.Do(func() {
		home, _ := os.UserHomeDir()
		logPath := filepath.Join(home, ".gomcp", "gomcp.log")
		_ = os.MkdirAll(filepath.Dir(logPath), 0755)

		// File Encoder config (JSON)
		encoderConfig := zap.NewProductionEncoderConfig()
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

		// File Core
		file, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		fileWriter := zapcore.AddSync(file)
		fileCore := zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			fileWriter,
			zap.InfoLevel,
		)

		// TUI Core - using a Console encoder for human readability in the TUI
		// heavily simplified for now.
		// Actually, let's make a custom Core for the TUI that sends structured LogEntry directly.
		tuiCore := NewTUICore(zap.DebugLevel)

		core := zapcore.NewTee(fileCore, tuiCore)
		Global = zap.New(core, zap.AddCaller())
	})
	return err
}

// TUICore is a custom zapcore that sends typed LogEntry structs to the channel.
type TUICore struct {
	zapcore.LevelEnabler
	encoder zapcore.Encoder
}

func NewTUICore(level zapcore.LevelEnabler) *TUICore {
	return &TUICore{
		LevelEnabler: level,
		encoder:      zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
	}
}

func (c *TUICore) With(fields []zapcore.Field) zapcore.Core {
	return c // Not implementing context fields for TUI simplicity yet
}

func (c *TUICore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

func (c *TUICore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	// Convert fields to map
	fieldMap := make(map[string]interface{})
	enc := zapcore.NewMapObjectEncoder()
	for _, f := range fields {
		f.AddTo(enc)
	}
	for k, v := range enc.Fields {
		fieldMap[k] = v
	}

	LogChan <- LogEntry{
		Level:     ent.Level.String(),
		Message:   ent.Message,
		Timestamp: ent.Time,
		Fields:    fieldMap,
	}
	return nil
}

func (c *TUICore) Sync() error {
	return nil
}
