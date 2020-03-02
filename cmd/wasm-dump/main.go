// Copyright 2016 The wasm Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/kjx98/go-wasm"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("wasm>> ")

	flag.Parse()

	fname := flag.Arg(0)
	mod, err := wasm.Open(fname)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("module header: %v\n", mod.Header)
	fmt.Printf("#sections: %d\n", len(mod.Sections))
	for _, section := range mod.Sections {
		fmt.Printf("section: %2d (%T)\n", section.ID(), section)
		if section.ID() == wasm.ExportID {
			sec := section.(wasm.ExportSection)
			for _, exEntry := range sec.Exports {
				fmt.Printf("Export %s %v @%d\n", exEntry.Field, exEntry.Kind, exEntry.Index)
			}
		} else if section.ID() == wasm.TypeID {
			sec := section.(wasm.TypeSection)
			for idx, tyEntry := range sec.Types {
				fmt.Printf("(type $%d %s)\n", idx, tyEntry.String())
			}
		} else if section.ID() == wasm.UnknownID {
			sec := section.(wasm.NameSection)
			fmt.Printf("Custom Section (%s), size: %d\n", sec.Name, sec.Size)
			if len(sec.ModName) > 0 {
				fmt.Printf("Module Name: %s\n", sec.ModName)
			}
			for _, fn := range sec.FuncName {
				fmt.Printf("Func$%d Name: %s\n", fn.Idx, fn.Name)
			}
		} else if section.ID() == wasm.ImportID {
			s := section.(wasm.ImportSection)
			fmt.Printf("Imports: %d\n", len(s.Imports))
			for ii, imp := range s.Imports {
				fmt.Printf("    entry[%d]: %q|%q|%s %v\n", ii, imp.Module,
					imp.Field, imp.Kind, imp.Typ)
			}
		}
	}
}
