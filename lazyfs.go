// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// A Go mirror of libfuse's hello.c

package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	pb "github.com/jakrach/lazyfs/protobuf"
)

type LazyFs struct {
	pathfs.FileSystem
}

func (me *LazyFs) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	switch name {
	case "file.txt":
		return &fuse.Attr{
			Mode: fuse.S_IFREG | 0644, Size: uint64(len(name)),
		}, fuse.OK
	case "":
		return &fuse.Attr{
			Mode: fuse.S_IFDIR | 0755,
		}, fuse.OK
	}
	return nil, fuse.ENOENT
}

func (me *LazyFs) OpenDir(name string, context *fuse.Context) (c []fuse.DirEntry, code fuse.Status) {
	if name == "" {
		c = []fuse.DirEntry{{Name: "file.txt", Mode: fuse.S_IFREG}}
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
	if len(flag.Args()) < 2 {
		log.Fatal("Usage:\n  lazyfs MOUNTPOINT IMGDIR")
	}
	nfs := pathfs.NewPathNodeFs(&LazyFs{FileSystem: pathfs.NewDefaultFileSystem()}, nil)
	server, _, err := nodefs.MountRoot(flag.Arg(0), nfs.Root(), nil)
	if err != nil {
		log.Fatalf("Mount fail: %v\n", err)
	}

	// Grab the regular files and parse into a map
	regFileMap := map[uint32]*pb.RegFileEntry{}
	imgRegFiles := RegFileImg{path.Join(flag.Arg(1), "reg-files.img")}
	entries, err := imgRegFiles.ReadEntries()
	if err != nil {
		log.Fatalf("Image read fail: %v\n", err)
	}
	for _, e := range entries {
		if *e.Flags != 0 {
			regFileMap[*e.Id] = e
		}
	}

	// Grab all the files in the image directory
	files, err := ioutil.ReadDir(flag.Arg(1))
	if err != nil {
		log.Fatalf("ReadDir fail: %v=\n", err)
	}

	// Iterate over image files, grab fdinfo and parse into a map
	fdInfoMap := map[uint32]*pb.FdinfoEntry{}
	for _, f := range files {
		if (len(f.Name()) > len("fdinfo")) &&
			(f.Name()[:len("fdinfo")] == "fdinfo") {
			imgFdinfo := FdinfoImg{path.Join(flag.Arg(1), f.Name())}
			fdEntries, err := imgFdinfo.ReadEntries()
			if err != nil {
				log.Fatalf("Image read fail: %v\n", err)
			}
			for _, e := range fdEntries {
				fdInfoMap[*e.Id] = e
			}
		}
	}

	// TODO Print Testing
	log.Println(regFileMap)
	log.Println(fdInfoMap)

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
