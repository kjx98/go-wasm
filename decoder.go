// Copyright 2016 The wasm Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wasm

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
)

func Open(name string) (Module, error) {
	f, err := os.Open(name)
	if err != nil {
		return Module{}, err
	}
	defer f.Close()

	dec := decoder{r: f}
	return dec.readModule()
}

func (d *decoder) readModule() (Module, error) {
	var (
		m   Module
		err error
	)

	if d.err != nil {
		err = d.err
		return m, err
	}

	d.readHeader(d.r, &m.Header)
	for {
		s := d.readSection()
		if s == nil {
			return m, d.err
		}
		m.Sections = append(m.Sections, s)
	}
	return m, d.err
}

func (d *decoder) readSection() Section {
	var (
		id  uint32
		sz  uint32
		sec Section
	)

	d.readVarU7(d.r, &id)
	if d.err != nil {
		if d.err == io.EOF {
			d.err = nil
		}
		return nil
	}
	d.readVarU32(d.r, &sz)

	r := &io.LimitedReader{R: d.r, N: int64(sz)}
	switch SectionID(id) {
	case UnknownID:
		var s NameSection
		d.readString(r, &s.Name)
		s.Size = int(r.N)
		// if s.Name == "name" could readNameSection
		if s.Name == "name" {
			d.readNameSection(r, &s)
		} else {
			fmt.Printf("--- name: %q, size: %d\n", s.Name, s.Size)
		}
		sec = s

	case TypeID:
		var s TypeSection
		d.readTypeSection(r, &s)
		// fmt.Printf("--- types: %d\n", len(s.Types))
		sec = s

	case ImportID:
		var s ImportSection
		d.readImportSection(r, &s)
		/*
			fmt.Printf("--- imports: %d\n", len(s.Imports))
			for ii, imp := range s.Imports {
				fmt.Printf("    entry[%d]: %q|%q|%s\n", ii, imp.Module, imp.Field, imp.Kind)
			}
		*/
		sec = s

	case FunctionID:
		var s FunctionSection
		d.readFunctionSection(r, &s)
		// fmt.Printf("--- functions: %d\n", len(s.types))
		sec = s

	case TableID:
		var s TableSection
		d.readTableSection(r, &s)
		// fmt.Printf("--- tables: %d\n", len(s.tables))
		sec = s

	case MemoryID:
		var s MemorySection
		d.readMemorySection(r, &s)
		// fmt.Printf("--- memories: %d\n", len(s.memories))
		sec = s

	case GlobalID:
		var s GlobalSection
		d.readGlobalSection(r, &s)
		// fmt.Printf("--- globals: %d\n", len(s.globals))
		/*
			for ii, ge := range s.globals {
				fmt.Printf("   ge[%d]: type={%x, 0x%x} init=%d\n",
					ii, ge.Type.ContentType, ge.Type.Mutability, len(ge.Init.Expr),
				)
			}
		*/
		sec = s

	case ExportID:
		var s ExportSection
		d.readExportSection(r, &s)
		// fmt.Printf("--- exports: %d\n", len(s.Exports))
		sec = s

	case StartID:
		var s StartSection
		d.readStartSection(r, &s)
		// fmt.Printf("--- start: 0x%x\n", s.Index)
		sec = s

	case ElementID:
		var s ElementSection
		d.readElementSection(r, &s)
		// fmt.Printf("--- elements: %d\n", len(s.elements))
		sec = s

	case CodeID:
		var s CodeSection
		d.readCodeSection(r, &s)
		// fmt.Printf("--- func-bodies: %d\n", len(s.Bodies))
		sec = s

	case DataID:
		var s DataSection
		d.readDataSection(r, &s)
		// fmt.Printf("--- data-segments: %d\n", len(s.segments))
		sec = s

	default:
		d.err = fmt.Errorf("wasm: invalid section ID")

	}

	if r.N != 0 {
		log.Printf("wasm: N=%d bytes unread! (section=%d)\n", r.N, sec.ID())
		buf := make([]byte, r.N)
		d.read(r, buf)
	}

	return sec
}

func (d *decoder) readNameSection(r io.Reader, s *NameSection) {
	for {
		if d.err != nil {
			return
		}
		var nType uint32
		d.readVarU7(r, &nType)
		if d.err != nil {
			return
		}
		var sz uint32
		d.readVarU32(r, &sz)
		if d.err != nil {
			return
		}
		rr := &io.LimitedReader{R: r, N: int64(sz)}
		switch nType {
		case 0: // Module Name
			d.readString(rr, &s.ModName)
			//log.Printf("wasm: got Module name: %s\n", s.ModName)
		case 1: // FunctionNames
			var n uint32
			d.readVarU32(rr, &n)
			s.FuncName = make([]FunctionNames, int(n))
			for i := range s.FuncName {
				d.readVarU32(rr, &s.FuncName[i].Idx)
				d.readString(rr, &s.FuncName[i].Name)
			}
		case 2: // Local
		}
		if rr.N > 0 {
			log.Printf("wasm: NameSection N=%d/%d bytes unread! (NameType=%d)\n",
				rr.N, sz, nType)
			buf := make([]byte, rr.N)
			d.read(rr, buf)
		}
	}
}

