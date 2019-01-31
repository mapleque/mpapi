package mpapi

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/mapleque/kelp/http"
)

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
