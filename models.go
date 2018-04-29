package syncdir

import (
	"os"
	"time"
)

type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type FileUpdate struct {
	Hashes       map[uint64]struct{}
	LastModified time.Time
}

type File struct {
	Path    string
	Size    int64
	Mode    os.FileMode
	ModTime time.Time
	IsDir   bool
	Hash    uint64 `hash:"ignore"`
	Content []byte
}
