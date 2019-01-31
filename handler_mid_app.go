package mpapi

import (
	"github.com/mapleque/kelp/http"
	"github.com/mapleque/kelp/mysql"
)

func (this *Server) WXAppCheck(c *http.Context) *http.Status {
	appid := c.Request.Header.Get("User-App")
	if appid == "" {
		appid = c.QueryDefault("appid", "")
	}
	if appid == "" {
		return http.JsonStatus(STATUS_NOT_ALLOW, "invalid app param")
	}
	app, err := NewWXApp(appid, this.conn)
	if err == mysql.NO_DATA_TO_BIND {
		return http.JsonStatus(STATUS_NOT_ALLOW, "need regist app")
	}
	if err != nil {
		return http.ErrorStatus(STATUS_INTERNAL_ERROR, err)
	}
	c.Set("User-App", app)
	c.Next()
	return nil
}
