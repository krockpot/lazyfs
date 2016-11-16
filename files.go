package main

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/jakrach/lazyfs/protobuf"
)

// Regfile represents a checkpointed open file
type LazyFile struct {
	Fd        uint32
	LocalName string
	Cached    bool
	PB        *protobuf.RegFileEntry
	inner     nodefs.File
}

func GetFile(l []*LazyFile, f string) *LazyFile {
	for _, entry := range l {
		if entry.LocalName == f {
			return entry
		}
	}
	return &LazyFile{}
}

func (f *LazyFile) fetchRemote(user, rhost string) error {
	fname := f.PB.Name
	cmd := exec.Command("scp", user+"@"+rhost+":"+*fname, *fname)
	fmt.Println(cmd)
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func NewLazyFile(fd uint32, e *protobuf.RegFileEntry) *LazyFile {
	return &LazyFile{
		Fd:        fd,
		LocalName: "remote_open_file_" + fmt.Sprint(fd),
		Cached:    false,
		PB:        e,
		inner:     nil,
	}
}

func (f *LazyFile) String() string {
	str := "fd #%d placeholder at %s for remote file %s"
	if f.Cached {
		str = "fd #%d cached at %s for remote file %s"
	}
	return fmt.Sprintf(str, f.Fd, f.LocalName, *f.PB.Name)
}

// InnerFile returns the loopback file held by the lazy file.
// If not cached, this field will be nil.
func (f *LazyFile) InnerFile() nodefs.File {
	return f.inner
}

func (f *LazyFile) Read(dest []byte, off int64) (fuse.ReadResult, fuse.Status) {
	/*
		if !f.Cached {
			f.Cached = true
		}
		return f.inner.Read(dest, off)
	*/
	return nil, fuse.ENOSYS
}

func (f *LazyFile) Write(data []byte, off int64) (written uint32, code fuse.Status) {
	return 0, fuse.ENOSYS
}

func (f *LazyFile) Flush() fuse.Status {
	return fuse.ENOSYS
}

func (f *LazyFile) Release() {
}

func (f *LazyFile) Fsync(flags int) fuse.Status {
	return fuse.ENOSYS
}

func (f *LazyFile) Truncate(size uint64) fuse.Status {
	return fuse.ENOSYS
}

func (f *LazyFile) GetAttr(out *fuse.Attr) fuse.Status {
	return fuse.ENOSYS
}

func (f *LazyFile) Chown(uid uint32, gid uint32) fuse.Status {
	return fuse.ENOSYS
}

func (f *LazyFile) Chmod(perms uint32) fuse.Status {
	return fuse.ENOSYS
}

func (f *LazyFile) Utimens(atime *time.Time, mtime *time.Time) fuse.Status {
	return fuse.ENOSYS
}

func (f *LazyFile) Allocate(off uint64, size uint64, mode uint32) fuse.Status {
	return fuse.ENOSYS
}

func (f *LazyFile) SetInode(inode *nodefs.Inode) {
	// Do nothing
}
