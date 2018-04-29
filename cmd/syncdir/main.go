package main

import (
	"flag"
	"fmt"
	"path/filepath"

	"github.com/schollz/syncdir"
)

func main() {
	var debug bool
	var port, passcode string
	flag.BoolVar(&debug, "debug", false, "set debug mode")
	flag.StringVar(&port, "port", "8045", "port for running server")
	flag.StringVar(&passcode, "code", "123", "passcode for running server")
	flag.Parse()

	if debug {
		syncdir.SetLogLevel("debug")
	} else {
		syncdir.SetLogLevel("info")
	}

	fname, _ := filepath.Abs(".")
	fmt.Println("synchronizing", fname)

	// start a new sync dir on the current directory
	sd, err := syncdir.New(".", port, passcode)
	if err != nil {
		panic(err)
	}

	sd.Watch()
}
