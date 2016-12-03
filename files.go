package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
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
	remote    string
	inner     *os.File
	lock      sync.Mutex
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
	fd, err := os.Open(fname)
	if err != nil {
		return err
	}
	fd.Chmod(os.FileMode(*f.PB.Mode))
	fd.Close()
	return nil
}

func NewLazyFile(fd uint32, e *protobuf.RegFileEntry, remote string) *LazyFile {
	return &LazyFile{
		Fd:        fd,
		LocalName: strings.Replace((*e.Name)[1:], "/", ".", -1),
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

func (f *LazyFile) Read(dest []byte, off int64) (fuse.ReadResult, fuse.Status) {
	if !f.cached {
		err := f.fetchRemote()
		if err != nil {
			return nil, fuse.ToStatus(err)
		}
		f.inner, err = os.OpenFile(f.PB.GetName(), int(f.PB.GetFlags()),
			os.FileMode(f.PB.GetMode()))
		if err != nil {
			return nil, fuse.ToStatus(err)
		}
		f.cached = true
	}
	f.lock.Lock()
	r := fuse.ReadResultFd(f.inner.Fd(), off, len(dest))
	f.lock.Unlock()
	return r, fuse.OK
}

func (f *LazyFile) Write(data []byte, off int64) (written uint32, code fuse.Status) {
	if !f.cached {
		err := f.fetchRemote()
		if err != nil {
			return 0, fuse.ToStatus(err)
		}
		f.inner, err = os.OpenFile(f.PB.GetName(), int(f.PB.GetFlags()),
			os.FileMode(f.PB.GetMode()))
		if err != nil {
			return 0, fuse.ToStatus(err)
		}
		f.cached = true
	}
	f.lock.Lock()
	n, err := f.inner.WriteAt(data, off)
	f.lock.Unlock()
	return uint32(n), fuse.ToStatus(err)
}

func (f *LazyFile) Flush() fuse.Status {
	if f.cached {
		f.lock.Lock()
		newFd, err := syscall.Dup(int(f.inner.Fd()))
		f.lock.Unlock()
		if err != nil {
			return fuse.ToStatus(err)
		}
		err = syscall.Close(newFd)
		return fuse.ToStatus(err)
	}
	return fuse.OK
}

func (f *LazyFile) Release() {
	if f.cached {
		f.lock.Lock()
		f.inner.Close()
		f.lock.Unlock()
	}
}

func (f *LazyFile) Fsync(flags int) fuse.Status {
	if f.cached {
		f.lock.Lock()
		r := fuse.ToStatus(syscall.Fsync(int(f.inner.Fd())))
		f.lock.Unlock()
		return r
	}
	return fuse.OK
}

func (f *LazyFile) Truncate(size uint64) fuse.Status {
	if f.cached {
		f.lock.Lock()
		r := fuse.ToStatus(syscall.Ftruncate(int(f.inner.Fd()), int64(size)))
		f.lock.Unlock()
		return r
	}
	return fuse.OK
}

func (f *LazyFile) GetAttr(out *fuse.Attr) fuse.Status {
	if f.cached {
		st := syscall.Stat_t{}
		f.lock.Lock()
		err := syscall.Fstat(int(f.inner.Fd()), &st)
		f.lock.Unlock()
		if err != nil {
			return fuse.ToStatus(err)
		}
		out.FromStat(&st)
	}
	return fuse.OK
}

func (f *LazyFile) Chown(uid uint32, gid uint32) fuse.Status {
	if f.cached {
		f.lock.Lock()
		r := fuse.ToStatus(f.inner.Chown(int(uid), int(gid)))
		f.lock.Unlock()
		return r
	}
	return fuse.OK
}

func (f *LazyFile) Chmod(perms uint32) fuse.Status {
	if f.cached {
		f.lock.Lock()
		r := fuse.ToStatus(f.inner.Chmod(os.FileMode(perms)))
		f.lock.Unlock()
		return r
	}
	return fuse.OK
}

func (f *LazyFile) Allocate(off uint64, size uint64, mode uint32) fuse.Status {
	if f.cached {
		f.lock.Lock()
		err := syscall.Fallocate(int(f.inner.Fd()), mode, int64(off),
			int64(size))
		f.lock.Unlock()
		if err != nil {
			return fuse.ToStatus(err)
		}
	}
	return fuse.OK
}

func (f *LazyFile) InnerFile() nodefs.File {
	// unimplemented
	return nil
}
func (f *LazyFile) Utimens(atime *time.Time, mtime *time.Time) fuse.Status {
	// unimplemented
	return fuse.OK
}
func (f *LazyFile) SetInode(inode *nodefs.Inode) {
	// Do nothing
}
