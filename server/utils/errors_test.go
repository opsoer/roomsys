package utils

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newCtx 造一个测试用请求上下文（同时返回响应记录器以便断言响应体）。
func newCtx() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/test", nil)
	return c, w
}

// 业务错误的构造与错误链。
func TestBizError(t *testing.T) {
	be := BadRequest("参数 %s 不合法", "id")
	if be.HTTPStatus != http.StatusBadRequest || be.Code != CodeBadRequest {
		t.Fatalf("状态码/业务码不符: %+v", be)
	}
	if be.Message != "参数 id 不合法" {
		t.Fatalf("文案不符: %q", be.Message)
	}

	// 内部错误：对外文案与内部原因分离
	cause := errors.New("sql: no such table: bills")
	inner := Internal(cause, "服务器内部错误，请稍后重试")
	if !errors.Is(inner, cause) {
		t.Fatal("应能通过 errors.Is 找到内部原因")
	}
	if inner.Error() != "服务器内部错误，请稍后重试" {
		t.Fatalf("对外文案不应包含内部细节: %q", inner.Error())
	}
	if errors.Unwrap(inner) != cause {
		t.Fatal("Unwrap 应返回内部原因")
	}
}

// errors.As 能识别业务错误。
func TestBizErrorAs(t *testing.T) {
	var be *BizError
	if !errors.As(error(BadRequest("x")), &be) {
		t.Fatal("应能识别为 BizError")
	}
	if be.Code != CodeBadRequest {
		t.Fatalf("业务码不符: %d", be.Code)
	}
}

// Fail：业务错误按其状态码返回，未知错误统一 500 且不泄露细节。
func TestFail(t *testing.T) {
	c, _ := newCtx()
	Fail(c, BadRequest("房间号已存在"))
	if c.Writer.Status() != http.StatusBadRequest {
		t.Fatalf("状态码应为 400，实际 %d", c.Writer.Status())
	}

	c2, w2 := newCtx()
	Fail(c2, errors.New("dial tcp 127.0.0.1:3306: connection refused"))
	if c2.Writer.Status() != http.StatusInternalServerError {
		t.Fatalf("状态码应为 500，实际 %d", c2.Writer.Status())
	}
	body := w2.Body.String()
	if strings.Contains(body, "connection refused") || strings.Contains(body, "3306") {
		t.Fatalf("内部细节不应出现在响应中: %s", body)
	}
	if !strings.Contains(body, "服务器内部错误") {
		t.Fatalf("响应应包含通用文案: %s", body)
	}

	// nil 不应写响应
	c3, w3 := newCtx()
	Fail(c3, nil)
	if w3.Body.Len() != 0 {
		t.Fatal("nil 错误不应写响应")
	}
}

// 自定义业务码。
func TestWithCode(t *testing.T) {
	be := WithCode(Conflict("名称冲突"), CodeNameConflict)
	if be.Code != CodeNameConflict || be.HTTPStatus != http.StatusConflict {
		t.Fatalf("业务码/状态码不符: %+v", be)
	}
	// 非业务错误包装后仍是业务错误
	be2 := WithCode(errors.New("普通错误"), CodeInvalidStatus)
	if be2.Code != CodeInvalidStatus {
		t.Fatalf("业务码不符: %+v", be2)
	}
}

// 统一响应格式。
func TestResponseHelpers(t *testing.T) {
	c, w := newCtx()
	Success(c, map[string]int{"a": 1})
	if c.Writer.Status() != http.StatusOK {
		t.Fatalf("状态码不符: %d", c.Writer.Status())
	}
	if !strings.Contains(w.Body.String(), `"code":0`) {
		t.Fatalf("响应体不符: %s", w.Body.String())
	}

	c2, w2 := newCtx()
	Error(c2, http.StatusNotFound, "房间不存在")
	if c2.Writer.Status() != http.StatusNotFound {
		t.Fatalf("状态码不符: %d", c2.Writer.Status())
	}
	if !strings.Contains(w2.Body.String(), `"code":404`) {
		t.Fatalf("业务码映射不符: %s", w2.Body.String())
	}

	c3, w3 := newCtx()
	ErrorWithCode(c3, http.StatusConflict, CodeActiveContract, "存在生效合同")
	if !strings.Contains(w3.Body.String(), `"code":1003`) {
		t.Fatalf("自定义业务码不符: %s", w3.Body.String())
	}
}

// 分页参数解析。
func TestParsePage(t *testing.T) {
	cases := []struct {
		query    string
		wantPage int
		wantSize int
	}{
		{"", 1, 20},
		{"?page=3&page_size=50", 3, 50},
		{"?page=0&page_size=0", 1, 20},
		{"?page=abc&page_size=999", 1, 20},
		{"?page=2&page_size=100", 2, 100},
	}
	for _, tc := range cases {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/api/x"+tc.query, nil)
		page, size := ParsePage(c)
		if page != tc.wantPage || size != tc.wantSize {
			t.Errorf("query=%q: 期望 page=%d size=%d，实际 page=%d size=%d",
				tc.query, tc.wantPage, tc.wantSize, page, size)
		}
	}
}

