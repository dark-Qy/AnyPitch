package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestTitleFontHasScopedCORSAndMIME(t *testing.T) {
	t.Chdir(t.TempDir())
	dir := filepath.Join("web", "dist", "assets", "miniprogram", "fonts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		filepath.Join("web", "dist", "index.html"):         "<html>app</html>",
		filepath.Join(dir, "source-han-serif-medium.woff"): "wOFF-test-font",
		filepath.Join(dir, "OFL.txt"):                      "license",
	} {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	handler := newTestHandler(t)
	for _, path := range []string{"/assets/miniprogram/fonts/source-han-serif-medium.woff", "/assets/miniprogram/fonts/OFL.txt", "/api/healthz", "/"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Origin", "https://servicewechat.com")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s: status %d", path, response.Code)
		}
		if path == "/assets/miniprogram/fonts/source-han-serif-medium.woff" {
			if response.Header().Get("Content-Type") != "font/woff" || response.Header().Get("Access-Control-Allow-Origin") != "*" {
				t.Fatalf("font headers: %v", response.Header())
			}
			if response.Body.String() != "wOFF-test-font" {
				t.Fatal("font route returned SPA instead of font")
			}
		} else if response.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatalf("CORS escaped font scope: %s", path)
		}
	}
}
