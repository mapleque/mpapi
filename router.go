package mpapi

import (
	"github.com/mapleque/kelp/http"
)

func (this *Server) initRouter(root *http.Server) {
	http.RegisterValidFunc("date", http.ValidRegexpWrapper(`^\d{4}-\d{2}-\d{2} \d{2}\:\d{2}\:\d{2}$`))

	root.Use(this.WXAppCheck) // 认证微信appid
	root.Handle("登陆", "/login", this.Login)
	root.Handle("反向代理用户认证", "/u_proxy", this.AuthToken, this.UProxy)
	root.Handle("代理请求微信生成二维码", "/wxcode", this.AuthToken, this.WXCode)
	root.Handle("上传敏感信息", "/credentials", this.AuthToken, this.Credentials)
	root.Handle("上传手机号", "/mobile", this.AuthToken, this.Mobile)
	root.Handle("推送消息", "/notify", this.AuthToken, this.Notify)

	root.Handle("接收客服消息", "/message_recieve", this.MessageRecieve)
	root.Handle("回复客服消息", "/message_reply", this.MessageReply)
}
