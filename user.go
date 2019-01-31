package mpapi

type User struct {
	Id         int64  `json:"id"`
	Appid      string `json:"appid"`
	Openid     string `json:"openid"`
	Unionid    string `json:"unionid"`
	Sessionkey string `json:"sessionkey"`
	Token      string `json:"token"`
}
