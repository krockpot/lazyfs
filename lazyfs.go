package main

import (
	"flag"
	"fmt"
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
	Files []RegFile
	RHost string
	User  string
}

// GetAttr returns file attributes for any regular file opened by the
// checkpointed process.
func (me *LazyFs) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	f := GetFile(me.Files, name)
	if f != (RegFile{}) {
		return &fuse.Attr{
			Mode: *(*f.RemoteEntry).Mode, Size: *(*f.RemoteEntry).Size,
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
		for _, f := range me.Files {
			c = append(c, fuse.DirEntry{Name: f.LocalName, Mode: fuse.S_IFREG})
		}
		return c, fuse.OK
	}
	return nil, fuse.ENOENT
}

// Open returns the lazy file which will fetch on reads/writes.
func (me *LazyFs) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	f := GetFile(me.Files, name)
	if f != (RegFile{}) {
		return &f, fuse.OK
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

	arr := strings.Split(flag.Arg(2), "@")
	user, remoteHost := "", ""
	if len(arr) > 1 {
		user = arr[0]
		remoteHost = arr[1]
	} else {
		remoteHost = arr[0]
	}

	// Grab all the files in the image directory
	files, err := ioutil.ReadDir(flag.Arg(1))
	if err != nil {
		log.Fatalf("ReadDir fail: %v=\n", err)
	}

	// Iterate over image files, grab fdinfo and parse into a map
	fdMap := map[uint32]*pb.FdinfoEntry{}
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
					fdMap[*e.Id] = e
				}
			}
		}
	}

	// Grab the regular files and parse into a list
	remoteFiles := []RegFile{}
	entries, err := RegFileImg{path.Join(flag.Arg(1),
		REGFILE_PATTERN)}.ReadEntries()
	if err != nil {
		log.Fatalf("Image read fail: %v\n", err)
	}
	for _, e := range entries {
		// Only store entries that are in our fd map
		if fdMap[*e.Id] != nil {
			// TODO remove temporary testing for loopback
			f, err := os.Open(*e.Name)
			if err != nil {
				log.Fatalf("Open failed %v\n", err)
			}
			remoteFiles = append(remoteFiles, RegFile{
				Fd:        *fdMap[*e.Id].Fd,
				LocalName: "remote_open_file_" + fmt.Sprint(*fdMap[*e.Id].Fd),
				// TODO cached should be false and inner set to nil
				Cached:      true,
				Inner:       nodefs.NewLoopbackFile(f),
				RemoteEntry: e,
			})
		}
	}

	// TODO remove debugging entry printing
	for _, e := range remoteFiles {
		log.Println(e)
	}

	// Setup fuse mount
	nfs := pathfs.NewPathNodeFs(&LazyFs{
		FileSystem: pathfs.NewDefaultFileSystem(),
		Files:      remoteFiles,
		RHost:      remoteHost,
		User:       user,
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
