package utils

import (
	"fmt"
	"os"
)

func CheckErr(err error, msg string) {
	if err != nil {
		fmt.Printf("%s: %v\n", msg, err)
		os.Exit(1)
	}
}
