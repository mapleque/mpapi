package mpapi

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	stdhttp "net/http"
	"net/url"
	"strconv"

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

func (this *Server) UProxy(c *http.Context) *http.Status {
	userProxyTo := c.Request.Header.Get("User-Proxy-To")
	if userProxyTo == "" {
		return http.JsonStatus(STATUS_INTERNAL_ERROR, "invalid proxy header")
	}
	encodeurl, err := url.Parse(userProxyTo)
	if err != nil {
		return http.JsonStatus(STATUS_INTERNAL_ERROR, "invalid proxy header")
	}
	tr := &stdhttp.Transport{
		Proxy:             stdhttp.ProxyFromEnvironment,
		DisableKeepAlives: true,
	}
	c.Request.Header.Set("Authorization", this.authorization)
	c.Request.URL.Path = encodeurl.Path
	c.Request.URL.Host = encodeurl.Host
	c.Request.URL.Scheme = encodeurl.Scheme
	c.Request.Host = encodeurl.Host
	resp, err := tr.RoundTrip(c.Request)
	if err != nil {
		return http.ErrorStatus(STATUS_INTERNAL_ERROR, err)
	}
	for k, v := range resp.Header {
		for _, item := range v {
			c.ResponseWriter.Header().Add(k, item)
		}
	}
	c.ManuResponse = true
	c.HasResponse = true
	c.Response = []byte("proxy to " + userProxyTo)
	c.ResponseWriter.WriteHeader(resp.StatusCode)
	io.Copy(c.ResponseWriter, resp.Body)
	return nil
}

type wxcode struct {
	Scene     string            `json:"scene,omitempty"`
	Page      string            `json:"page,omitempty"`
	Width     int               `json:"width,omitempty"`
	AutoColor bool              `json:"auto_color,omitempty"`
	LineColor map[string]string `json:"line_color,omitempty"`
	IsHyaline bool              `json:"is_hyaline,omitempty"`
}

