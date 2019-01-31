package mpapi

import (
	"github.com/mapleque/kelp/http"
	"github.com/mapleque/kelp/mysql"
	"github.com/mapleque/kelp/str"
)

type LoginParam struct {
	Code string `json:"code" valid:"message=invalid code"`
}

type LoginResponse struct {
	Token  string `json:"token"`
	Openid string `json:"openid"`
}

func (this *Server) Login(in *LoginParam, out *LoginResponse, c *http.Context) *http.Status {
	app := c.MustGet("User-App").(*WXApp)
	// 使用in.Code去微信换session，同时可以获取openid,unionid
	auth, err := app.Jscode2session(in.Code)
	if err != nil {
		return http.ErrorStatus(STATUS_INTERNAL_ERROR, err)
	}
	c.Request.Header.Set("uuid", auth.Openid)
	token := str.RandMd5()
	user := &User{}

	if err := this.conn.QueryOne(
		user,
		"SELECT * FROM `user` WHERE appid = ? AND openid = ?",
		app.Appid,
		auth.Openid,
	); err == mysql.NO_DATA_TO_BIND {
		_, err := this.conn.Insert(
			"INSERT INTO `user` (appid,openid,unionid,sessionkey,token,expired_at) VALUES (?,?,?,?,?,DATE_ADD(NOW(),INTERVAL 7 DAY))",
			app.Appid,
			auth.Openid,
			auth.Unionid,
			auth.Sessionkey,
			token,
		)
		if err != nil {
			return http.ErrorStatus(STATUS_INTERNAL_ERROR, err)
		}
	} else if err != nil {
		return http.ErrorStatus(STATUS_INTERNAL_ERROR, err)
	} else {
		if auth.Unionid == "" {
			if _, err := this.conn.Execute(
				"UPDATE `user` SET token = ?, sessionkey = ?, expired_at = DATE_ADD(NOW(),INTERVAL 7 DAY) WHERE id = ? LIMIT 1",
				token,
				auth.Sessionkey,
				user.Id,
			); err != nil {
				return http.ErrorStatus(STATUS_INTERNAL_ERROR, err)
			}
		} else {
			if _, err := this.conn.Execute(
				"UPDATE `user` SET token = ?, sessionkey = ?, unionid = ?, expired_at = DATE_ADD(NOW(),INTERVAL 7 DAY) WHERE id = ? LIMIT 1",
				token,
				auth.Sessionkey,
				auth.Unionid,
				user.Id,
			); err != nil {
				return http.ErrorStatus(STATUS_INTERNAL_ERROR, err)
			}
		}
	}

	out.Token = token
	out.Openid = auth.Openid
	return nil
}
