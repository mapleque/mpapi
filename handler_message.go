package mpapi

import (
	"bytes"
	"encoding/binary"
	"encoding/json"

	"github.com/mapleque/kelp/http"
	"github.com/mapleque/kelp/str"
)

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
	if _, err := http.RequestKelp(app.MessageApi+"?partner="+app.Appid, app.MessageApiToken, param, nil, c); err != nil {
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
