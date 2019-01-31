package mpapi

import (
	"github.com/mapleque/kelp/http"
)

type NotifyParam struct {
	Description     string `json:"description" valid:"(0,),message=invalid description"`
	TemplateId      string `json:"template_id" valid:"(0,),message=invalid template_id"`
	FormId          string `json:"formid" valid:"(0,),message=invalid formid"`
	ActiveAt        string `json:"active_at" valid:"@date,message=invalid active_at"`
	Page            string `json:"page"`
	Data            string `json:"data"`
	EmphasisKeyword string `json:"emphasis_keyword"`
}

func (this *Server) Notify(in *NotifyParam, out interface{}, c *http.Context) *http.Status {
	userInter, exist := c.Get("User")
	if !exist {
		return http.JsonStatus(STATUS_NOT_LOGIN, "user must login")
	}
	user := userInter.(*User)
	if _, err := this.conn.Insert(
		"INSERT INTO mp_notify (description, user_id, appid, template_id, page, form_id, data, emphasis_keyword, status, active_at) VALUES (?,?,?,?,?,?,?,?,?,?)",
		in.Description,
		user.Id,
		user.Appid,
		in.TemplateId,
		in.Page,
		in.FormId,
		in.Data,
		in.EmphasisKeyword,
		MP_NOTIFY_STATUS_NEW,
		in.ActiveAt,
	); err != nil {
		return http.ErrorStatus(STATUS_INTERNAL_ERROR, err)
	}
	return nil
}
