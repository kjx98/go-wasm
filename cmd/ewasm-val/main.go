// Copyright 2016 The wasm Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/kjx98/go-wasm"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("wasm>> ")
	var oPath string
	flag.StringVar(&oPath, "out", "/tmp", "output directory")
	flag.Parse()

	fname := flag.Arg(0)
	oname := oPath + "/" + path.Base(fname)
	var inBuff []byte
	if f, err := os.Open(fname); err != nil {
		log.Fatal(err)
	} else {
		inBuff, err = ioutil.ReadAll(f)
		if err != nil {
			log.Fatal(err)
		}
		f.Close()
	}
	var mod wasm.ValModule
	if err := mod.ReadValModule(inBuff); err != nil {
		log.Fatal("Read and Validate Module", err)
	}
	if err := mod.Validate(); err != nil {
		log.Fatal("Module Validate()", err)
	}
	if err := ioutil.WriteFile(oname, mod.Bytes(), 0666); err != nil {
		log.Fatal("WriteFile", err)
	}
}
