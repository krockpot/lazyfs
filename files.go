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

// LazyFile represents a placeholder for a checkpointed file, potentially with
// a local copy to operate on.
type LazyFile struct {
	Fd        uint32
	LocalName string
	PB        *protobuf.RegFileEntry
	cached    bool
	remote    string
	inner     *os.File
	lock      sync.Mutex
}

// GetFile finds the file with name f in slice l.
func GetFile(l []*LazyFile, f string) *LazyFile {
	for _, entry := range l {
		if entry.LocalName == f {
			return entry
		}
	}
	return &LazyFile{}
}

// fetchRemote SCP's the source file from the saved remote host.
func (f *LazyFile) fetchRemote() error {
	fname := f.PB.GetName()
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
	fd.Chmod(os.FileMode(f.PB.GetMode()))
	fd.Close()
	return nil
}

// NewLazyFile creates a new lazy file structure.
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

// String prints the local and original filename, as well as if it is cached.
func (f *LazyFile) String() string {
	str := "PLACEHOLDER: (%s) -> %s"
	if f.cached {
		str = "CACHED: (%s) -> %s"
	}
	return fmt.Sprintf(str, f.LocalName, f.PB.GetName())
}

// Read fetches the remote file if it is not cached locally. Reads from the
// local copy of the file.
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

// Write fetches the remote file if it is not cached locally. Writes to the
// local copy of the file.
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

// Flush if cached, flushes the local file. Otherwise, always does nothing
// and returns OK.
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

// Release if cached, closes the local file. Otherwise, does nothing.
func (f *LazyFile) Release() {
	if f.cached {
		f.lock.Lock()
		f.inner.Close()
		f.lock.Unlock()
	}
}

// Fsync if cached, syncs the local file as normal. Otherwise, always does
// nothing and returns OK.
func (f *LazyFile) Fsync(flags int) fuse.Status {
	if f.cached {
		f.lock.Lock()
		r := fuse.ToStatus(syscall.Fsync(int(f.inner.Fd())))
		f.lock.Unlock()
		return r
	}
	return fuse.OK
}

// Truncate if cached, truncates the local file as normal. Otherwise, always
// does nothing and returns OK.
func (f *LazyFile) Truncate(size uint64) fuse.Status {
	if f.cached {
		f.lock.Lock()
		r := fuse.ToStatus(syscall.Ftruncate(int(f.inner.Fd()), int64(size)))
		f.lock.Unlock()
		return r
	}
	return fuse.OK
}

// GetAttr if cached, gets the attributes for the local file as normal.
// Otherwise, always does nothing and returns OK.
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

// Chown if cached, calls chown on the local file. Otherwise, always does
// nothing and returns OK.
func (f *LazyFile) Chown(uid uint32, gid uint32) fuse.Status {
	if f.cached {
		f.lock.Lock()
		r := fuse.ToStatus(f.inner.Chown(int(uid), int(gid)))
		f.lock.Unlock()
		return r
	}
	return fuse.OK
}

// Chmod if cached, calls chmod on the local file. Otherwise, always does
// nothing and returns OK.
func (f *LazyFile) Chmod(perms uint32) fuse.Status {
	if f.cached {
		f.lock.Lock()
		r := fuse.ToStatus(f.inner.Chmod(os.FileMode(perms)))
		f.lock.Unlock()
		return r
	}
	return fuse.OK
}

// Allocate if cached, calls allocate on the local file. Otherwise, always does
// nothing and returns OK.
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

// InnerFile lazy files do not implement this function.
func (f *LazyFile) InnerFile() nodefs.File { return nil }

// Utimens lazy files do not implement this function.
func (f *LazyFile) Utimens(atime *time.Time, mtime *time.Time) fuse.Status {
	return fuse.OK
}

// SetInode lazy files do not implement this function.
func (f *LazyFile) SetInode(inode *nodefs.Inode) {}
