package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerServesAPIAndSPA(t *testing.T) {
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("api-ok"))
	})
	handler, err := NewHandler(api)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	apiRecorder := httptest.NewRecorder()
	handler.ServeHTTP(apiRecorder, httptest.NewRequest(http.MethodGet, "/api/v1/system/health", nil))
	if apiRecorder.Body.String() != "api-ok" {
		t.Fatalf("api route was not delegated: %q", apiRecorder.Body.String())
	}

	rootRecorder := httptest.NewRecorder()
	handler.ServeHTTP(rootRecorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if rootRecorder.Code != http.StatusOK || !strings.Contains(rootRecorder.Body.String(), "<div id=\"root\">") {
		t.Fatalf("root did not serve index: status=%d body=%q", rootRecorder.Code, rootRecorder.Body.String())
	}

	spaRecorder := httptest.NewRecorder()
	handler.ServeHTTP(spaRecorder, httptest.NewRequest(http.MethodGet, "/dashboard", nil))
	if spaRecorder.Code != http.StatusOK || !strings.Contains(spaRecorder.Body.String(), "<div id=\"root\">") {
		t.Fatalf("spa route did not fallback to index: status=%d body=%q", spaRecorder.Code, spaRecorder.Body.String())
	}
}
