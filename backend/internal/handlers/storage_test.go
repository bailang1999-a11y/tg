package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestPublicUploadAllowsOnlyPublicCategories(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tmp := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	tenantID := uuid.New().String()
	publicPath := filepath.Join("storage", "uploads", tenantID, "avatars")
	if err := os.MkdirAll(publicPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(publicPath, "ok.txt"), []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}

	r := gin.New()
	s := &Server{}
	r.GET("/storage/uploads/*filepath", s.PublicUpload)

	req := httptest.NewRequest(http.MethodGet, "/storage/uploads/"+tenantID+"/avatars/ok.txt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || w.Body.String() != "ok" {
		t.Fatalf("expected public upload to be served, got status=%d body=%q", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/storage/uploads/"+tenantID+"/terminals/secret.session", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected sensitive upload category to be hidden, got status=%d", w.Code)
	}
}
