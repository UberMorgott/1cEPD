// Команда hashpw печатает bcrypt-хеш пароля для переменной APP_PASSWORD_HASH.
//
//	go run ./cmd/hashpw "мой пароль"
package main

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "использование: hashpw <пароль>")
		os.Exit(2)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(os.Args[1]), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "не удалось посчитать хеш:", err)
		os.Exit(1)
	}
	fmt.Println(string(hash))
}