func (d *decoder) readTableSection(r io.Reader, s *TableSection) {
	if d.err != nil {
		return
	}

	var sz uint32
	d.readVarU32(r, &sz)
	s.tables = make([]TableType, int(sz))
	for i := range s.tables {
		d.readTableType(r, &s.tables[i])
	}
}

func (d *decoder) readMemorySection(r io.Reader, s *MemorySection) {
	if d.err != nil {
		return
	}

	var sz uint32
	d.readVarU32(r, &sz)
	s.memories = make([]MemoryType, int(sz))
	for i := range s.memories {
		d.readMemoryType(r, &s.memories[i])
	}
}

func (d *decoder) readGlobalSection(r io.Reader, s *GlobalSection) {
	if d.err != nil {
		return
	}

	var sz uint32
	d.readVarU32(r, &sz)
	s.globals = make([]GlobalVariable, int(sz))
	for i := range s.globals {
		d.readGlobalVariable(r, &s.globals[i])
	}
}

func (d *decoder) readGlobalVariable(r io.Reader, gv *GlobalVariable) {
	if d.err != nil {
		return
	}

	out := new(bytes.Buffer)
	r = io.TeeReader(r, out)
	d.readGlobalType(r, &gv.Type)
	d.readInitExpr(r, &gv.Init)
}

func (d *decoder) readInitExpr(r io.Reader, ie *InitExpr) {
	if d.err != nil {
		return
	}

	var err error
	var n int
	var buf [1]byte
	n, err = r.Read(buf[:])
	if err != nil || n <= 0 {
		return
	}
	switch Opcode(buf[0]) {
	case Op_i32_const:
		fallthrough
	case Op_i64_const:
		d.readVarI32(r, &ie.Expr)
	default: // error
		d.err = errInvOp
	}
	n, err = r.Read(buf[:])
	if err != nil || n <= 0 {
		d.err = err
		return
	}
	v := buf[0]
	if v != Op_end {
		// error
		d.err = errOpEnd
	}
}

func (d *decoder) readStartSection(r io.Reader, s *StartSection) {
	if d.err != nil {
		return
	}

	d.readVarU32(r, &s.Index)
}

func (d *decoder) readElementSection(r io.Reader, s *ElementSection) {
	if d.err != nil {
		return
	}

	var sz uint32
	d.readVarU32(r, &sz)
	s.elements = make([]ElemSegment, int(sz))
	for i := range s.elements {
		d.readElemSegment(r, &s.elements[i])
	}
}

func (d *decoder) readElemSegment(r io.Reader, es *ElemSegment) {
	if d.err != nil {
		return
	}

	d.readVarU32(r, &es.Index)
	d.readInitExpr(r, &es.Offset)

	var sz uint32
	d.readVarU32(r, &sz)
	es.Elems = make([]uint32, int(sz))
	for i := range es.Elems {
		d.readVarU32(r, &es.Elems[i])
	}
}

func (d *decoder) readCodeSection(r io.Reader, s *CodeSection) {
	if d.err != nil {
		return
	}

	var sz uint32
	d.readVarU32(r, &sz)
	s.Bodies = make([]FunctionBody, int(sz))
	for i := range s.Bodies {
		d.readFunctionBody(r, &s.Bodies[i])
	}
}

func (d *decoder) readFunctionBody(r io.Reader, fb *FunctionBody) {
	if d.err != nil {
		return
	}

	d.readVarU32(r, &fb.BodySize)
	r = io.LimitReader(r, int64(fb.BodySize))
	var locals uint32
	d.readVarU32(r, &locals)
	fb.Locals = make([]LocalEntry, int(locals))
	for i := range fb.Locals {
		d.readLocalEntry(r, &fb.Locals[i])
	}

	fb.Code, d.err = ioutil.ReadAll(r)
}

func (d *decoder) readLocalEntry(r io.Reader, le *LocalEntry) {
	if d.err != nil {
		return
	}

	d.readVarU32(r, &le.Count)
	d.readValueType(r, &le.Type)
}

func (d *decoder) readDataSection(r io.Reader, s *DataSection) {
	if d.err != nil {
		return
	}

	var sz uint32
	d.readVarU32(r, &sz)
	s.segments = make([]DataSegment, int(sz))
	for i := range s.segments {
		d.readDataSegment(r, &s.segments[i])
	}
}

func (d *decoder) readDataSegment(r io.Reader, ds *DataSegment) {
	if d.err != nil {
		return
	}

	d.readVarU32(r, &ds.Index)
	d.readInitExpr(r, &ds.Offset)

	var sz uint32
	d.readVarU32(r, &sz)
	ds.Data = make([]byte, int(sz))
	d.read(r, ds.Data)
}
