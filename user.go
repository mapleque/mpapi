package mpapi

import (
	"encoding/json"
)

type User struct {
	Id         int64  `json:"id"`
	Appid      string `json:"appid"`
	Openid     string `json:"openid"`
	Unionid    string `json:"unionid"`
	Sessionkey string `json:"sessionkey"`
	Token      string `json:"token"`
}

type UserExtra struct {
	Openid    string `json:"openId" column:"openid"`
	UnionId   string `json:"unionId" column:"unionid"`
	NickName  string `json:"nickName" column:"nickname"`
	AvatarUrl string `json:"avatarUrl" column:"avatar_url"`
}

type UserMobile struct {
	PhoneNumber     string `json:"phoneNumber" column:"phone_number"`
	PurePhoneNumber string `json:"purePhoneNumber" column:"pure_phone_number"`
	CountryCode     string `json:"countryCode" column:"country_code"`
}

func (this *UserExtra) ToString() string {
	bt, _ := json.Marshal(this)
	return string(bt)
}

func (this *UserMobile) ToString() string {
	bt, _ := json.Marshal(this)
	return string(bt)
}
