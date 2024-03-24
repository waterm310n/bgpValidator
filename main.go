package main

import (
	"log/slog"

	"gopkg.in/natefinch/lumberjack.v2"
	"bgpvalidator/cmd"
)

//设置滚动日志
func setLog() {
	log := &lumberjack.Logger{
		Filename:   "./logs/excute.log", // 日志文件的位置
		MaxSize:    10,                  // 文件最大尺寸（以MB为单位）
		MaxBackups: 3,                   // 保留的最大旧文件数量
		MaxAge:     28,                  // 保留旧文件的最大天数
		Compress:   true,                // 是否压缩/归档旧文件
		LocalTime:  true,                // 使用本地时间创建时间戳
	}

	logger := slog.New(slog.NewTextHandler(log, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelInfo,
	}))
	slog.SetDefault(logger)
}

func main() {
	setLog()
	cmd.Execute()
}
