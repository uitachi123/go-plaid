package db

import (
	"reflect"
	"testing"
)

func Test_Init(t *testing.T) {
	db, err := Init()
	if err != nil {
		t.Errorf("Error creating database %v", err)
	}
	if db == nil {
		panic("null db pointer")
	}
	txn := db.Txn(false)
	defer txn.Abort()

	// test id indexing
	raw, err := txn.First("user", "id", "bob@test.com")
	if err != nil {
		t.Errorf("Error fetching user bob@test.com: %v", err)
	}
	if !reflect.DeepEqual(raw.(*User).Name, "Bob") {
		t.Errorf("Data mismatch, expected:  %s got: %s", "Bob", raw.(*User).Name)
	}

	// testing
	raw, err = txn.First("user", "name", "Alice")
	if err != nil {
		t.Errorf("Error fetching user bob@test.com: %v", err)
	}
	if !reflect.DeepEqual(raw.(*User).Email, "alice@test.com") {
		t.Errorf("Data mismatch, expected:  %s got: %s", "alice@test.com", raw.(*User).Email)
	}
}
