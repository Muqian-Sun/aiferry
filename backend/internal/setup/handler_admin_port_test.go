package setup

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestInstallRejectsAdminPortEqualToServerPort 管理站与用户站是两个监听器，端口相同会让其中一个起不来。
// 校验必须在执行安装（连数据库、写配置）之前完成。
func TestInstallRejectsAdminPortEqualToServerPort(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	gin.SetMode(gin.TestMode)

	payload := map[string]any{
		"database": map[string]any{"host": "127.0.0.1", "port": 5432, "user": "postgres", "password": "x", "dbname": "sub2api"},
		"redis":    map[string]any{"host": "127.0.0.1", "port": 6379},
		"admin":    map[string]any{"email": "admin@example.com", "password": "password123"},
		"server":   map[string]any{"port": 9000, "admin_port": 9000},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/setup/install", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	install(c)

	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "Admin port must differ from server port") {
		t.Fatalf("expected 400 admin port conflict, got %d: %s", w.Code, w.Body.String())
	}
}
