package main

import (
	"encoding/binary"
	"fmt"
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

func readEntries(fname string) [](*RegFileEntry) {
	imgFile, err := ioutil.ReadFile(fname)
	if err != nil {
		panic(err)
	}

	newbuf := imgFile[8:]
	entries := [](*RegFileEntry){}
	for len(newbuf) != 0 {
		var tmp *RegFileEntry
		tmp, newbuf, err = readEntry(newbuf)
		if err != nil {
			panic(err)
		}
		entries = append(entries, tmp)
	}
	return entries
}

func main() {
	for _, e := range readEntries("./test.img") {
		fmt.Println(e)
	}
}
