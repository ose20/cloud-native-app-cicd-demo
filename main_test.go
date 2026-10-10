package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHelloWorld(t *testing.T) {
	reqBody := bytes.NewBufferString("dummy")
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://sample.com/", reqBody)
	res := httptest.NewRecorder()

	helloWorld(res, req)

	if res.Code != http.StatusOK {
		t.Errorf("want %d, but %d", http.StatusOK, res.Code)
	}

	t.Parallel()
}
