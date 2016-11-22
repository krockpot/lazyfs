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

type Wrap struct {
	File   *os.File
	cached bool
	name   string
	flags  uint32
	lock   sync.Mutex
}

func NewWrapFile(nm string, flags uint32) nodefs.File {
	return &Wrap{
		File:   nil,
		cached: false,
		name:   nm,
		flags:  flags}
}

func (f *Wrap) String() string {
	return "wrap"
}

func (f *Wrap) InnerFile() nodefs.File {
	return nil
}
func (f *Wrap) SetInode(inode *nodefs.Inode) {
	// Do nothing
}

func (f *Wrap) Read(dest []byte, off int64) (fuse.ReadResult, fuse.Status) {
	var err error
	fmt.Println("reading")
	if !f.cached {
		f.File, err = os.OpenFile("/mirror/"+f.name, int(f.flags), 0)
		if err != nil {
			return nil, fuse.ToStatus(err)
		}
		f.cached = true
	}
	f.lock.Lock()
	r := fuse.ReadResultFd(f.File.Fd(), off, len(dest))
	f.lock.Unlock()
	return r, fuse.OK
}
func (f *Wrap) Write(data []byte, off int64) (written uint32, code fuse.Status) {
	var err error
	if !f.cached {
		f.File, err = os.OpenFile("/mirror/"+f.name, int(f.flags), 0)
		if err != nil {
			return 0, fuse.ToStatus(err)
		}
		f.cached = true
	}
	f.lock.Lock()
	n, err := f.File.WriteAt(data, off)
	f.lock.Unlock()
	return uint32(n), fuse.ToStatus(err)
}

func (f *Wrap) Flush() fuse.Status {
	if f.cached {
		f.lock.Lock()
		newFd, err := syscall.Dup(int(f.File.Fd()))
		f.lock.Unlock()
		if err != nil {
			return fuse.ToStatus(err)
		}
		err = syscall.Close(newFd)
		return fuse.ToStatus(err)
	}
	return fuse.OK
}

func (f *Wrap) Release() {
	if f.cached {
		f.lock.Lock()
		f.File.Close()
		f.lock.Unlock()
	}
}
func (f *Wrap) Fsync(flags int) fuse.Status {
	if f.cached {
		f.lock.Lock()
		r := fuse.ToStatus(syscall.Fsync(int(f.File.Fd())))
		f.lock.Unlock()
		return r
	}
	return fuse.OK
}
func (f *Wrap) Truncate(size uint64) fuse.Status {
	if f.cached {
		f.lock.Lock()
		r := fuse.ToStatus(syscall.Ftruncate(int(f.File.Fd()), int64(size)))
		f.lock.Unlock()
		return r
	}
	return fuse.OK
}
func (f *Wrap) GetAttr(out *fuse.Attr) fuse.Status {
	if f.cached {
		st := syscall.Stat_t{}
		f.lock.Lock()
		err := syscall.Fstat(int(f.File.Fd()), &st)
		f.lock.Unlock()
		if err != nil {
			return fuse.ToStatus(err)
		}
		out.FromStat(&st)
	}
	return fuse.OK
}
func (f *Wrap) Chown(uid uint32, gid uint32) fuse.Status {
	if f.cached {
		f.lock.Lock()
		r := fuse.ToStatus(f.File.Chown(int(uid), int(gid)))
		f.lock.Unlock()
		return r
	}
	return fuse.OK
}
func (f *Wrap) Chmod(perms uint32) fuse.Status {
	if f.cached {
		f.lock.Lock()
		r := fuse.ToStatus(f.File.Chmod(os.FileMode(perms)))
		f.lock.Unlock()
		return r
	}
	return fuse.OK
}
func (f *Wrap) Utimens(atime *time.Time, mtime *time.Time) fuse.Status {
	// unimplemented
	return fuse.OK
}
func (f *Wrap) Allocate(off uint64, size uint64, mode uint32) fuse.Status {
	if f.cached {
		f.lock.Lock()
		err := syscall.Fallocate(int(f.File.Fd()), mode, int64(off), int64(size))
		f.lock.Unlock()
		if err != nil {
			return fuse.ToStatus(err)
		}
	}
	return fuse.OK
}

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
		fp, err := os.OpenFile(f.PB.GetName(), int(f.PB.GetFlags()), os.FileMode(f.PB.GetMode()))
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
		fp, err := os.OpenFile(f.PB.GetName(), int(f.PB.GetFlags()), os.FileMode(f.PB.GetMode()))
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
