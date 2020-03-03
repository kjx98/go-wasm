// Copyright 2016 The wasm Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wasm

import (
	"bytes"
	"errors"
	"io"
	"log"
)

var (
	errExports     = errors.New("wasm: MUST only 2 exports")
	errExpMiss     = errors.New("wasm: exports no main or memory")
	errExpError    = errors.New("wasm: exports main or memory sig error")
	errHasStart    = errors.New("wasm: start Entry not empty")
	errReadSection = errors.New("wasm: Validate Module, section malformed")
)

// Module is a WebAssembly module.
type ValModule struct {
	typ        TypeSection
	imp        ImportSection
	exp        ExportSection
	fn         FunctionSection
	startEntry bool
	buff       []byte
}

func (vm *ValModule) ReadValModule(inbuf []byte) error {
	rd := bytes.NewReader(inbuf)
	d := decoder{r: rd}
	var hdr ModuleHeader
	d.readHeader(d.r, &hdr)
	for {
		if err := vm.readSection(&d); err != nil {
			return err
		}
	}
	return nil
}

func (vm *ValModule) readSection(d *decoder) error {
	var (
		id  uint32
		sz  uint32
		sec Section
	)
	var wBuff bytes.Buffer
	dr := io.TeeReader(d.r, &wBuff)
	d.readVarU7(dr, &id)
	if d.err != nil {
		if d.err == io.EOF {
			d.err = nil
		}
		return nil
	}
	d.readVarU32(dr, &sz)

	r := &io.LimitedReader{R: dr, N: int64(sz)}
	switch SectionID(id) {
	case TypeID:
		d.readTypeSection(r, &vm.typ)
	case ImportID:
		d.readImportSection(r, &vm.imp)
	case FunctionID:
		d.readFunctionSection(r, &vm.fn)
	case ExportID:
		var s ExportSection
		d.readExportSection(r, &s)
		for _, ep := range s.Exports {
			if (ep.Field == "main" && ep.Kind == FunctionKind) ||
				(ep.Field == "memory" && ep.Kind == MemoryKind) {
				vm.exp.Exports = append(vm.exp.Exports, ep)
				if len(vm.exp.Exports) >= 2 {
					break
				}
			}
		}
	case StartID:
		vm.startEntry = true
		fallthrough
	default:
		buf := make([]byte, sz)
		d.read(r, buf)
	}
	if r.N != 0 {
		log.Printf("wasm: N=%d bytes unread! (section=%d)\n", r.N, sec.ID())
		return errReadSection
	}
	switch SectionID(id) {
	case UnknownID: // skip
	case ExportID: // filler only memory and main
		// generate new export section
		{
			var obuf []byte
			for _, ep := range vm.exp.Exports {
				ebuff := make([]byte, len(ep.Field)+2)
				ebuff[0] = byte(len(ep.Field))
				copy(ebuff[1:], []byte(ep.Field))
				ebuff[len(ep.Field)+1] = byte(ep.Kind)
				uv32 := varuint32(ep.Index)
				ebuff = append(ebuff, uv32.bytes())
				obuf = append(obuf)
			}
			if len(obuf) > 0 {
				uv32 := varuint32(len(obuf))
				vm.buff = append(vm.buff, byte(id), uv32.bytes()...)
				vm.buff = append(vm.buff, obuf...)
			}
		}
	default:
		vm.buff = append(vm.buff, wBuff.Bytes()...)
	}
	return nil
}

func (vm *ValModule) findExport(nam string) *ExportEntry {
	for i := range vm.exp.Exports {
		if vm.exp.Exports[i].Field == nam {
			return &vm.exp.Exports[i]
		}
	}
	return nil
}

func (vm *ValModule) getFuncSig(idx uint32) *FuncType {
	if int(idx) < len(vm.imp.Imports) {
		return nil
	}
	idx -= uint32(len(vm.imp.Imports))
	if int(idx) >= len(vm.fn.Types) {
		return nil
	}
	tyIdx := vm.fn.Types[idx]
	return &vm.typ.Types[tyIdx]
}

func (vm *ValModule) Validate() error {
	if len(vm.exp.Exports) != 2 {
		return errExports
	}
	if ep := vm.findExport("main"); ep == nil {
		return errExpMiss
	} else if ep.Kind != FunctionKind {
		return errExpError
	} else if typ := vm.getFuncSig(ep.Index); typ == nil ||
		len(typ.params) != 0 || len(typ.results) != 0 {
		return errExpError
	}
	if ep := vm.findExport("memory"); ep == nil {
		return errExpMiss
	} else if ep.Kind != MemoryKind || ep.Index != 0 {
		return errExpError
	}
	if vm.startEntry {
		return errHasStart
	}
	// shall we validate import
	return nil
}

func (vm *ValModule) Bytes() []byte {
	return vm.buff
}
