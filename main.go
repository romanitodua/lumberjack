package main

import (
	"gopkg.in/natefinch/lumberjack.v2/lamberjack"
	"log"
)

func main() {
	// Setup the logger with lumberjack
	log.SetOutput(&lamberjack.Logger{
		Filename:   "./test.log",
		MaxSize:    1, // megabytes
		MaxBackups: 3, // keep 3 backup files
		MaxAge:     1, // days
		Compress:   true,
	})

	// Continuously write logs
	for i := 0; i < 50000; i++ {
		log.Printf("This is log line numbsdsdser %d", i)
	}
}
