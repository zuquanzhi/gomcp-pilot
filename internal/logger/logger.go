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
	// LogChan 用于发送日志到 TUI 显示。
	// 使用缓冲以避免 TUI 处理慢时阻塞应用。
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

// ChannelSink 实现 zapcore.WriteSyncer，将日志通过信道传输到 TUI。
type ChannelSink struct{}

func (cs *ChannelSink) Write(p []byte) (n int, err error) {
	// 将 JSON 日志条目解析回结构体供 TUI 使用。
	// 这里略微牺牲效率以保持 TUI 与 Zap 内部实现的解耦。
	// 注意：p 包含换行符。
	msg := string(p)

	// 创建简单条目。未来可解析结构化数据。
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

// InitLogger 初始化全局日志，同时写入文件和 LogChan。
func InitLogger() error {
	var err error
	once.Do(func() {
		home, _ := os.UserHomeDir()
		logPath := filepath.Join(home, ".gomcp", "gomcp.log")
		_ = os.MkdirAll(filepath.Dir(logPath), 0755)

		// 文件编码器配置 (JSON)
		encoderConfig := zap.NewProductionEncoderConfig()
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

		// 文件核心
		file, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		fileWriter := zapcore.AddSync(file)
		fileCore := zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			fileWriter,
			zap.InfoLevel,
		)

		// TUI 核心 - 发送结构化 LogEntry
		tuiCore := NewTUICore(zap.DebugLevel)

		core := zapcore.NewTee(fileCore, tuiCore)
		Global = zap.New(core, zap.AddCaller())
	})
	return err
}

// TUICore 自定义 zapcore，发送类型化的 LogEntry 到信道。
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
	return c // 暂未实现 TUI 的 context 字段映射
}

func (c *TUICore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

func (c *TUICore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	// 字段转 map
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
