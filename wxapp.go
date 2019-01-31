package mpapi

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	stdhttp "net/http"
	"time"

	"github.com/mapleque/kelp/http"
	"github.com/mapleque/kelp/logger"
	"github.com/mapleque/kelp/mysql"
)

type WXApp struct {
	Id              int64  `json:"id"`
	Appid           string `json:"appid"`
	Secret          string `json:"secret"`
	Host            string `json:"host"`
	HostToken       string `json:"host_token"`
	MessageKey      string `json:"message_key"`
	MessageApi      string `json:"message_api"`
	MessageApiToken string `json:"message_api_token"`
	Additional      string `json:"additional"`

	hasRefreshToken bool

	conn mysql.Connector
	log  logger.Loggerer
}

func NewWXApp(appid string, conn mysql.Connector) (*WXApp, error) {
	wxapp := &WXApp{
		conn: conn,
		log:  logger.Get("http"),
	}
	if err := wxapp.conn.QueryOne(
		wxapp,
		"SELECT * FROM `app` WHERE appid = ?",
		appid,
	); err != nil {
		return nil, err
	}
	return wxapp, nil
}

type AuthInfo struct {
	Sessionkey string `json:"session_key" valid:"message=need sessionkey"`
	Openid     string `json:"openid" valid:"message=need openid"`
	Unionid    string `json:"unionid"`
}

func (this *WXApp) Jscode2session(code string) (*AuthInfo, error) {
	// https://developers.weixin.qq.com/miniprogram/dev/api/open-api/login/code2Session.html
	urlTpl := "https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code"
	url := fmt.Sprintf(urlTpl, this.Appid, this.Secret, code)

	ret, err := this.getHttp(url)
	if err != nil {
		return nil, err
	}
	auth := &AuthInfo{}
	if err := http.BindAndValidJson(auth, ret); err != nil {
		return nil, fmt.Errorf("%v, %s", err, string(ret))
	}
	return auth, nil
}

type WXResponse struct {
	Code int    `json:"errcode"`
	Msg  string `json:"errmsg"`
}

func (this *WXApp) WXCode(body []byte) ([]byte, error) {
	urlTpl := "https://api.weixin.qq.com/wxa/getwxacodeunlimit?access_token=%s"
	return this.postWX(urlTpl, body)
}

func (this *WXApp) SendTemplateMessage(body []byte) error {
	urlTpl := "https://api.weixin.qq.com/cgi-bin/message/wxopen/template/send?access_token=%s"
	ret, err := this.postWX(urlTpl, body)
	if err != nil {
		return err
	}
	wxresp := &WXResponse{}
	if err := http.BindAndValidJson(wxresp, ret); err == nil {
		if wxresp.Code != 0 {
			return fmt.Errorf("wx response is %s", string(ret))
		}
	}
	return nil
}

func (this *WXApp) SendCustomerMessage(body []byte) error {
	urlTpl := "https://api.weixin.qq.com/cgi-bin/message/custom/send?access_token=%s"
	ret, err := this.postWX(urlTpl, body)
	if err != nil {
		return err
	}
	wxresp := &WXResponse{}
	if err := http.BindAndValidJson(wxresp, ret); err == nil {
		if wxresp.Code != 0 {
			return fmt.Errorf("wx response is %s", string(ret))
		}
	} else {
		return fmt.Errorf("wx response is %s", string(ret))
	}
	return nil
}

func (this *WXApp) UploadTempMedia(url string) (string, error) {
	download, err := stdhttp.Get(url)
	if err != nil {
		return "", err
	}
	defer download.Body.Close()
	body, err := ioutil.ReadAll(download.Body)
	if err != nil {
		return "", err
	}

	extArr, err := mime.ExtensionsByType(download.Header.Get("Content-Type"))
	if err != nil {
		return "", fmt.Errorf("unknown download image type, %v", err)
	}
	if len(extArr) < 1 {
		return "", fmt.Errorf("unknown download image type")
	}

	return this.postForUpload(extArr[0], body)
}

func (this *WXApp) postForUpload(ext string, body []byte) (string, error) {
	urlTpl := "https://api.weixin.qq.com/cgi-bin/media/upload?access_token=%s&type=image"
	url, err := this.buildUrl(urlTpl)
	if err != nil {
		return "", err
	}
	dataHeader := bytes.NewBufferString("")
	dataHeaderWriter := multipart.NewWriter(dataHeader)
	_, err = dataHeaderWriter.CreateFormFile("media", "tmp_upload_image"+ext)
	if err != nil {
		return "", err
	}
	data := bytes.NewReader(body)
	boundary := dataHeaderWriter.Boundary()
	dataFooter := bytes.NewBufferString(fmt.Sprintf("\r\n--%s--\r\n", boundary))

	dataReq := io.MultiReader(dataHeader, data, dataFooter)

	request, err := stdhttp.NewRequest("POST", url, dataReq)
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	request.ContentLength = int64(dataHeader.Len()) + int64(data.Len()) + int64(dataFooter.Len())

	ret, err, recall := this.doPost(url, "byte body", request)
	if recall {
		return this.postForUpload(ext, body)
	}
	if err != nil {
		return "", err
	}
	wxresp := &WXResponse{}
	if err := http.BindAndValidJson(wxresp, ret); err == nil {
		if wxresp.Code != 0 {
			return "", fmt.Errorf("wx response is %s", string(ret))
		}
	} else {
		return "", fmt.Errorf("wx response is %s", string(ret))
	}
	mediaresp := &struct {
		MediaId string `json:"media_id"`
	}{}
	if err := http.BindAndValidJson(mediaresp, ret); err != nil {
		return "", fmt.Errorf("wx response is %s", string(ret))
	}
	return mediaresp.MediaId, nil
}

