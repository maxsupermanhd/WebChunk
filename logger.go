package main

import (
	"io"
	"log"

	"github.com/gorilla/handlers"
	"github.com/natefinch/lumberjack"
)

func customLogger(_ io.Writer, params handlers.LogFormatterParams) {
	r := params.Request
	ip := r.Header.Get("CF-Connecting-IP")
	if ip == "" {
		ip = r.RemoteAddr
	}
	geo := r.Header.Get("CF-IPCountry")
	if geo == "" {
		geo = "??"
	}
	ua := r.Header.Get("user-agent")
	log.Println("["+geo+" "+ip+"]", r.Method, params.StatusCode, r.RequestURI, "["+ua+"]")
}

func createLogger() *lumberjack.Logger {
	return &lumberjack.Logger{
		Filename: cfg.GetDSString("./logs/WebChunk.log", "logs_path"),
		MaxSize:  10,
		Compress: true,
	}
}
