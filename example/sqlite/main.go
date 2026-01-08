package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT UNIQUE
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`INSERT INTO users (name, email) VALUES (?, ?)`, "Alice", "alice@example.com")
	if err != nil {
		log.Fatal(err)
	}

	var name, email string
	err = db.QueryRow(`SELECT name, email FROM users WHERE id = 1`).Scan(&name, &email)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("User: %s <%s>\n", name, email)
}
