// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// A Go mirror of libfuse's hello.c

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

const (
	FDINFO_PATTERN  string = "fdinfo"
	REGFILE_PATTERN string = "reg-files.img"
)

type LazyFs struct {
	pathfs.FileSystem
	Files []RegFile
	RHost string
	RPort string
}

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

func (me *LazyFs) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	if name != "file.txt" {
		return nil, fuse.ENOENT
	}
	if flags&fuse.O_ANYWRITE != 0 {
		return nil, fuse.EPERM
	}
	return nodefs.NewDataFile([]byte(name)), fuse.OK
}

func main() {
	flag.Parse()
	if len(flag.Args()) < 3 {
		log.Fatal("Usage:\n  lazyfs MOUNTPOINT IMGDIR RHOST:PORT")
	}

	arr := strings.Split(flag.Arg(2), ":")
	remoteHost := arr[0]
	remotePort := arr[1]

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
			remoteFiles = append(remoteFiles, RegFile{
				Fd:          *fdMap[*e.Id].Fd,
				LocalName:   "remote_open_file_" + fmt.Sprint(*fdMap[*e.Id].Fd),
				RemoteEntry: e,
			})
		}
	}

	for _, e := range remoteFiles {
		log.Println(e)
		err := e.FetchRemote(remoteHost, remotePort)
		if err != nil {
			log.Fatalf("Fetch failed: %v\n", err)
		}
	}

	// Setup fuse mount
	nfs := pathfs.NewPathNodeFs(&LazyFs{
		FileSystem: pathfs.NewDefaultFileSystem(),
		Files:      remoteFiles,
		RHost:      remoteHost,
		RPort:      remotePort,
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
