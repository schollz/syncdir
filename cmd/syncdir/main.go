package main

import (
	"github.com/schollz/syncdir"
)

func main() {
	sd, err := syncdir.New(".", "8045", "123")
	if err != nil {
		panic(err)
	}

	sd.Watch()
}
