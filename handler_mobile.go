package mpapi

import (
	"github.com/mapleque/kelp/http"
	"github.com/mapleque/kelp/str"
)

type Mobile struct {
	EncryptedData string `json:"encrypted_data" valid:"(0,),message=invalid param"`
	IV            string `json:"iv" valid:"(0,),message=invalid param"`
}

func (this *Server) Mobile(in *Mobile, out interface{}, c *http.Context) *http.Status {
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
	mobile := &UserMobile{}
	if err := http.BindAndValidJson(mobile, []byte(data)); err != nil {
		return http.ErrorStatus(STATUS_INTERNAL_ERROR, err)
	}
	if _, err := this.conn.Execute(
		"INSERT INTO `user_mobile` (user_id,phone_number,pure_phone_number,country_code) VALUES (?,?,?,?) "+
			"ON DUPLICATE KEY UPDATE phone_number=?,pure_phone_number=?,country_code=?",
		user.Id,
		mobile.PhoneNumber,
		mobile.PurePhoneNumber,
		mobile.CountryCode,
		mobile.PhoneNumber,
		mobile.PurePhoneNumber,
		mobile.CountryCode,
	); err != nil {
		return http.ErrorStatus(STATUS_INTERNAL_ERROR, err)
	}
	return nil
}
