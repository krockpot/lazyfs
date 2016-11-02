package main

import (
	"encoding/binary"
	"io/ioutil"

	"github.com/golang/protobuf/proto"
	pb "github.com/jakrach/lazyfs/protobuf"
)

type ImageReader interface {
	ReadEntries() ([](*proto.Message), error)
}

type RegFileImg struct {
	fname string
}

func (img RegFileImg) readEntry(buf []byte) (*pb.RegFileEntry, []byte, error) {
	len := binary.LittleEndian.Uint32(buf[0:4])
	entry := &pb.RegFileEntry{}
	err := proto.Unmarshal(buf[4:4+len], entry)
	if err != nil {
		return nil, nil, err
	}
	return entry, buf[4+len:], nil
}

func (img RegFileImg) ReadEntries() ([](*pb.RegFileEntry), error) {
	imgFile, err := ioutil.ReadFile(img.fname)
	if err != nil {
		return nil, err
	}

	newbuf := imgFile[8:]
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

type FdinfoImg struct {
	fname string
}

func (img FdinfoImg) readEntry(buf []byte) (*pb.FdinfoEntry, []byte, error) {
	len := binary.LittleEndian.Uint32(buf[0:4])
	entry := &pb.FdinfoEntry{}
	err := proto.Unmarshal(buf[4:4+len], entry)
	if err != nil {
		return nil, nil, err
	}
	return entry, buf[4+len:], nil
}

func (img FdinfoImg) ReadEntries() ([](*pb.FdinfoEntry), error) {
	imgFile, err := ioutil.ReadFile(img.fname)
	if err != nil {
		return nil, err
	}

	newbuf := imgFile[8:]
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
