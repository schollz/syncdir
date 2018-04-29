package main

import (
	"flag"

	"github.com/schollz/syncdir"
)

func main() {
	var port, passcode string
	flag.StringVar(&port, "port", "8045", "port for running server")
	flag.StringVar(&passcode, "code", "123", "passcode for running server")
	flag.Parse()
	// go func() {
	// 	f, _ := os.Create("cpuprofile")
	// 	pprof.StartCPUProfile(f)
	// 	time.Sleep(10 * time.Second)
	// 	pprof.StopCPUProfile()
	// 	os.Exit(1)
	// }()

	sd, err := syncdir.New(".", port, passcode)
	if err != nil {
		panic(err)
	}

	sd.Watch()
}
