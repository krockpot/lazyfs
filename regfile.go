package main

import (
	"fmt"
	"os/exec"
	//"syscall"

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

func (f *RegFile) FetchRemote(rhost, rport string) error {
	/*
		saved := syscall.Getuid()
		err := syscall.Setuid(0)
		if err != nil {
			return err
		}
	*/
	fname := f.RemoteEntry.Name
	cmd := exec.Command("scp", "-P "+rport,
		rhost+":"+*fname, *fname)
	fmt.Println(cmd)
	/*
		err = syscall.Setuid(saved)
		if err != nil {
			return err
		}
	*/
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}
