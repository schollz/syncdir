package main

import (
	"fmt"

	"github.com/schollz/syncdir"
)

func main() {
	sd, err := syncdir.New("../../", "123")
	if err != nil {
		panic(err)
	}
	fmt.Println(sd)
	sd.Watch()
}
