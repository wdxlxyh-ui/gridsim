package main

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	existingHash := "$2a$10$N9qo8uLOkgxGqrUH0qgZ0uPJLV1QqvGJHlLZv.sL0dCkq0Ym9qW0y"
	err := bcrypt.CompareHashAndPassword([]byte(existingHash), []byte("admin"))
	fmt.Printf("Existing hash matches 'admin': %v (err=%v)\n", err == nil, err)

	err = bcrypt.CompareHashAndPassword([]byte(existingHash), []byte("123456"))
	fmt.Printf("Existing hash matches '123456': %v (err=%v)\n", err == nil, err)

	err = bcrypt.CompareHashAndPassword([]byte(existingHash), []byte("password"))
	fmt.Printf("Existing hash matches 'password': %v (err=%v)\n", err == nil, err)

	correctHash, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	fmt.Printf("\nCorrect hash for 'admin': %s\n", string(correctHash))

	err = bcrypt.CompareHashAndPassword(correctHash, []byte("admin"))
	fmt.Printf("New hash matches 'admin': %v\n", err == nil)
}
