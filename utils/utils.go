package utils

import "log"


func FailOnError(err error) {
	if err != nil {
		log.Println("Caused Error: ", err)
		panic(err)
	}
}

