package echo

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func Test_Echo(t *testing.T) {
	req := httptest.NewRequest("GET", "/echo/hello", nil)
	w := httptest.NewRecorder()

	expectedOutput := "hello!"
	h := http.HandlerFunc(Echo)

	h.ServeHTTP(w, req)
	body := w.Body.String()

	if body != expectedOutput {
		t.Errorf("Expected %v\tGot %v", expectedOutput, body)
	}
}
