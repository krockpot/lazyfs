package main

import (
	"encoding/binary"
	"io/ioutil"

	"github.com/golang/protobuf/proto"
	pb "github.com/jakrach/lazyfs/protobuf"
)

// ImageReader interface specifies the ability to read a particular CRIU image
// into the protobuf generated structures stored in the image file.
type ImageReader interface {
	ReadEntries() ([](*proto.Message), error)
}

// RegFileImg operates on RegFileEntries from CRIU. These entries represent
// regular files used by the checkpointed process.
type RegFileImg struct {
	fname string
}

// readEntry helper method that reads a single RegFileEntry given a byte slice.
func (img RegFileImg) readEntry(buf []byte) (*pb.RegFileEntry, []byte, error) {
	len := binary.LittleEndian.Uint32(buf[0:4])
	entry := &pb.RegFileEntry{}
	err := proto.Unmarshal(buf[4:4+len], entry)
	if err != nil {
		return nil, nil, err
	}
	return entry, buf[4+len:], nil
}

// ReadEntries reads every RegFileEntry from the specified image.
func (img RegFileImg) ReadEntries() ([](*pb.RegFileEntry), error) {
	imgFile, err := ioutil.ReadFile(img.fname)
	if err != nil {
		return nil, err
	}
	newbuf := imgFile[8:] // skip magic
	entries := [](*pb.RegFileEntry){}
	for len(newbuf) != 0 {
		var tmp *pb.RegFileEntry
		tmp, newbuf, err = img.readEntry(newbuf)
		if err != nil {
			return nil, err
		}
		entries = append(entries, tmp)
	}
	return entries, nil
}

// FdinfoImg operates on FdinfoEntries from CRIU. These entries represent
// file descriptors used by the checkpointed process.
type FdinfoImg struct {
	fname string
}

// readEntry helper method that reads a single FdinfoEntry given a byte slice.
func (img FdinfoImg) readEntry(buf []byte) (*pb.FdinfoEntry, []byte, error) {
	len := binary.LittleEndian.Uint32(buf[0:4])
	entry := &pb.FdinfoEntry{}
	err := proto.Unmarshal(buf[4:4+len], entry)
	if err != nil {
		return nil, nil, err
	}
	return entry, buf[4+len:], nil
}

// ReadEntries reads every FdinfoEntry from the specified image.
func (img FdinfoImg) ReadEntries() ([](*pb.FdinfoEntry), error) {
	imgFile, err := ioutil.ReadFile(img.fname)
	if err != nil {
		return nil, err
	}
	newbuf := imgFile[8:] // skip magic
	entries := [](*pb.FdinfoEntry){}
	for len(newbuf) != 0 {
		var tmp *pb.FdinfoEntry
		tmp, newbuf, err = img.readEntry(newbuf)
		if err != nil {
			return nil, err
		}
		entries = append(entries, tmp)
	}
	return entries, nil
}