func (this *Server) WXCode(c *http.Context) *http.Status {
	app := c.MustGet("User-App").(*WXApp)
	req := &wxcode{
		Scene: c.QueryDefault("scene", ""),
		Page:  c.QueryDefault("page", ""),
	}
	if width := c.QueryDefault("width", ""); width != "" {
		if w, err := strconv.Atoi(width); err != nil {
			return http.StatusInvalidParam(fmt.Errorf("invalid param width"))
		} else {
			req.Width = w
		}
	}
	if c.QueryDefault("auto_color", "") == "true" {
		req.AutoColor = true
	}
	if lineColor := c.QueryDefault("line_color", ""); lineColor != "" {
		lc := map[string]string{}
		if err := json.Unmarshal([]byte(lineColor), &lc); err != nil {
			return http.StatusInvalidParam(fmt.Errorf("invalid param line_color"))
		} else {
			req.LineColor = lc
		}
	}
	if c.QueryDefault("is_hyaline", "") == "true" {
		req.IsHyaline = true
	}
	body, _ := json.Marshal(req)
	resp, err := app.WXCode(body)
	if err != nil {
		return http.ErrorStatus(STATUS_INTERNAL_ERROR, err)
	}
	c.ManuResponse = true
	c.ResponseWriter.Header().Set("Content-Type", "image/jpeg")
	c.ResponseWriter.Write(resp)
	return nil
}

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
	sessionkey := str.Base64Decode(user.Sessionkey)
	iv := str.Base64Decode(in.IV)
	encryptedData := str.Base64Decode(in.EncryptedData)
	data, err := str.AesCbcDecrypt(
		sessionkey,
		iv,
		encryptedData,
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

type Mobile struct {
	EncryptedData string `json:"encrypted_data" valid:"(0,),message=invalid param"`
	IV            string `json:"iv" valid:"(0,),message=invalid param"`
}

type MobileInfo struct {
	PhoneNumber     string `json:"phoneNumber" column:"phone_number"`
	PurePhoneNumber string `json:"purePhoneNumber" column:"pure_phone_number"`
	CountryCode     string `json:"countryCode" column:"country_code"`
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
	mobile := &MobileInfo{}
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
		return http.JsonStatus(STATUS_NOT_LOGIN, "must login")
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

type WxRecieveMessage struct {
	MsgId        int64
	ToUserName   string
	FromUserName string
	CreateTime   int64
	MsgType      string
	Event        string
	SessionFrom  string
	Encrypt      string

	// text
	Content string

	// image
	PicUrl  string
	MediaId string

	// miniprogrampage
	Title        string
	AppId        string
	PagePath     string
	ThumbUrl     string
	ThumbMediaId string
}

type QaliveParam struct {
	UserId         string `json:"user_id"`
	UserName       string `json:"user_name"`
	UserAvatar     string `json:"user_avatar"`
	MessageType    string `json:"message_type"`
	MessageEvent   string `json:"message_event"`
	MessageContent string `json:"message_content"`
}

type QaliveParamImage struct {
	ExternalUrl string `json:"external_url"`
	InternalUrl string `json:"internal_url"`
	MediaId     string `json:"media_id"`
}

type QaliveParamMiniprogram struct {
	Title        string `json:"title"`
	AppId        string `json:"appid"`
	Pagepath     string `json:"pagepath"`
	ThumbUrl     string `json:"thumb_url"`
	ThumbMediaId string `json:"thumb_media_id"`
}

func (this *Server) MessageRecieve(c *http.Context) {
	// signature := c.QueryDefault("signature", "")
	// timestamp := c.QueryDefault("timestamp", "")
	// nonce := c.QueryDefault("nonce", "")
	// openid := c.QueryDefault("openid", "")
	// encryptType := c.QueryDefault("encrypt_type", "")
	// msgSignature := c.QueryDefault("msg_signature", "")
	// TODO check sign

	app := c.MustGet("User-App").(*WXApp)

	// 这是在添加消息接口，验证合法性
	echostr := c.QueryDefault("echostr", "")
	if echostr != "" {
		c.Text(echostr)
		return
	}

	// 这里处理消息
	message := &WxRecieveMessage{}
	if err := c.BindAndValidJson(message); err != nil {
		c.Text(err.Error())
		return
	}

	if message.Encrypt != "" {
		// decrypt message
		iv := make([]byte, 16)
		key := str.Base64Decode(app.MessageKey + "=")
		data := str.Base64Decode(message.Encrypt)
		messageStr, err := str.AesCbcDecrypt(key, iv, data)
		if err != nil {
			this.logger.Error(err)
			c.Text(err.Error())
			return
		}
		messageBytes := []byte(messageStr)
		if len(messageBytes) < 20 {
			this.logger.Error("invalid message head", messageStr)
			c.Text("invalid message head")
			return
		}
		contentLength := bytesToInt(messageBytes[16:20])
		if len(messageBytes) < contentLength+20 {
			this.logger.Error("invalid message content", messageStr)
			c.Text("invalid message content")
			return
		}
		contentBytes := messageBytes[20 : contentLength+20]
		if err := http.BindAndValidJson(message, contentBytes); err != nil {
			this.logger.Error(err)
			c.Text(err.Error())
		}
	}

	user := &UserInfo{}
	this.conn.QueryOne(
		user,
		"SELECT * FROM user_extra WHERE user_id = (SELECT id FROM `user` WHERE appid=? AND openid=?)",
		app.Appid,
		message.FromUserName,
	)
	param := &QaliveParam{
		UserId:       message.FromUserName,
		UserName:     user.NickName,
		UserAvatar:   user.AvatarUrl,
		MessageType:  message.MsgType,
		MessageEvent: message.Event,
	}
	switch message.MsgType {
	case "text":
		param.MessageContent = message.Content
	case "image":
		image := &QaliveParamImage{
			ExternalUrl: message.PicUrl,
			InternalUrl: message.PicUrl,
			MediaId:     message.MediaId,
		}
		cont, _ := json.Marshal(image)
		param.MessageContent = string(cont)
	case "miniprogrampage":
		mini := &QaliveParamMiniprogram{
			Title:        message.Title,
			AppId:        message.AppId,
			Pagepath:     message.PagePath,
			ThumbUrl:     message.ThumbUrl,
			ThumbMediaId: message.ThumbMediaId,
		}
		cont, _ := json.Marshal(mini)
		param.MessageContent = string(cont)
	default:
		param.MessageContent = "未知消息类型"
	}
	if _, err := http.RequestKelp(this.messageApi+"?partner="+app.Appid, this.messageApiToken, param, nil, c); err != nil {
		this.logger.Error(err)
		c.Text(err.Error())
		return
	}

	c.Text("success")
	return
}

type WxReplyMessage struct {
	ToUser  string `json:"touser"`
	MsgType string `json:"msgtype"`
	Content string `json:"content,omitempty"`

	Text            *WxReplyMessageText            `json:"text"`
	Image           *WxReplyMessageImage           `json:"image,omitempty"`
	Link            *WxReplyMessageLink            `json:"link,omitempty"`
	Miniprogrampage *WxReplyMessageMiniprogrampage `json:"miniprogrampage,omitempty"`
}

type WxReplyMessageText struct {
	Content string `json:"content"`
}

type WxReplyMessageImage struct {
	MediaId string `json:"media_id"`
}

type WxReplyMessageLink struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Url         string `json:"url"`
	ThumbUrl    string `json:"ThumbUrl"`
}

type WxReplyMessageMiniprogrampage struct {
	Title        string `json:"title"`
	AppId        string `json:"appid,omitempty"`
	Pagepath     string `json:"pagepath"`
	ThumbUrl     string `json:"thumb_url,omitempty"`
	ThumbMediaId string `json:"thumb_media_id"`
}

type QaliveReplyMessageImage struct {
	ExternalUrl string `json:"external_url"`
	InternalUrl string `json:"internal_url"`
}

func (this *Server) MessageReply(in *WxReplyMessage, out interface{}, c *http.Context) *http.Status {
	app := c.MustGet("User-App").(*WXApp)
	switch in.MsgType {
	case "text":
		in.Text = &WxReplyMessageText{in.Content}
	case "image":
		imageContent := &QaliveReplyMessageImage{}
		if err := json.Unmarshal([]byte(in.Content), imageContent); err != nil {
			return http.ErrorStatus(STATUS_INTERNAL_ERROR, err)
		}
		mediaId, err := app.UploadTempMedia(imageContent.InternalUrl)
		if err != nil {
			return http.ErrorStatus(STATUS_INTERNAL_ERROR, err)
		}
		in.Image = &WxReplyMessageImage{mediaId}
	default:
		return http.JsonStatus(STATUS_INTERNAL_ERROR, "unsupport message type")
	}
	in.Content = ""
	body, _ := json.Marshal(in)
	if err := app.SendCustomerMessage(body); err != nil {
		return http.ErrorStatus(STATUS_INTERNAL_ERROR, err)
	}
	return nil
}

func bytesToInt(b []byte) int {
	bytesBuffer := bytes.NewBuffer(b)

	var newInt int32
	binary.Read(bytesBuffer, binary.BigEndian, &newInt)

	return int(newInt)
}