type AccessToken struct {
	AccessToken string    `json:"access_token"`
	NeedRefresh bool      `json:"need_refresh"`
	ExpiresIn   int       `json:"expires_in"`
	Code        int       `json:"errcode"`
	UpdateAt    time.Time `json:"update_at"`
}

func (this *AccessToken) isExpired() bool {
	return time.Now().After(this.UpdateAt.Add(time.Duration(this.ExpiresIn) * time.Second))
}

func (this *WXApp) resetRefreshFlag() {
	if _, err := this.conn.Execute(
		"UPDATE app_access_token SET refresh = FALSE WHERE appid = ?",
		this.Appid,
	); err != nil {
		this.log.Error(fmt.Sprintf("rollback err refresh %s access token ", this.Appid))
	}
}

func (this *WXApp) getNewAccessToken() (string, error) {
	defer this.resetRefreshFlag()
	urlTpl := "https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s"
	url := fmt.Sprintf(urlTpl, this.Appid, this.Secret)
	ret, err := this.getHttp(url)
	if err != nil {
		return "", err
	}
	res := &AccessToken{}
	if err := http.BindAndValidJson(res, ret); err != nil {
		return "", fmt.Errorf("%v, %s", err, string(ret))
	}
	if res.Code != 0 {
		return "", fmt.Errorf(string(ret))
	}
	if _, err := this.conn.Execute(
		"UPDATE app_access_token SET need_refresh = FALSE, access_token = ?, expires_in = ?, update_at = NOW() WHERE appid = ?",
		res.AccessToken,
		res.ExpiresIn,
		this.Appid,
	); err != nil {
		return "", err
	}
	return res.AccessToken, nil
}

func (this *WXApp) queryAccessToken() (string, error) {
	accessToken := AccessToken{}
	if err := this.conn.QueryOne(
		&accessToken,
		"SELECT * FROM app_access_token WHERE appid = ?",
		this.Appid,
	); err != nil {
		return "", err
	}
	return accessToken.AccessToken, nil
}

func (this *WXApp) getAccessToken() (string, error) {
	accessToken := AccessToken{}
	if err := this.conn.QueryOne(
		&accessToken,
		"SELECT * FROM app_access_token WHERE appid = ?",
		this.Appid,
	); err != nil {
		return "", err
	}
	if accessToken.NeedRefresh || accessToken.isExpired() || accessToken.AccessToken == "" {
		eff, err := this.conn.Execute(
			"UPDATE app_access_token SET refresh = TRUE WHERE appid = ? AND refresh = FALSE",
			this.Appid,
		)
		if err != nil {
			return "", err
		}
		if eff != 0 {
			return this.getNewAccessToken()
		} else {
			return this.queryAccessToken()
		}
	}
	return accessToken.AccessToken, nil
}

func (this *WXApp) postWX(urlTpl string, body []byte) ([]byte, error) {
	url, err := this.buildUrl(urlTpl)
	if err != nil {
		return nil, err
	}
	data := bytes.NewReader(body)
	request, err := stdhttp.NewRequest("POST", url, data)
	if err != nil {
		return []byte(""), err
	}
	request.Header.Set("Content-Type", "application/json")
	ret, err, recall := this.doPost(url, string(body), request)
	if recall {
		return this.postWX(urlTpl, body)
	}
	return ret, err
}

func (this *WXApp) getHttp(url string) ([]byte, error) {
	resp, err := stdhttp.Get(url)
	if err != nil {
		return []byte(""), err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []byte(""), err
	}
	this.log.Info("get http", url, string(body))
	return body, nil
}

func (this *WXApp) buildUrl(urlTpl string) (string, error) {
	accessToken, err := this.getAccessToken()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(urlTpl, accessToken), nil
}

func (this *WXApp) doPost(url, body string, request *stdhttp.Request) ([]byte, error, bool) {
	resp, err := stdhttp.DefaultClient.Do(request)
	if err != nil {
		return nil, err, false
	}
	defer resp.Body.Close()
	ret, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err, false
	}

	logBody := string(ret)
	if len(logBody) > 500 {
		this.log.Info("post wx", url, body, fmt.Sprintf("response is too large (with %d bytes, head is %s)", len(logBody), logBody[0:100]))
	} else {
		this.log.Info("post wx", url, body, logBody)
	}
	// 这里先判断返回的是json，如果不是json，那么就认为是正常的返回
	wxresp := &WXResponse{}
	if err := http.BindAndValidJson(wxresp, ret); err == nil {
		// 如果返回过期码，重新获取access token请求
		if wxresp.Code == 40001 {
			if !this.hasRefreshToken {
				if _, err := this.conn.Execute(
					"UPDATE app_access_token SET need_refresh = TRUE WHERE appid = ? AND refresh = FALSE",
					this.Appid,
				); err != nil {
					return nil, err, false
				}
				this.hasRefreshToken = true
				return nil, nil, true
			} else {
				this.hasRefreshToken = false
				return ret, nil, false
			}
		}
	}
	this.hasRefreshToken = false
	return ret, nil, false
}
