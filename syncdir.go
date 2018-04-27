package syncdir

import (
	"bytes"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
	"github.com/schollz/listfiles"
	"github.com/schollz/peerdiscovery"
)

type SyncDir struct {
	Directory string
	Passcode  string

	peers      []string
	pathToFile map[string]listfiles.File
	hashToPath map[uint64]string
	sync.RWMutex
}

// New returns the syncdir
func New(dir, passcode string) (sd *SyncDir, err error) {
	dir = filepath.Clean(dir)

	sd = new(SyncDir)
	sd.Lock()
	defer sd.Unlock()

	sd.Directory = dir
	sd.Passcode = passcode
	files, err := listfiles.ListFilesRecursivelyInParallel(dir)
	if err != nil {
		return
	}
	sd.pathToFile = make(map[string]listfiles.File)
	sd.hashToPath = make(map[uint64]string)
	for _, f := range files {
		sd.pathToFile[f.Path] = f
		sd.hashToPath[f.Hash] = f.Path
	}
	return
}

func (sd *SyncDir) watchForPeers() (err error) {
	sd.RLock()
	passcode := []byte(sd.Passcode)
	sd.RUnlock()

	for {
		discoveries, errDiscover := peerdiscovery.Discover(peerdiscovery.Settings{
			Limit:     -1,
			TimeLimit: 10 * time.Second,
			Delay:     1 * time.Second,
			Payload:   []byte(sd.Passcode),
		})
		if errDiscover != nil {
			err = errors.Wrap(errDiscover, "problem discovering")
			return
		}
		peers := make([]string, len(discoveries))
		i := 0
		for _, discovery := range discoveries {
			if !bytes.Equal(discovery.Payload, passcode) {
				continue
			}
			peers[i] = discovery.Address
			i++
		}
		log.Println("have peers:", peers)
		sd.Lock()
		sd.peers = peers
		sd.Unlock()
	}
}

func (sd *SyncDir) Watch() (err error) {
	go sd.watchForPeers()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	defer watcher.Close()
	sd.RLock()
	for p := range sd.pathToFile {
		if !sd.pathToFile[p].IsDir {
			continue
		}
		log.Println("watching ", p)
		watcher.Add(p)
	}
	sd.RUnlock()

	lastChange := time.Now()
	hasSynced := false
	for {
		select {
		case event := <-watcher.Events:
			hasSynced = false
			lastChange = time.Now()
			log.Println("event:", event)
			// if event.Op&fsnotify.Write == fsnotify.Write {
			// 	log.Println("modified file:", event.Name)
			// }
		case err := <-watcher.Errors:
			log.Println("error:", err)
		default:
			if time.Since(lastChange) > 1*time.Second && !hasSynced {
				fmt.Println("time to do stuff")
				hasSynced = true
			}
		}
	}
	return
}
