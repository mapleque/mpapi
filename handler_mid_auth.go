package mpapi

import (
	"strconv"

	"github.com/mapleque/kelp/http"
	"github.com/mapleque/kelp/mysql"
)

func (this *Server) AuthToken(c *http.Context) *http.Status {
	token := c.Request.Header.Get("User-Token")
	if token != "" {
		user := &User{}
		if err := this.conn.QueryOne(
			user,
			"SELECT * FROM `user` WHERE token = ? AND expired_at >= NOW()",
			token,
		); err == mysql.NO_DATA_TO_BIND {
			return http.JsonStatus(STATUS_NOT_LOGIN, "invalid token")
		} else if err != nil {
			return http.ErrorStatus(STATUS_INTERNAL_ERROR, err)
		}
		if _, err := this.conn.Execute(
			"UPDATE `user` SET expired_at = DATE_ADD(NOW(),INTERVAL 7 DAY) WHERE id = ? LIMIT 1",
			user.Id,
		); err != nil {
			return http.ErrorStatus(STATUS_INTERNAL_ERROR, err)
		}
		c.Request.Header.Set("User-Id", strconv.FormatInt(user.Id, 10))
		c.Request.Header.Set("User-Openid", user.Openid)
		c.Request.Header.Set("User-Unionid", user.Unionid)

		userExtra := &UserExtra{}
		this.conn.QueryOne(
			userExtra,
			"SELECT * FROM `user_extra` WHERE user_id = ? LIMIT 1",
			user.Id,
		)
		c.Request.Header.Set("User-Extra", userExtra.ToString())

		userMobile := &UserMobile{}
		this.conn.QueryOne(
			userMobile,
			"SELECT * FROM `user_mobile` WHERE user_id = ? LIMIT 1",
			user.Id,
		)
		c.Request.Header.Set("User-Mobile", userMobile.ToString())

		c.Request.Header.Set("uuid", user.Openid)
		c.Set("User", user)
		c.Set("UserExtra", userExtra)
		c.Set("UserMobile", userMobile)
	}
	c.Next()
	return nil
}
