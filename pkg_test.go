package main

import (
	"fmt"
	"go-tube/youtube"
	"testing"

	"github.com/joho/godotenv"
)

func TestXxx(t *testing.T) {
	godotenv.Load()
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(`Error:`, r)
		}
	}()
	title, url, err := youtube.YtMp3("https://www.youtube.com/watch?v=9Bj8DGw0brA")
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(title, url)
}
