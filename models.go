package syncdir

import (
	"time"

	"github.com/schollz/listfiles"
)

type FileUpdate struct {
	Hashes       map[uint64]struct{}
	LastModified time.Time
}

type File struct {
	Info    listfiles.File
	Content []byte
}