// 上下文取用户与公寓 ID。
func TestContextGetters(t *testing.T) {
	c, _ := newCtx()
	if _, err := GetUserID(c); err == nil {
		t.Fatal("未登录时应返回错误")
	}
	if _, err := GetBuildingID(c); err == nil {
		t.Fatal("未关联公寓时应返回错误")
	}

	c.Set("user_id", uint(7))
	c.Set("building_id", uint(3))
	uid, err := GetUserID(c)
	if err != nil || uid != 7 {
		t.Fatalf("用户ID不符: %v %v", uid, err)
	}
	bid, err := GetBuildingID(c)
	if err != nil || bid != 3 {
		t.Fatalf("公寓ID不符: %v %v", bid, err)
	}
}

// 令牌生成、解析、吊销。
func TestTokenLifecycle(t *testing.T) {
	const secret = "test-secret-key"
	token, err := GenerateToken(7, "root", "super_admin", secret, 3)
	if err != nil {
		t.Fatalf("生成令牌失败: %v", err)
	}
	claims, err := ParseToken(token, secret)
	if err != nil {
		t.Fatalf("解析令牌失败: %v", err)
	}
	if claims.UserID != 7 || claims.Username != "root" || claims.Role != "super_admin" || claims.BuildingID != 3 {
		t.Fatalf("claims 不符: %+v", claims)
	}

	// 错误密钥解析失败
	if _, err := ParseToken(token, "wrong-secret"); err == nil {
		t.Fatal("错误密钥应解析失败")
	}
	// 非法令牌
	if _, err := ParseToken("not-a-token", secret); err == nil {
		t.Fatal("非法令牌应解析失败")
	}

	// 吊销后视为已吊销，且幂等
	RevokeToken(token)
	if !IsTokenRevoked(token) {
		t.Fatal("吊销后应返回 true")
	}
	RevokeToken(token)
	if !IsTokenRevoked(token) {
		t.Fatal("重复吊销应保持 true")
	}
	if IsTokenRevoked("other-token") {
		t.Fatal("未吊销的令牌应返回 false")
	}

	// 清理
	if before := CleanupRevokedTokens(); before == "" {
		t.Fatal("清理应返回结果描述")
	}
}

// 刷新令牌与访问令牌类型区分。
func TestRefreshTokenType(t *testing.T) {
	const secret = "test-secret-key"
	access, err := GenerateToken(1, "u", "admin", secret, 0)
	if err != nil {
		t.Fatalf("生成访问令牌失败: %v", err)
	}
	accessClaims, _ := ParseToken(access, secret)
	if IsRefreshTokenFromClaims(accessClaims) {
		t.Fatal("访问令牌不应被识别为刷新令牌")
	}

	refresh, err := GenerateRefreshToken(1, "u", "admin", secret, 0)
	if err != nil {
		t.Fatalf("生成刷新令牌失败: %v", err)
	}
	refreshClaims, _ := ParseToken(refresh, secret)
	if !IsRefreshTokenFromClaims(refreshClaims) {
		t.Fatal("刷新令牌应被识别为刷新令牌")
	}
}

// 缓存包装：命中、过期重取、错误透传。
func TestCacheGetOrSet(t *testing.T) {
	calls := 0
	fetch := func() (interface{}, error) {
		calls++
		return "value", nil
	}
	v, err := CacheGetOrSet("k1", time.Minute, fetch)
	if err != nil || v != "value" {
		t.Fatalf("取值失败: %v %v", v, err)
	}
	// 再次取应命中缓存，fetch 不再执行
	if _, err := CacheGetOrSet("k1", time.Minute, fetch); err != nil {
		t.Fatalf("二次取值失败: %v", err)
	}
	if calls != 1 {
		t.Fatalf("命中缓存时不应重复抓取，实际调用 %d 次", calls)
	}

	// 错误透传
	wantErr := errors.New("boom")
	if _, err := CacheGetOrSet("k2", time.Minute, func() (interface{}, error) {
		return nil, wantErr
	}); !errors.Is(err, wantErr) {
		t.Fatalf("应透传原始错误，实际 %v", err)
	}
}

// 验证码配额计数。
func TestRevealQuota(t *testing.T) {
	ip := "10.0.0.1"
	IncrReveal(ip)
	IncrReveal(ip)
	if got := RevealCount(ip); got != 2 {
		t.Fatalf("计数应为 2，实际 %d", got)
	}
	if got := RevealCount("10.0.0.2"); got != 0 {
		t.Fatalf("其他 IP 计数应为 0，实际 %d", got)
	}
}
