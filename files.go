package main

import (
	"fmt"
	"os"
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
	PB        *protobuf.RegFileEntry
	cached    bool
	inner     nodefs.File
	remote    string
}

func GetFile(l []*LazyFile, f string) *LazyFile {
	for _, entry := range l {
		if entry.LocalName == f {
			return entry
		}
	}
	return &LazyFile{}
}

func (f *LazyFile) fetchRemote() error {
	fname := *f.PB.Name
	cmd := exec.Command("scp", f.remote+":"+fname, fname)
	fmt.Println(cmd)
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func NewLazyFile(fd uint32, e *protobuf.RegFileEntry, remote string) *LazyFile {
	return &LazyFile{
		Fd:        fd,
		LocalName: "remote_open_file_" + fmt.Sprint(fd),
		PB:        e,
		cached:    false,
		inner:     nil,
		remote:    remote,
	}
}

func (f *LazyFile) String() string {
	str := "fd #%d placeholder at %s for remote file %s"
	if f.cached {
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
	if !f.cached {
		err := f.fetchRemote()
		if err != nil {
			// TODO choose proper error code
			return nil, fuse.ENOENT
		}
		fp, err := os.Open(*f.PB.Name)
		if err != nil {
			// TODO choose proper error code
			return nil, fuse.ENOENT
		}
		f.inner = nodefs.NewLoopbackFile(fp)
		f.cached = true
	}
	return f.inner.Read(dest, off)
}

func (f *LazyFile) Write(data []byte, off int64) (written uint32, code fuse.Status) {
	if !f.cached {
		err := f.fetchRemote()
		if err != nil {
			// TODO choose proper error code
			return 0, fuse.EBADF
		}
		fp, err := os.Open(*f.PB.Name)
		if err != nil {
			// TODO choose proper error code
			return 0, fuse.EBADF
		}
		f.inner = nodefs.NewLoopbackFile(fp)
		f.cached = true
	}
	return f.inner.Write(data, off)
}

func (f *LazyFile) Flush() fuse.Status {
	if !f.cached {
		return fuse.OK
	}
	return f.inner.Flush()
}

func (f *LazyFile) Release() {
	return // Need this here for some reason
	if !f.cached {
		return
	}
	f.inner.Release()
}

func (f *LazyFile) Fsync(flags int) fuse.Status {
	if !f.cached {
		return fuse.OK
	}
	return f.inner.Fsync(flags)
}

func (f *LazyFile) Truncate(size uint64) fuse.Status {
	if !f.cached {
		return fuse.OK
	}
	return f.inner.Truncate(size)
}

func (f *LazyFile) GetAttr(out *fuse.Attr) fuse.Status {
	if !f.cached {
		return fuse.ENOENT
	}
	return f.inner.GetAttr(out)
}

func (f *LazyFile) Chown(uid uint32, gid uint32) fuse.Status {
	if !f.cached {
		return fuse.OK
	}
	return f.inner.Chown(uid, gid)
}

func (f *LazyFile) Chmod(perms uint32) fuse.Status {
	if !f.cached {
		return fuse.OK
	}
	return f.inner.Chmod(perms)
}

func (f *LazyFile) Utimens(atime *time.Time, mtime *time.Time) fuse.Status {
	if !f.cached {
		return fuse.OK
	}
	return f.inner.Utimens(atime, mtime)
}

func (f *LazyFile) Allocate(off uint64, size uint64, mode uint32) fuse.Status {
	if !f.cached {
		return fuse.OK
	}
	return f.inner.Allocate(off, size, mode)
}

func (f *LazyFile) SetInode(inode *nodefs.Inode) {
	// Do nothing
}
