package db

import (
	memdb "github.com/hashicorp/go-memdb"
)

type User struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

var d *memdb.MemDB

func Init() (*memdb.MemDB, error) {
	if d != nil {
		return d, nil
	}
	// Create the DB schema
	schema := &memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			"user": &memdb.TableSchema{
				Name: "user",
				Indexes: map[string]*memdb.IndexSchema{
					"id": &memdb.IndexSchema{
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.StringFieldIndex{Field: "Email"},
					},
					"name": &memdb.IndexSchema{
						Name:    "name",
						Unique:  false,
						Indexer: &memdb.StringFieldIndex{Field: "Name"},
					},
				},
			},
		},
	}

	d, err := memdb.NewMemDB(schema)
	if err != nil {
		return nil, err
	}

	// Insert data
	txn := d.Txn(true)
	users := []*User{
		&User{"bob@test.com", "Bob"},
		&User{"alice@test.com", "Alice"},
	}
	for _, u := range users {
		if err := txn.Insert("user", u); err != nil {
			return d, err
		}
	}
	txn.Commit()
	return d, nil
}
