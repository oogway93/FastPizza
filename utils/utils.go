package utils

import (
	"log"
	"net/mail"
)


func FailOnError(err error, msg string) {
	if err != nil {
		log.Println("Caused Error: ", err)
		panic(err)
	}
}

func ValidEmail(email string) error {
    _, err := mail.ParseAddress(email)
	return err
}
