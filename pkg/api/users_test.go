package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/uitachi123/go-plaid/pkg/db"
)

func Test_List(t *testing.T) {
	users, _ := list()
	if !reflect.DeepEqual(len(users), 2) {
		t.Errorf("Wrong user counts - expected 2, actual %d", len(users))
	}
	user := db.User{Email: "alice@test.com", Name: "Alice"}
	if !reflect.DeepEqual(user, users[0]) {
		t.Errorf("Data mismatch, expected:  %v got: %v", user, users[0])
	}
	user = db.User{Email: "bob@test.com", Name: "Bob"}
	if !reflect.DeepEqual(user, users[1]) {
		t.Errorf("Data mismatch, expected:  %v got: %v", user, users[1])
	}
}

func Test_Users(t *testing.T) {
	req := httptest.NewRequest("GET", "/users", nil)
	w := httptest.NewRecorder()

	expectedOutput, err := json.Marshal(
		[]db.User{
			db.User{Email: "alice@test.com", Name: "Alice"},
			db.User{Email: "bob@test.com", Name: "Bob"},
		},
	)
	if err != nil {
		t.Errorf("error marshal expected return json: %v", err)
	}
	expected := string(expectedOutput)
	h := http.HandlerFunc(Users)

	h.ServeHTTP(w, req)
	body := w.Body.String()

	if body != expected {
		t.Errorf("Expected %v\tGot %v", expected, body)
	}
}
