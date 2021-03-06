package mpapi

import (
	"fmt"

	"github.com/mapleque/kelp/http"
	"github.com/mapleque/kelp/logger"
	"github.com/mapleque/kelp/mysql"
)

const (
	STATUS_NOT_ALLOW      = 10000
	STATUS_NOT_LOGIN      = 10001
	STATUS_INTERNAL_ERROR = 10002
)

type Server struct {
	conn   mysql.Connector
	logger logger.Loggerer
}

func New(conn mysql.Connector, log logger.Loggerer) *Server {
	return &Server{
		conn:   conn,
		logger: log,
	}
}

func (this *Server) Run(host string) {
	server := http.New(host)
	server.Use(http.LogHandler)
	server.Use(http.RecoveryHandler)
	server.Use(http.TraceHandler)

	this.initRouter(server)
	fmt.Println("http server listen on", host)
	server.Run()
}
