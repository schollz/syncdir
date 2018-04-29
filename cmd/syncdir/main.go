package main

import (
	"flag"

	"github.com/schollz/syncdir"
)

func main() {
	var debug bool
	var port, passcode string
	flag.BoolVar(&debug, "debug", false, "set debug mode")
	flag.StringVar(&port, "port", "8045", "port for running server")
	flag.StringVar(&passcode, "code", "123", "passcode for running server")
	flag.Parse()

	// start a new sync dir on the current directory
	sd, err := syncdir.New(".", port, passcode)
	if err != nil {
		panic(err)
	}
	if debug {
		syncdir.SetLogLevel("debug")
	} else {
		syncdir.SetLogLevel("info")
	}

	sd.Watch()
}
