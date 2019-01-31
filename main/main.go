package main

import (
	"fmt"
	"os"

	service "github.com/mapleque/mpapi"

	"github.com/mapleque/kelp/config"
	"github.com/mapleque/kelp/http"
	"github.com/mapleque/kelp/logger"
	"github.com/mapleque/kelp/mysql"
)

func main() {
	config.AddConfiger(config.ENV, "config", "")
	conf := config.Use("config")

	logdir := conf.Get("LOG_DIR") + conf.Get("HOSTNAME")
	if logdir != "" {
		if err := os.MkdirAll(logdir, 0777); err != nil {
			fmt.Println("load log dir error", err)
			os.Exit(2)
		}

		logger.Add("http", logdir+"/http.log").SetTagOutput(logger.DEBUG, false)
	}

	// init database
	log := logger.Get("http")
	mysql.SetLogger(log)
	http.SetLogger(log)
	if err := mysql.AddDB(
		"database",
		conf.Get("DATABASE_DSN"),
		conf.Int("DATABASE_MAX_CONN"),
		conf.Int("DATABASE_MAX_IDLE"),
	); err != nil {
		fmt.Println("invalid database configure", err)
		os.Exit(2)
	}

	// init service
	conn := mysql.Get("database")
	ss := service.New(
		conn,
		log,
	)

	// boot service
	go ss.Run(
		"0.0.0.0:" + conf.Get("HTTP"),
	)

	ns := service.NewNotifyServer(conn)
	go ns.StartNotify(conf.Int("NOTIFY_MAX_THREAD"))
	// waiting for exit
	select {}

}
