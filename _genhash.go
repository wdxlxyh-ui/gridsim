package main

import (
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash"`
	Role         string `json:"role"`
	CreatedAt    int64  `json:"created_at"`
}

type UserConfig struct {
	Users []User `json:"users"`
}

func main() {
	hash, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}

	cfg := UserConfig{
		Users: []User{
			{
				ID:           "user-admin-001",
				Username:     "admin",
				PasswordHash: string(hash),
				Role:         "admin",
				CreatedAt:    1715846400,
			},
		},
	}

	data, _ := json.MarshalIndent(cfg, "", "  ")
	fmt.Println(string(data))
}
