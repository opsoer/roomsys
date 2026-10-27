package testkit

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Client 带固定测试 IP 的 HTTP 客户端；X-Forwarded-For 用于隔离限流/电话额度。
type Client struct {
	T       *testing.T
	BaseURL string
	IP      string
}

// Response 统一 API 响应 + 原始信息。
type Response struct {
	Status  int
	Code    int
	Message string
	Data    json.RawMessage
	Raw     []byte
	Header  http.Header
}

func NewClient(t *testing.T, baseURL, ip string) *Client {
	return &Client{T: t, BaseURL: strings.TrimRight(baseURL, "/"), IP: ip}
}

func (c *Client) cloneWithIP(ip string) *Client {
	cp := *c
	cp.IP = ip
	return &cp
}

// Do 发 JSON 请求；token 为空表示匿名。
func (c *Client) Do(method, path, token string, body any) *Response {
	c.T.Helper()
	return c.do(method, path, token, body, nil)
}

// DoWithHeaders 发 JSON 请求并附加自定义请求头。
func (c *Client) DoWithHeaders(method, path, token string, body any, headers map[string]string) *Response {
	c.T.Helper()
	return c.do(method, path, token, body, headers)
}

func (c *Client) do(method, path, token string, body any, headers map[string]string) *Response {
	c.T.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(c.T, err)
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, reader)
	require.NoError(c.T, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if c.IP != "" {
		req.Header.Set("X-Forwarded-For", c.IP)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(c.T, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(c.T, err)

	r := &Response{Status: resp.StatusCode, Header: resp.Header, Raw: raw}
	if len(raw) > 0 && strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		var env struct {
			Code    *int            `json:"code"`
			Message string          `json:"message"`
			Data    json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(raw, &env); err == nil && env.Code != nil {
			r.Code, r.Message, r.Data = *env.Code, env.Message, env.Data
		} else {
			r.Code, r.Data = 0, raw
		}
	}
	return r
}

// Upload 发 multipart 上传。
func (c *Client) Upload(path, token, field, filename string, content []byte, extra map[string]string) *Response {
	c.T.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile(field, filename)
	require.NoError(c.T, err)
	_, err = part.Write(content)
	require.NoError(c.T, err)
	for k, v := range extra {
		require.NoError(c.T, w.WriteField(k, v))
	}
	require.NoError(c.T, w.Close())

	req, err := http.NewRequest(http.MethodPost, c.BaseURL+path, &buf)
	require.NoError(c.T, err)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if c.IP != "" {
		req.Header.Set("X-Forwarded-For", c.IP)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(c.T, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(c.T, err)
	r := &Response{Status: resp.StatusCode, Header: resp.Header, Raw: raw}
	var env struct {
		Code    *int            `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err == nil && env.Code != nil {
		r.Code, r.Message, r.Data = *env.Code, env.Message, env.Data
	} else {
		r.Code, r.Data = 0, raw
	}
	return r
}

// Decode 解码 data。
func (r *Response) Decode(t *testing.T, out any) {
	t.Helper()
	require.NoError(t, json.Unmarshal(r.Data, out), "decode data failed, raw=%s", string(r.Raw))
}

// RequireOK 断言 HTTP 200/201 且 code=0。
func (r *Response) RequireOK(t *testing.T) *Response {
	t.Helper()
	require.Contains(t, []int{http.StatusOK, http.StatusCreated}, r.Status, "unexpected status, raw=%s", string(r.Raw))
	require.Equal(t, 0, r.Code, "unexpected code, raw=%s", string(r.Raw))
	return r
}

// RequireStatus 断言 HTTP 状态与业务 code。
func (r *Response) RequireStatus(t *testing.T, status, code int) *Response {
	t.Helper()
	require.Equal(t, status, r.Status, "unexpected status, raw=%s", string(r.Raw))
	require.Equal(t, code, r.Code, "unexpected code, raw=%s", string(r.Raw))
	return r
}

// RequireError 断言 HTTP 状态非 2xx 且 code 匹配。
func (r *Response) RequireError(t *testing.T, status, code int) *Response {
	t.Helper()
	return r.RequireStatus(t, status, code)
}

// RequireMessageContains 断言 message 包含关键字。
func (r *Response) RequireMessageContains(t *testing.T, sub string) *Response {
	t.Helper()
	require.Contains(t, r.Message, sub, "unexpected message, raw=%s", string(r.Raw))
	return r
}

// DataMap 把 data 解成 map，便于快速断言。
func (r *Response) DataMap(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	r.Decode(t, &m)
	return m
}

// URLValues 把 map 转 query。
func URLValues(kv map[string]string) string {
	v := url.Values{}
	for k, val := range kv {
		v.Set(k, val)
	}
	return v.Encode()
}
