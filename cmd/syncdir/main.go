package main

import (
	"github.com/schollz/syncdir"
)

func main() {
	// go func() {
	// 	f, _ := os.Create("cpuprofile")
	// 	pprof.StartCPUProfile(f)
	// 	time.Sleep(10 * time.Second)
	// 	pprof.StopCPUProfile()
	// 	os.Exit(1)
	// }()

	sd, err := syncdir.New(".", "8045", "123")
	if err != nil {
		panic(err)
	}

	sd.Watch()
}
