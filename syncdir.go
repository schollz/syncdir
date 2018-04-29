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

	updating     bool
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

	discoveries, errDiscover := peerdiscovery.Discover(peerdiscovery.Settings{
		Limit:     -1,
		TimeLimit: 2 * time.Second,
		Delay:     1 * time.Millisecond,
		Payload:   []byte(sd.Passcode),
	})
	if errDiscover != nil {
		err = errors.Wrap(errDiscover, "problem discovering")
		return
	}
	for {
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

		discoveries, errDiscover = peerdiscovery.Discover(peerdiscovery.Settings{
			Limit:     -1,
			TimeLimit: 10 * time.Second,
			Delay:     1 * time.Millisecond,
			Payload:   []byte(sd.Passcode),
		})
		if errDiscover != nil {
			err = errors.Wrap(errDiscover, "problem discovering")
			return
		}

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

	toWatch := []string{}
	sd.RLock()
	currentDir := sd.Directory
	for p := range sd.pathToFile {
		if !sd.pathToFile[p].IsDir {
			continue
		}
		toWatch = append(toWatch, p)
	}
	sd.RUnlock()

	for _, d := range toWatch {
		watcher.Add(d)
	}

	lastChange := time.Now()
	hasSynced := false
	eventToggle := false
	for {
		select {
		case event := <-watcher.Events:
			eventToggle = true
			lastChange = time.Now()
			sd.Lock()
			if !sd.updating {
				hasSynced = false
				sd.lastModified = time.Now()
				log.Debug("event:", event)
			} else {
				log.Debug("ignoring event:", event)
			}
			sd.Unlock()
			// if event.Op&fsnotify.Write == fsnotify.Write {
			// 	log.Debug("modified file:", event.Name)
			// }
		case err := <-watcher.Errors:
			log.Debug("error:", err)
		default:
			if time.Since(lastChange) > 1*time.Second && eventToggle {

				eventToggle = false
				log.Debug("time to do stuff")
				// list current files
				errGetFiles := sd.getFiles()
				if errGetFiles != nil {
					log.Error(errGetFiles)
				}

				if !hasSynced {
					errUpdatePeers := sd.updatePeers()
					if errUpdatePeers != nil {
						log.Error(errUpdatePeers)
					}
					hasSynced = true
				}
				// update watchers
				for _, d := range toWatch {
					watcher.Remove(d)
				}
				toWatch = []string{currentDir}
				sd.RLock()
				for p := range sd.pathToFile {
					if !sd.pathToFile[p].IsDir {
						continue
					}
					toWatch = append(toWatch, p)
				}
				sd.RUnlock()
				for _, d := range toWatch {
					watcher.Add(d)
				}

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
			Hashes: make(map[uint64]File),
		}
		sd.RLock()
		defer sd.RUnlock()
		for k := range sd.pathToFile {
			response.Hashes[sd.pathToFile[k].Hash] = File{
				Path:  sd.pathToFile[k].Path,
				IsDir: sd.pathToFile[k].IsDir,
			}
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

			sd.Lock()
			sd.updating = true
			sd.Unlock()

			// create files
			for _, f := range files {
				if f.Delete {
					continue
				}
				log.Debugf("updating %s", f.Path)
				dir := filepath.Dir(f.Path)
				if !exists(dir) {
					os.MkdirAll(dir, 0777)
				}

				if !f.IsDir {
					err = ioutil.WriteFile(f.Path, f.Content, f.Mode)
					if err != nil {
						continue
					}
				} else {
					os.MkdirAll(f.Path, f.Mode)
				}

				err = os.Chmod(f.Path, f.Mode)
				if err != nil {
					continue
				}
				err = os.Chtimes(f.Path, *f.ModTime, *f.ModTime)
				if err != nil {
					continue
				}
				log.Infof("updated %s from %s", f.Path, c.Request.RemoteAddr)

			}

			// delete files
			for _, f := range files {
				if !f.Delete {
					continue
				}
				log.Debugf("deleting %s", f.Path)
				os.RemoveAll(f.Path)
			}

			go func() {
				time.Sleep(100 * time.Millisecond)
				sd.getFiles()
				sd.Lock()
				sd.updating = false
				sd.Unlock()
			}()

			err = nil
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
	lastModified := sd.lastModified
	hashToPath := sd.hashToPath
	pathToFile := sd.pathToFile
	sd.RUnlock()

	if len(peers) == 0 {
		return
	}
	for _, peer := range peers {
		peerList, err := getPeerList(peer + ":" + port)
		if err != nil {
			log.Warn(err)
		}
		if lastModified.Sub(peerList.LastModified) < 0 {
			log.Debugf("%s is ahead", peer)
			continue
		}
		log.Debugf("updating %s", peer)

		// figure out which ones need to be deleted
		filesToDelete := make(map[string]struct{})
		for h := range peerList.Hashes {
			// find which ones are in mine but aren't in peer
			if _, ok := pathToFile[peerList.Hashes[h].Path]; !ok {
				filesToDelete[peerList.Hashes[h].Path] = struct{}{}
			}
		}
		if len(filesToDelete) > 0 {
			log.Infof("found %d files to delete on peer %s", len(filesToDelete), peer)
		}
		filesToSend := make([]File, len(filesToDelete))
		i := 0
		for f := range filesToDelete {
			filesToSend[i] = File{
				Delete: true,
				Path:   f,
			}
			i++
		}

		// find which of mine need to be added to peer
		hashesToAdd := make(map[uint64]struct{})
		for h := range hashToPath {
			if _, ok := peerList.Hashes[h]; ok {
				continue
			}
			hashesToAdd[h] = struct{}{}
		}
		if len(hashesToAdd) > 0 {
			log.Infof("found %d files to add on peer %s", len(hashesToAdd), peer)
		}
		for h := range hashesToAdd {
			f, err := os.Stat(hashToPath[h])
			if err != nil {
				log.Warn(err)
				continue
			}
			modTime := f.ModTime()
			fileToSend := File{
				Path:    hashToPath[h],
				Size:    f.Size(),
				Mode:    f.Mode(),
				ModTime: &modTime,
				IsDir:   f.IsDir(),
				Hash:    h,
			}

			if !f.IsDir() {
				content, err := ioutil.ReadFile(hashToPath[h])
				if err != nil {
					log.Warn(err)
					continue
				}
				fileToSend.Content = content
			}

			filesToSend = append(filesToSend, fileToSend)
		}
		if len(filesToSend) > 0 {
			start := time.Now()
			err = sendFiles("POST", peer+":"+port, filesToSend)
			if err != nil {
				log.Warn("problem sending files", err)
			} else {
				log.Debugf("sent %d files in %s", len(filesToSend), time.Since(start))
			}
		}

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

func sendFiles(reqtype string, server string, fis []File) (err error) {
	payloadBytes, err := json.Marshal(fis)
	if err != nil {
		return
	}
	body := bytes.NewReader(payloadBytes)
	req, err := http.NewRequest(reqtype, "http://"+server+"/update", body)
	if err != nil {
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var target Response
	err = json.NewDecoder(resp.Body).Decode(&target)
	if err != nil {
		return
	}
	if !target.Success {
		err = errors.New(target.Message)
	}
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
		log.Debugf("%v %v %v %s", c.Request.RemoteAddr, c.Request.Method, c.Request.URL, time.Since(t))
	}
}
