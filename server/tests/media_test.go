package tests

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"rental-server/models"
	"rental-server/tests/testkit"
)

var (
	pngBytes = mustB64("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	gifBytes = mustB64("R0lGODlhAQABAIAAAAAAAP///ywAAAAAAQABAAACAUwAOw==")
	mp4Bytes = append([]byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'm', 'p', '4', '2', 0x00, 0x00, 0x00, 0x00, 'm', 'p', '4', '2', 'i', 's', 'o', 'm'}, make([]byte, 64)...)
)

func mustB64(s string) []byte {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func uploadMedia(t *testing.T, app *testkit.App, token string, roomID uint, filename string, content []byte, category string) *testkit.Response {
	t.Helper()
	extra := map[string]string{}
	if category != "" {
		extra["category"] = category
	}
	return app.Client.Upload(fmt.Sprintf("/api/building/rooms/%d/media", roomID), token, "file", filename, content, extra)
}

func uploadCover(t *testing.T, app *testkit.App, token string, content []byte) *testkit.Response {
	t.Helper()
	return app.Client.Upload("/api/building/cover", token, "file", "cover.png", content, nil)
}

func mediaResult(t *testing.T, r *testkit.Response) models.RoomMedia {
	t.Helper()
	var data struct {
		Media models.RoomMedia `json:"media"`
	}
	r.Decode(t, &data)
	return data.Media
}

func TestMedia_UploadImageAndCover(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	r := uploadMedia(t, app, f.TokenA, f.Rooms["A101"].ID, "test.png", pngBytes, "gallery")
	r.RequireOK(t)
	m := mediaResult(t, r)
	require.Equal(t, "image", m.Type)
	require.Equal(t, "gallery", m.Category)
	require.Equal(t, "ready", m.Status)
	require.NotEmpty(t, m.FilePath)
	_, err := os.Stat(filepath.Join(app.Cfg.Server.UploadDir, filepath.FromSlash(m.FilePath)))
	require.NoError(t, err)
	require.EqualValues(t, 1, countWhere(t, app.DB, &models.RoomMedia{}, "room_id = ? AND type = ?", f.Rooms["A101"].ID, "image"))

	cover := uploadCover(t, app, f.TokenA, pngBytes)
	cover.RequireOK(t)
	var cd struct {
		CoverImage string `json:"cover_image"`
	}
	cover.Decode(t, &cd)
	require.NotEmpty(t, cd.CoverImage)
	require.Equal(t, cd.CoverImage, getBuilding(t, app, f.A.ID).CoverImage)

	// 再传一次封面，替换旧值
	cover2 := uploadCover(t, app, f.TokenA, pngBytes)
	cover2.RequireOK(t)
	var cd2 struct {
		CoverImage string `json:"cover_image"`
	}
	cover2.Decode(t, &cd2)
	require.NotEqual(t, cd.CoverImage, cd2.CoverImage)
}

func TestMedia_UploadVideo(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	r := uploadMedia(t, app, f.TokenA, f.Rooms["A101"].ID, "test.mp4", mp4Bytes, "video")
	r.RequireOK(t)
	m := mediaResult(t, r)
	require.Equal(t, "video", m.Type)
	require.Equal(t, "ready", m.Status)
}

func TestMedia_UploadValidation(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	noFile := app.Client.Upload(fmt.Sprintf("/api/building/rooms/%d/media", f.Rooms["A101"].ID), f.TokenA, "other", "x.txt", []byte("x"), nil)
	noFile.RequireError(t, http.StatusBadRequest, 400)

	fake := uploadMedia(t, app, f.TokenA, f.Rooms["A101"].ID, "fake.png", []byte("not an image"), "gallery")
	fake.RequireError(t, http.StatusBadRequest, 400)

	// 合法 PNG 头 + 11MB，触发大小限制
	big := append(append([]byte{}, pngBytes...), make([]byte, 11*1024*1024)...)
	bigR := uploadMedia(t, app, f.TokenA, f.Rooms["A101"].ID, "big.png", big, "gallery")
	bigR.RequireError(t, http.StatusBadRequest, 400)

	cross := uploadMedia(t, app, f.TokenA, f.Rooms["B201"].ID, "x.png", pngBytes, "gallery")
	cross.RequireError(t, http.StatusNotFound, 404)
}

func TestMedia_Limits(t *testing.T) {
	t.Run("10 images", func(t *testing.T) {
		app, f := testkit.NewAppWithFixture(t)
		for i := 0; i < 10; i++ {
			r := uploadMedia(t, app, f.TokenA, f.Rooms["A107"].ID, fmt.Sprintf("i%d.png", i), pngBytes, "gallery")
			r.RequireOK(t)
		}
		eleven := uploadMedia(t, app, f.TokenA, f.Rooms["A107"].ID, "i11.png", pngBytes, "gallery")
		eleven.RequireError(t, http.StatusBadRequest, 400).
			RequireMessageContains(t, "10")
	})

	t.Run("2 videos", func(t *testing.T) {
		app, f := testkit.NewAppWithFixture(t)
		for i := 0; i < 2; i++ {
			r := uploadMedia(t, app, f.TokenA, f.Rooms["A106"].ID, fmt.Sprintf("v%d.mp4", i), mp4Bytes, "video")
			r.RequireOK(t)
		}
		third := uploadMedia(t, app, f.TokenA, f.Rooms["A106"].ID, "v3.mp4", mp4Bytes, "video")
		third.RequireError(t, http.StatusBadRequest, 400).
			RequireMessageContains(t, "2")
	})
}

func TestMedia_DeleteAndCopy(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	up := uploadMedia(t, app, f.TokenA, f.Rooms["A101"].ID, "copy.png", pngBytes, "gallery")
	up.RequireOK(t)
	m := mediaResult(t, up)

	// 复制到 A106
	copyR := app.Client.Do(http.MethodPost, fmt.Sprintf("/api/building/rooms/%d/media/copy", f.Rooms["A106"].ID), f.TokenA, map[string]any{
		"source_room_number": "A101",
	})
	copyR.RequireOK(t)
	require.EqualValues(t, 1, countWhere(t, app.DB, &models.RoomMedia{}, "room_id = ?", f.Rooms["A106"].ID))

	self := app.Client.Do(http.MethodPost, fmt.Sprintf("/api/building/rooms/%d/media/copy", f.Rooms["A101"].ID), f.TokenA, map[string]any{
		"source_room_number": "A101",
	})
	self.RequireError(t, http.StatusBadRequest, 400)

	emptySource := app.Client.Do(http.MethodPost, fmt.Sprintf("/api/building/rooms/%d/media/copy", f.Rooms["A106"].ID), f.TokenA, map[string]any{
		"source_room_number": "A107",
	})
	emptySource.RequireError(t, http.StatusBadRequest, 400)

	missingSource := app.Client.Do(http.MethodPost, fmt.Sprintf("/api/building/rooms/%d/media/copy", f.Rooms["A106"].ID), f.TokenA, map[string]any{
		"source_room_number": "NOT_EXIST",
	})
	missingSource.RequireError(t, http.StatusNotFound, 404)

	crossSource := app.Client.Do(http.MethodPost, fmt.Sprintf("/api/building/rooms/%d/media/copy", f.Rooms["A106"].ID), f.TokenA, map[string]any{
		"source_room_number": "B201",
	})
	crossSource.RequireError(t, http.StatusNotFound, 404)

	// 当前实现行为：删除 A101 记录后，A106 的引用记录仍在，但物理文件被删除。
	del := app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/building/rooms/%d/media/%d", f.Rooms["A101"].ID, m.ID), f.TokenA, nil)
	del.RequireOK(t)
	require.Zero(t, countWhere(t, app.DB, &models.RoomMedia{}, "id = ?", m.ID))
	require.EqualValues(t, 1, countWhere(t, app.DB, &models.RoomMedia{}, "file_path = ?", m.FilePath))
	_, err := os.Stat(filepath.Join(app.Cfg.Server.UploadDir, filepath.FromSlash(m.FilePath)))
	require.True(t, os.IsNotExist(err), "按当前实现断言：共享文件被删除")

	// 跨栋删除
	require.NoError(t, app.DB.Create(&models.RoomMedia{
		RoomID: f.Rooms["B201"].ID, Type: "image", Category: "gallery", FilePath: "buildings/b/x.png", Status: "ready",
	}).Error)
	var bm models.RoomMedia
	require.NoError(t, app.DB.Where("room_id = ?", f.Rooms["B201"].ID).First(&bm).Error)
	crossDel := app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/building/rooms/%d/media/%d", f.Rooms["B201"].ID, bm.ID), f.TokenA, nil)
	crossDel.RequireError(t, http.StatusForbidden, 403)
}

func TestMedia_UploadTokenAndConfirm(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	// 本地模式没有七牛 token
	token := app.Client.Do(http.MethodGet, fmt.Sprintf("/api/building/media/upload-token?room_id=%d&ext=.jpg", f.Rooms["A101"].ID), f.TokenA, nil)
	token.RequireError(t, http.StatusBadRequest, 400)

	t.Run("confirm success", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows 下路径前缀校验不匹配，跳过该子测试")
		}

		confirm := app.Client.Do(http.MethodPost, fmt.Sprintf("/api/building/rooms/%d/media/confirm", f.Rooms["A101"].ID), f.TokenA, map[string]any{
			"key": fmt.Sprintf("rooms/%d/images/x.png", f.Rooms["A101"].ID), "type": "image", "category": "gallery", "file_name": "x.png", "file_size": 10,
		})
		confirm.RequireOK(t)
		m := mediaResult(t, confirm)
		require.Equal(t, fmt.Sprintf("rooms/%d/images/x.png", f.Rooms["A101"].ID), m.FilePath)
	})

	badPath := app.Client.Do(http.MethodPost, fmt.Sprintf("/api/building/rooms/%d/media/confirm", f.Rooms["A101"].ID), f.TokenA, map[string]any{
		"key": "../../etc/passwd", "type": "image",
	})
	badPath.RequireError(t, http.StatusBadRequest, 400)

	wrongRoom := app.Client.Do(http.MethodPost, fmt.Sprintf("/api/building/rooms/%d/media/confirm", f.Rooms["A101"].ID), f.TokenA, map[string]any{
		"key": fmt.Sprintf("rooms/%d/images/x.png", f.Rooms["B201"].ID), "type": "image",
	})
	wrongRoom.RequireError(t, http.StatusBadRequest, 400)
}

