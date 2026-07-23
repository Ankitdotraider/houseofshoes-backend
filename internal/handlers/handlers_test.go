package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Ankitdotraider/houseofshoes/internal/db"
	"github.com/gin-gonic/gin"
)

func TestUninitializedDBHandlerSafety(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db.DB = nil // Ensure nil DB does not panic handlers

	r := gin.New()
	r.GET("/products", GetProducts)

	req, _ := http.NewRequest("GET", "/products?category=casual&gender=men", nil)
	resp := httptest.NewRecorder()

	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 status code for nil db, got: %d", resp.Code)
	}
}
