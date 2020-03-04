// Copyright 2016 The wasm Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wasm

import (
	//"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
)

type decoder struct {
	r   io.Reader
	err error
}

func (d *decoder) readVarI7(r io.Reader, v *int32) {
	if d.err != nil {
		return
	}
	var n int
	var vv int64
	vv, n, d.err = varint(r)
	if d.err == nil && n != 1 {
		d.err = errMalform
	}
	*v = int32(vv)
}

func (d *decoder) readVarI32(r io.Reader, v *int32) {
	if d.err != nil {
		return
	}
	var vv int64
	vv, _, d.err = varint(r)
	*v = int32(vv)
}

func (d *decoder) readVarI64(r io.Reader, v *int64) {
	if d.err != nil {
		return
	}
	*v, _, d.err = varint(r)
}

func (d *decoder) readVarU1(r io.Reader, v *uint32) {
	// FIXME ?
	d.readVarU7(r, v)
}

func (d *decoder) readVarU7(r io.Reader, v *uint32) {
	if d.err != nil {
		return
	}
	var n int
	*v, n, d.err = uvarint(r)
	if d.err == nil && n != 1 {
		d.err = errMalform
	}
}

func (d *decoder) readVarU32(r io.Reader, v *uint32) {
	if d.err != nil {
		return
	}
	*v, _, d.err = uvarint(r)
}

func (d *decoder) readString(r io.Reader, s *string) {
	if d.err != nil {
		return
	}
	var sz uint32
	d.readVarU32(r, &sz)
	if d.err != nil {
		return
	}
	var buf = make([]byte, sz)
	d.read(r, buf)
	*s = string(buf)
}

func (d *decoder) read(r io.Reader, buf []byte) {
	if d.err != nil || len(buf) == 0 {
		return
	}
	_, d.err = r.Read(buf)
}

func (d *decoder) readHeader(r io.Reader, hdr *ModuleHeader) {
	if d.err != nil {
		return
	}
	d.err = binary.Read(r, order, hdr)
	if d.err != nil {
		return
	}

	if hdr.Magic != magicWASM {
		d.err = fmt.Errorf("wasm: invalid magic number (%q)", string(hdr.Magic[:]))
		return
	}
}

func (d *decoder) readTypeSection(r io.Reader, s *TypeSection) {
	var n uint32
	d.readVarU32(r, &n)
	if d.err != nil {
		return
	}

	s.Types = make([]FuncType, int(n))
	for i := range s.Types {
		d.readFuncType(r, &s.Types[i])
	}
}

func (d *decoder) readFuncType(r io.Reader, ft *FuncType) {
	if d.err != nil {
		return
	}

	d.readValueType(r, &ft.form)

	var params uint32
	d.readVarU32(r, &params)
	if d.err != nil {
		return
	}
	ft.params = make([]ValueType, int(params))
	for i := range ft.params {
		d.readValueType(r, &ft.params[i])
	}

	var results uint32
	d.readVarU32(r, &results)
	if d.err != nil {
		return
	}
	ft.results = make([]ValueType, int(results))
	for i := range ft.results {
		d.readValueType(r, &ft.results[i])
	}
}

func (d *decoder) readValueType(r io.Reader, vt *ValueType) {
	if d.err != nil {
		return
	}

	var v int32
	d.readVarI7(r, &v)
	*vt = ValueType(v)
}

func (d *decoder) readImportSection(r io.Reader, s *ImportSection) {
	var sz uint32
	d.readVarU32(r, &sz)
	if d.err != nil {
		return
	}

	s.Imports = make([]ImportEntry, int(sz))
	for i := range s.Imports {
		d.readImportEntry(r, &s.Imports[i])
	}
}

func (d *decoder) readImportEntry(r io.Reader, ie *ImportEntry) {
	if d.err != nil {
		return
	}

	d.readString(r, &ie.Module)
	d.readString(r, &ie.Field)
	d.readExternalKind(r, &ie.Kind)

	switch ie.Kind {
	case FunctionKind:
		var idx uint32
		d.readVarU32(r, &idx)
		ie.Typ = idx

	case TableKind:
		var tt TableType
		d.readTableType(r, &tt)
		ie.Typ = tt

	case MemoryKind:
		var mt MemoryType
		d.readMemoryType(r, &mt)
		ie.Typ = mt

	case GlobalKind:
		var gt GlobalType
		d.readGlobalType(r, &gt)
		ie.Typ = gt

	default:
		log.Printf("module=%q field=%q\n", ie.Module, ie.Field)
		d.err = fmt.Errorf("wasm: invalid ExternalKind (%d)", byte(ie.Kind))
	}
}

func (d *decoder) readExternalKind(r io.Reader, ek *ExternalKind) {
	if d.err != nil {
		return
	}

	var v [1]byte
	d.read(r, v[:])
	*ek = ExternalKind(v[0])
}

func (d *decoder) readTableType(r io.Reader, tt *TableType) {
	if d.err != nil {
		return
	}

	d.readElemType(r, &tt.ElemType)
	d.readResizableLimits(r, &tt.Limits)
}

func (d *decoder) readElemType(r io.Reader, et *ElemType) {
	if d.err != nil {
		return
	}

	var v int32
	d.readVarI7(r, &v)
	*et = ElemType(v)
}

func (d *decoder) readResizableLimits(r io.Reader, tl *ResizableLimits) {
	if d.err != nil {
		return
	}

	d.readVarU32(r, &tl.Flags)
	d.readVarU32(r, &tl.Initial)
	if (tl.Flags & 0x1) != 0 {
		d.readVarU32(r, &tl.Maximum)
	}
}

func (d *decoder) readMemoryType(r io.Reader, mt *MemoryType) {
	if d.err != nil {
		return
	}

	d.readResizableLimits(r, &mt.Limits)
}

func (d *decoder) readGlobalType(r io.Reader, gt *GlobalType) {
	if d.err != nil {
		return
	}

	d.readValueType(r, &gt.ContentType)
	var mut uint32
	d.readVarU1(r, &mut)
	gt.Mutability = varuint1(mut)
}

func (d *decoder) readFunctionSection(r io.Reader, s *FunctionSection) {
	var sz uint32
	d.readVarU32(r, &sz)
	if d.err != nil {
		return
	}

	s.Types = make([]uint32, int(sz))
	for i := range s.Types {
		d.readVarU32(r, &s.Types[i])
	}
}

func (d *decoder) readExportSection(r io.Reader, s *ExportSection) {
	var sz uint32
	d.readVarU32(r, &sz)
	if d.err != nil {
		return
	}

	s.Exports = make([]ExportEntry, int(sz))
	for i := range s.Exports {
		d.readExportEntry(r, &s.Exports[i])
	}
}

func (d *decoder) readExportEntry(r io.Reader, ee *ExportEntry) {
	if d.err != nil {
		return
	}

	d.readString(r, &ee.Field)
	d.readExternalKind(r, &ee.Kind)
	d.readVarU32(r, &ee.Index)
}