func TestMedia_PfopCallback(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)
	app.Cfg.Storage.Qiniu.PfopCallbackSecret = "test-secret"

	noSecret := app.Client.Do(http.MethodPost, "/api/qiniu/pfop-callback", "", map[string]any{})
	noSecret.RequireError(t, http.StatusForbidden, 403)

	wrong := app.Client.DoWithHeaders(http.MethodPost, "/api/qiniu/pfop-callback", "", map[string]any{},
		map[string]string{"X-Pfop-Secret": "wrong"})
	wrong.RequireError(t, http.StatusForbidden, 403)

	badBody := app.Client.DoWithHeaders(http.MethodPost, "/api/qiniu/pfop-callback", "", nil,
		map[string]string{"X-Pfop-Secret": "test-secret"})
	badBody.RequireError(t, http.StatusBadRequest, 400)

	missing := app.Client.DoWithHeaders(http.MethodPost, "/api/qiniu/pfop-callback", "", map[string]any{"id": "x"},
		map[string]string{"X-Pfop-Secret": "test-secret"})
	missing.RequireError(t, http.StatusBadRequest, 400)

	media := models.RoomMedia{
		RoomID: f.Rooms["A101"].ID, Type: "video", Category: "video",
		FilePath: "old.mp4", Status: "processing", TranscodeID: "tid-1",
	}
	require.NoError(t, app.DB.Create(&media).Error)

	ok := app.Client.DoWithHeaders(http.MethodPost, "/api/qiniu/pfop-callback", "", map[string]any{
		"id": "tid-1", "code": 0, "inputKey": "old.mp4",
		"items": []any{map[string]any{"code": 0, "key": "new_720p.mp4"}},
	}, map[string]string{"X-Pfop-Secret": "test-secret"})
	ok.RequireOK(t)

	var got models.RoomMedia
	require.NoError(t, app.DB.First(&got, media.ID).Error)
	require.Equal(t, "ready", got.Status)
	require.Equal(t, "new_720p.mp4", got.FilePath)
	require.Empty(t, got.TranscodeID)

	notExist := app.Client.DoWithHeaders(http.MethodPost, "/api/qiniu/pfop-callback", "", map[string]any{
		"id": "not-exist", "code": 0, "inputKey": "old.mp4", "items": []any{map[string]any{"code": 0, "key": "new.mp4"}},
	}, map[string]string{"X-Pfop-Secret": "test-secret"})
	notExist.RequireOK(t)
}

func TestMedia_Serve(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	up := uploadMedia(t, app, f.TokenA, f.Rooms["A101"].ID, "serve.png", pngBytes, "gallery")
	up.RequireOK(t)
	m := mediaResult(t, up)

	serve := app.Client.Do(http.MethodGet, "/api/media/"+m.FilePath, "", nil)
	require.Equal(t, 200, serve.Status)
	require.NotEmpty(t, serve.Raw)

	missing := app.Client.Do(http.MethodGet, "/api/media/buildings/none/not-exist.png", "", nil)
	require.Contains(t, []int{400, 404}, missing.Status)

	traversal := app.Client.Do(http.MethodGet, "/api/media/%2e%2e%2f%2e%2e%2fserver%2fconfig.yaml", "", nil)
	require.Contains(t, []int{400, 404}, traversal.Status)
	require.NotContains(t, string(traversal.Raw), "jwt_secret")
}

func TestMedia_CheckTranscodeNoop(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	r := app.Client.Do(http.MethodPost, fmt.Sprintf("/api/building/rooms/%d/media/check-transcode", f.Rooms["A101"].ID), f.TokenA, nil)
	r.RequireOK(t)
}
