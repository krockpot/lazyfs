package main

import (
	"encoding/binary"
	"io/ioutil"

	"github.com/golang/protobuf/proto"
)

func readEntry(buf []byte) (*RegFileEntry, []byte, error) {
	size := buf[0:4]
	len := binary.LittleEndian.Uint32(size)
	entry := &RegFileEntry{}
	data := buf[4:(4 + len)]
	err := proto.Unmarshal(data, entry)
	if err != nil {
		return nil, nil, err
	}
	return entry, buf[(4 + len):], nil
}

func ReadEntries(fname string) ([](*RegFileEntry), error) {
	imgFile, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, err
	}

	newbuf := imgFile[8:]
	entries := [](*RegFileEntry){}
	for len(newbuf) != 0 {
		var tmp *RegFileEntry
		tmp, newbuf, err = readEntry(newbuf)
		if err != nil {
			return nil, err
		}
		entries = append(entries, tmp)
	}
	return entries, nil
}
