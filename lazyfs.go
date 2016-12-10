package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path"
	"strings"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	pb "github.com/jakrach/lazyfs/protobuf"
)

// Patterns for matching CRIU image files.
const (
	FDINFO_PATTERN  string = "fdinfo"
	REGFILE_PATTERN string = "reg-files.img"
)

// LazyFs represents a filesystem that migrates files lazily on read/write.
type LazyFs struct {
	pathfs.FileSystem
	FileMap    map[string]*pb.RegFileEntry
	RemoteInfo string
}

// GetAttr returns file attributes for any regular file opened by the
// checkpointed process.
func (me *LazyFs) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	f := me.FileMap[name]
	if f != nil {
		return &fuse.Attr{
			Mode: f.GetMode(), Size: f.GetSize(),
		}, fuse.OK
	} else if name == "" {
		return &fuse.Attr{
			Mode: fuse.S_IFDIR | 0755,
		}, fuse.OK
	} else {
		return nil, fuse.ENOENT
	}
}

// OpenDir builds a directory listing from each checkpointed open file.
func (me *LazyFs) OpenDir(name string, context *fuse.Context) (c []fuse.DirEntry, code fuse.Status) {
	if name == "" {
		c = []fuse.DirEntry{}
		for name, e := range me.FileMap {
			c = append(c, fuse.DirEntry{Name: name, Mode: e.GetMode()})
		}
		return c, fuse.OK
	}
	return nil, fuse.ENOENT
}

// Open returns the lazy file which will fetch on reads/writes.
func (me *LazyFs) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	f := me.FileMap[name]
	if f != nil {
		return NewLazyFile(f, me.RemoteInfo), fuse.OK
	} else {
		return nil, fuse.ENOENT
	}
}

// main parses user input and CRIU image files, registers the remote host, and
// mounts the filesystem.
func main() {
	flag.Parse()
	if len(flag.Args()) != 3 {
		log.Fatal("Usage:\n  lazyfs MOUNTPOINT IMGDIR USER@RHOST")
	}

	// Grab all the files in the image directory
	files, err := ioutil.ReadDir(flag.Arg(1))
	if err != nil {
		log.Fatalf("ReadDir fail: %v=\n", err)
	}

	// Iterate over image files, grab fdinfo and parse into a map
	fdMap := map[uint32]bool{}
	for _, f := range files {
		// If we found a fdinfo-XX.img, parse that file
		if (len(f.Name()) > len(FDINFO_PATTERN)) &&
			(f.Name()[:len(FDINFO_PATTERN)] == FDINFO_PATTERN) {
			fdEntries, err := FdinfoImg{path.Join(flag.Arg(1),
				f.Name())}.ReadEntries()
			if err != nil {
				log.Fatalf("Image read fail: %v\n", err)
			}
			for _, e := range fdEntries {
				// Only keep the regular open file descriptors
				if *e.Type == pb.FdTypes_REG {
					fdMap[*e.Id] = true
				}
			}
		}
	}

	// Grab the regular files and parse into a list
	remoteFiles := map[string]*pb.RegFileEntry{}
	entries, err := RegFileImg{path.Join(flag.Arg(1),
		REGFILE_PATTERN)}.ReadEntries()
	if err != nil {
		log.Fatalf("Image read fail: %v\n", err)
	}
	for _, e := range entries {
		// Only store entries that are in our fd map
		if fdMap[*e.Id] {
			local := strings.Replace((e.GetName()), "/", ".", -1)[1:]
			remoteFiles[local] = e
		}
	}

	// Log the retrieved files from the checkpointed process
	for name, e := range remoteFiles {
		log.Printf("Placeholder %s saved with: %v\n", name, e)
	}

	// Setup fuse mount
	nfs := pathfs.NewPathNodeFs(&LazyFs{
		FileSystem: pathfs.NewDefaultFileSystem(),
		FileMap:    remoteFiles,
		RemoteInfo: flag.Arg(2),
	}, nil)
	server, _, err := nodefs.MountRoot(flag.Arg(0), nfs.Root(), nil)
	if err != nil {
		log.Fatalf("Mount fail: %v\n", err)
	}

	// Catch SIGINT and exit cleanly.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for {
			sig := <-c
			log.Println("Recieved", sig.String(), "-- unmounting")
			err := server.Unmount()
			if err != nil {
				log.Println("Error while unmounting: %v", err)
			}
		}
	}()

	server.Serve()
}
