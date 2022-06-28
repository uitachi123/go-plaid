package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/uitachi123/go-plaid/pkg/db"
)

func Users(w http.ResponseWriter, r *http.Request) {
	users, err := list()
	if err != nil {
		io.WriteString(w, err.Error())
	}
	b, err := json.Marshal(users)
	if err != nil {
		io.WriteString(w, err.Error())
	}
	io.WriteString(w, string(b))
}

func list() ([]db.User, error) {
	d, err := db.Init()
	if err != nil {
		return []db.User{}, err
	}
	txn := d.Txn(false)
	defer txn.Abort()
	iter, err := txn.Get("user", "id")
	if err != nil {
		return []db.User{}, err
	}
	var res []db.User
	for {
		elem := iter.Next()
		if elem == nil {
			break
		}
		res = append(res, *elem.(*db.User))
	}
	return res, nil
}
