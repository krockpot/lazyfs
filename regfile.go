package main

import (
	"github.com/jakrach/lazyfs/protobuf"
)

// Regfile represents a checkpointed open file
type RegFile struct {
	Fd          uint32
	LocalName   string
	RemoteEntry *protobuf.RegFileEntry
}

func GetFile(l []RegFile, f string) RegFile {
	for _, entry := range l {
		if entry.LocalName == f {
			return entry
		}
	}
	return RegFile{}
}
