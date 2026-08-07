package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"urockclimbing.com/backend/internal/config"
	"urockclimbing.com/backend/internal/platform/database"
)

func main() {
	if len(os.Args) != 2 || os.Args[1] != "create" {
		fmt.Fprintln(os.Stderr, "usage: admin create")
		os.Exit(2)
	}
	username := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_USERNAME")))
	password := os.Getenv("ADMIN_PASSWORD")
	if username == "" || len(password) < 12 {
		fmt.Fprintln(os.Stderr, "ADMIN_USERNAME and ADMIN_PASSWORD (at least 12 characters) are required")
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		fatal(err)
	}
	db, err := database.OpenMySQL(cfg.MySQLDSN)
	if err != nil {
		fatal(err)
	}
	defer db.Close()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		fatal(err)
	}
	memberNo, err := randomMemberNo()
	if err != nil {
		fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		fatal(err)
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO users (member_no, nickname, role, status) VALUES (?, ?, 'ADMIN', 'ACTIVE')`, memberNo, username)
	if err == nil {
		var userID int64
		userID, err = result.LastInsertId()
		if err == nil {
			_, err = tx.ExecContext(ctx, `INSERT INTO admin_credentials (user_id, username, password_hash, must_change_password) VALUES (?, ?, ?, TRUE)`, userID, username, string(hash))
		}
	}
	if err != nil {
		_ = tx.Rollback()
		fatal(err)
	}
	if err := tx.Commit(); err != nil {
		fatal(err)
	}
	fmt.Printf("admin %s created\n", username)
}

func randomMemberNo() (string, error) {
	value := make([]byte, 6)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return "ADM" + strings.ToUpper(hex.EncodeToString(value)), nil
}

func fatal(err error) {
	if err == nil {
		err = errors.New("unknown error")
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
