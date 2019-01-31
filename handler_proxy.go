package mpapi

import (
	"fmt"
	"github.com/mapleque/kelp/http"
	"io"
	stdhttp "net/http"
	"net/url"
)

func (this *Server) UProxy(c *http.Context) *http.Status {
	app := c.MustGet("User-App").(*WXApp)
	if app.Host == "" {
		return http.JsonStatus(STATUS_INTERNAL_ERROR, "this app has no host setting")
	}
	userProxyTo := app.Host + c.Request.URL.Path[4:]
	encodeurl, err := url.Parse(userProxyTo)
	if err != nil {
		return http.JsonStatus(STATUS_INTERNAL_ERROR, fmt.Sprintf("invalid proxy path %s", userProxyTo))
	}
	tr := &stdhttp.Transport{
		Proxy:             stdhttp.ProxyFromEnvironment,
		DisableKeepAlives: true,
	}
	c.Request.Header.Set("Authorization", app.HostToken)
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
