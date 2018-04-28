package syncdir

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/schollz/listfiles"
	"github.com/schollz/peerdiscovery"
)

type SyncDir struct {
	Directory string
	Passcode  string
	Port      string

	lastModified time.Time
	peers        []string
	pathToFile   map[string]listfiles.File
	hashToPath   map[uint64]string
	sync.RWMutex
}

// New returns the syncdir object
func New(dir, port, passcode string) (sd *SyncDir, err error) {
	dir = filepath.Clean(dir)

	sd = new(SyncDir)
	sd.Lock()
	sd.Directory = dir
	sd.Passcode = passcode
	sd.Port = port
	sd.Unlock()

	err = sd.getFiles()

	lastModified := time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC)
	sd.Lock()
	for f := range sd.pathToFile {
		log.Debug(f, sd.pathToFile[f].ModTime.UTC())
		if lastModified.Sub(sd.pathToFile[f].ModTime.UTC()) < 0 {
			lastModified = sd.pathToFile[f].ModTime.UTC()
		}
	}
	sd.lastModified = lastModified
	log.Debugf("last modified: %s", sd.lastModified)
	sd.Unlock()
	return
}

func (sd *SyncDir) watchForPeers() (err error) {
	sd.RLock()
	passcode := []byte(sd.Passcode)
	port := sd.Port
	sd.RUnlock()

	for {
		discoveries, errDiscover := peerdiscovery.Discover(peerdiscovery.Settings{
			Limit:     -1,
			TimeLimit: 1 * time.Second,
			Delay:     400 * time.Millisecond,
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
			errCheck := checkPeer(discovery.Address + ":" + port)
			if errCheck != nil {
				// log.Debugf("peer %s recieved error: %s", discovery.Address, errCheck.Error())
				continue
			}
			peers[i] = discovery.Address
			i++
		}
		peers = peers[:i]
		log.Debugf("have %d peers: %+v", len(peers), peers)
		sd.Lock()
		sd.peers = peers
		sd.Unlock()
	}
}

func (sd *SyncDir) Watch() (err error) {
	go sd.watchForPeers()
	go sd.listen()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	defer watcher.Close()
	sd.RLock()
	watcher.Add(sd.Directory)
	for p := range sd.pathToFile {
		if !sd.pathToFile[p].IsDir {
			continue
		}
		log.Debug("watching ", p)
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
			log.Debug("event:", event)
			// if event.Op&fsnotify.Write == fsnotify.Write {
			// 	log.Debug("modified file:", event.Name)
			// }
		case err := <-watcher.Errors:
			log.Debug("error:", err)
		default:
			if time.Since(lastChange) > 1*time.Second && !hasSynced {
				log.Debug("time to do stuff")
				// list current files
				errGetFiles := sd.getFiles()
				if errGetFiles != nil {
					log.Error(errGetFiles)
				}
				errUpdatePeers := sd.updatePeers()
				if errUpdatePeers != nil {
					log.Error(errUpdatePeers)
				}
				hasSynced = true
			}
		}
	}
	return
}

func (sd *SyncDir) getFiles() (err error) {
	sd.Lock()
	defer sd.Unlock()
	start := time.Now()

	files, err := listfiles.ListFiles(sd.Directory)
	if err != nil {
		return
	}
	sd.pathToFile = make(map[string]listfiles.File)
	sd.hashToPath = make(map[uint64]string)
	for _, f := range files {
		sd.pathToFile[f.Path] = f
		sd.hashToPath[f.Hash] = f.Path
	}
	log.Debugf("found %d files in %s", len(sd.pathToFile), time.Since(start))
	return
}

func (sd *SyncDir) listen() (err error) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(middleWareHandler(), gin.Recovery())
	r.HEAD("/", func(c *gin.Context) { // handler for the uptime robot
		c.String(http.StatusOK, "OK")
	})
	r.GET("/list", func(c *gin.Context) {
		response := FileUpdate{
			Hashes: make(map[uint64]struct{}),
		}
		sd.RLock()
		defer sd.RUnlock()
		for k := range sd.pathToFile {
			if sd.pathToFile[k].IsDir {
				continue
			}
			response.Hashes[sd.pathToFile[k].Hash] = struct{}{}
		}
		response.LastModified = sd.lastModified
		c.JSON(200, response)
	})
	r.POST("/update", func(c *gin.Context) {
		err := func(c *gin.Context) (err error) {
			var files []File
			err = c.ShouldBindJSON(&files)
			if err != nil {
				return
			}
			for _, f := range files {
				dir := filepath.Dir(f.Info.Path)
				if !exists(dir) {
					err = os.MkdirAll(dir, 0777)
					if err != nil {
						return
					}
				}
				err = ioutil.WriteFile(f.Info.Path, f.Content, f.Info.Mode)
				if err != nil {
					return
				}
				err = os.Chtimes(f.Info.Path, f.Info.ModTime, f.Info.ModTime)
				if err != nil {
					return
				}
				log.Debugf("created: %+v", f.Info)
			}
			return
		}(c)
		if err != nil {
			c.JSON(200, gin.H{"success": false, "message": err.Error()})
		} else {
			c.JSON(200, gin.H{"success": true, "message": "updated"})
		}
	})
	sd.RLock()
	address := getLocalIP() + ":" + sd.Port
	sd.RUnlock()
	log.Infof("running server on %s", address)
	err = r.Run(address)
	return
}

func (sd *SyncDir) updatePeers() (err error) {
	sd.RLock()
	peers := sd.peers
	port := sd.Port
	sd.RUnlock()

	if len(peers) == 0 {
		return
	}
	for _, peer := range peers {
		peerList, err := getPeerList(peer + ":" + port)
		if err != nil {
			log.Warn(err)
		}
		log.Debug(peer, peerList)
	}
	return
}

func getPeerList(server string) (peerList FileUpdate, err error) {
	req, err := http.NewRequest("GET", "http://"+server+"/list", nil)
	if err != nil {
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&peerList)
	return
}

func checkPeer(server string) (err error) {
	timeout := time.Duration(1 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}
	req, err := http.NewRequest("HEAD", "http://"+server+"/", nil)
	if err != nil {
		return
	}
	_, err = client.Do(req)
	return
}

func middleWareHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		t := time.Now()
		// // Add base headers
		// addCORS(c)
		// Run next function
		c.Next()
		// Log request
		log.Infof("%v %v %v %s", c.Request.RemoteAddr, c.Request.Method, c.Request.URL, time.Since(t))
	}
}
