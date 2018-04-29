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
	Hashes       map[uint64]File
	LastModified time.Time
}

type File struct {
	Delete  bool        `json:"de,omitempty"`
	Path    string      `json:"p,omitempty"`
	Size    int64       `json:"s,omitempty"`
	Mode    os.FileMode `json:"m,omitempty"`
	ModTime *time.Time  `json:"t,omitempty"`
	IsDir   bool        `json:"d,omitempty"`
	Hash    uint64      `json:"h,omitempty",hash:"ignore"`
	Content []byte      `json:"c,omitempty"`
}
