package mpapi

import (
	"fmt"

	"github.com/mapleque/kelp/http"
	"github.com/mapleque/kelp/str"
)

type Credential struct {
	RawData       string `json:"raw_data" valid:"(0,),message=invalid param"`
	Signature     string `json:"signature" valid:"(0,),message=invalid param"`
	EncryptedData string `json:"encrypted_data" valid:"(0,),message=invalid param"`
	IV            string `json:"iv" valid:"(0,),message=invalid param"`
}

type UserInfo struct {
	Openid    string `json:"openId" column:"openid"`
	UnionId   string `json:"unionId" column:"unionid"`
	NickName  string `json:"nickName" column:"nickname"`
	AvatarUrl string `json:"avatarUrl" column:"avatar_url"`
}

func (this *Server) Credentials(in *Credential, out interface{}, c *http.Context) *http.Status {
	userInter, exist := c.Get("User")
	if !exist {
		return http.JsonStatus(STATUS_NOT_LOGIN, "must login")
	}
	user := userInter.(*User)
	// 解密
	data, err := str.AesCbcDecrypt(
		str.Base64Decode(user.Sessionkey),
		str.Base64Decode(in.IV),
		str.Base64Decode(in.EncryptedData),
	)
	if err != nil {
		return http.ErrorStatus(STATUS_INTERNAL_ERROR, err)
	}

	this.logger.Info("decode credentials", map[string]interface{}{
		"appid":          user.Appid,
		"openid":         user.Openid,
		"sessionkey":     user.Sessionkey,
		"iv":             in.IV,
		"encrypted_data": in.EncryptedData,
		"result":         data,
	})
	userinfo := &UserInfo{}
	if err := http.BindAndValidJson(userinfo, []byte(data)); err != nil {
		return http.ErrorStatus(STATUS_INTERNAL_ERROR, err)
	}
	if user.Openid != userinfo.Openid {
		return http.JsonStatus(
			STATUS_INTERNAL_ERROR,
			fmt.Sprintf("openid is not same %s %s", user.Openid, userinfo.Openid),
		)
	}

	if _, err := this.conn.Execute(
		"UPDATE `user` SET unionid=? WHERE appid=? AND openid=? LIMIT 1",
		userinfo.UnionId,
		user.Appid,
		user.Openid,
	); err != nil {
		return http.ErrorStatus(STATUS_INTERNAL_ERROR, err)
	}

	if _, err := this.conn.Execute(
		"INSERT INTO user_extra (user_id, nickname, avatar_url) VALUES (?,?,?) "+
			"ON DUPLICATE KEY UPDATE nickname=?,avatar_url=?",
		user.Id,
		userinfo.NickName,
		userinfo.AvatarUrl,
		userinfo.NickName,
		userinfo.AvatarUrl,
	); err != nil {
		return http.ErrorStatus(STATUS_INTERNAL_ERROR, err)
	}

	// 验签
	return nil
}
