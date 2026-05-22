package main

import (
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	hash, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}

	cfg := map[string]interface{}{
		"users": []map[string]interface{}{
			{
				"id":            "user-admin-001",
				"username":      "admin",
				"password_hash": string(hash),
				"role":          "admin",
				"created_at":    1715846400,
			},
		},
	}

	data, _ := json.MarshalIndent(cfg, "", "  ")
	fmt.Println(string(data))
}
