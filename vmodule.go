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
	errHead          = errors.New("wasm: header missing")
	errExports       = errors.New("wasm: MUST only 2 exports")
	errExpMiss       = errors.New("wasm: exports no main or memory")
	errExpError      = errors.New("wasm: exports main or memory sig error")
	errHasStart      = errors.New("wasm: start Entry not empty")
	errHasCustom     = errors.New("wasm: MUST strip custom section")
	errReadSection   = errors.New("wasm: Validate Module, section malformed")
	errImportFunc    = errors.New("wasm: Validate, unsolved import")
	errImportNotFunc = errors.New("wasm: Validate, import not func")
)

// Module is a WebAssembly module.
type ValModule struct {
	OnlyValidate bool
	typ          TypeSection
	imp          ImportSection
	exp          ExportSection
	fn           FunctionSection
	startEntry   bool
	bCustom      bool
	buff         []byte
}

func (vm *ValModule) ReadValModule(inbuf []byte) error {
	rd := bytes.NewReader(inbuf)
	d := decoder{r: rd}
	var hdr ModuleHeader
	d.readHeader(d.r, &hdr)
	if d.err != nil {
		return errHead
	}
	vm.buff = inbuf[:8]
	for {
		if err := vm.readSection(&d); err != nil {
			if err == io.EOF {
				break
			}
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
	out := new(bytes.Buffer)
	dr := io.TeeReader(d.r, out)
	d.readVarU7(dr, &id)
	if d.err != nil {
		return d.err
	}
	d.readVarU32(dr, &sz)
	if d.err != nil {
		return errReadSection
	}

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
				//log.Printf("Got export %s %s\n", ep.Field, ep.Kind)
				vm.exp.Exports = append(vm.exp.Exports, ep)
				if len(vm.exp.Exports) >= 2 {
					break
				}
			}
		}
	default:
		buf := make([]byte, sz)
		d.read(r, buf)
	}
	if d.err != nil {
		return errReadSection
	}
	if r.N != 0 {
		log.Printf("wasm: N=%d bytes unread! (section=%d)\n", r.N, sec.ID())
		return errReadSection
	}
	switch SectionID(id) {
	case UnknownID: // skip
		vm.bCustom = true
	case ExportID: // filler only memory and main
		// generate new export section
		if !vm.OnlyValidate {
			var obuf []byte
			for _, ep := range vm.exp.Exports {
				namLen := len(ep.Field)
				if namLen > 64 {
					return errReadSection
				}
				ebuff := make([]byte, namLen+2)
				ebuff[0] = byte(namLen)
				copy(ebuff[1:], []byte(ep.Field))
				ebuff[namLen+1] = byte(ep.Kind)
				uv32 := varuint32(ep.Index)
				ebuff = append(ebuff, uv32.bytes()...)
				obuf = append(obuf, ebuff...)
				/*
					log.Printf("encode export %s len: %d, %v\n", ep.Field,
						len(ebuff), ebuff)
				*/
			}
			if len(obuf) > 0 {
				var ebuff []byte
				nExp := len(vm.exp.Exports)
				if nExp > 64 {
					return errExports
				}
				uv32 := varuint32(len(obuf) + 1)
				ebuff = append(ebuff, byte(id))
				ebuff = append(ebuff, uv32.bytes()...)
				ebuff = append(ebuff, byte(nExp))
				ebuff = append(ebuff, obuf...)
				//log.Printf("export section len: %d, %v\n", len(ebuff), ebuff)
				vm.buff = append(vm.buff, ebuff...)
			}
		}
	case StartID:
		vm.startEntry = true
		fallthrough
	default:
		if !vm.OnlyValidate {
			vm.buff = append(vm.buff, out.Bytes()...)
		}
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

type funcMap struct {
	params  []ValueType
	results []ValueType
}

var dbgMap = map[string]funcMap{
	"print":           {params: []ValueType{ValueI32, ValueI32}},
	"printMem":        {params: []ValueType{ValueI32, ValueI32}},
	"printMemHex":     {params: []ValueType{ValueI32, ValueI32}},
	"printStorage":    {params: []ValueType{ValueI32}},
	"printStorageHex": {params: []ValueType{ValueI32}},
}

var ethMap = map[string]funcMap{
	"finish":             {params: []ValueType{ValueI32, ValueI32}},
	"revert":             {params: []ValueType{ValueI32, ValueI32}},
	"getAddress":         {params: []ValueType{ValueI32}},
	"getExternalBalance": {params: []ValueType{ValueI32, ValueI32}},
	"storageStore":       {params: []ValueType{ValueI32, ValueI32}},
	"storageLoad":        {params: []ValueType{ValueI32, ValueI32}},
	"getCaller":          {params: []ValueType{ValueI32}},
	"getTxOrigin":        {params: []ValueType{ValueI32}},
	"log":                {params: []ValueType{ValueI32, ValueI32, ValueI32, ValueI32, ValueI32, ValueI32, ValueI32}},
	"selfDestruct":       {params: []ValueType{ValueI32}},
	"useGas":             {params: []ValueType{ValueI64}},
	"getCallValue":       {params: []ValueType{ValueI32}},
	"getCallDataSize":    {results: []ValueType{ValueI32}},
	"callDataCopy":       {params: []ValueType{ValueI32, ValueI32, ValueI32}},
	"getCodeSize":        {results: []ValueType{ValueI32}},
	"codeCopy":           {params: []ValueType{ValueI32, ValueI32, ValueI32}},
	"getGasLeft":         {results: []ValueType{ValueI64}},
	"getBlockGasLimit":   {results: []ValueType{ValueI64}},
	"getTxGasPrice":      {params: []ValueType{ValueI32}},
	"getBlockNumber":     {results: []ValueType{ValueI64}},
	"getBlockTimestamp":  {results: []ValueType{ValueI64}},
}

func eqValues(lv, rv []ValueType) bool {
	if ll := len(lv); ll != len(rv) {
		return false
	} else {
		for i := range lv {
			if lv[i] != rv[i] {
				return false
			}
		}
	}
	return true
}

func solveImport(modName string, fn string, typ *FuncType) bool {
	verify := func(mm map[string]funcMap) bool {
		if sig, ok := mm[fn]; !ok {
			log.Printf("unsolved import: mod(%s) func(%s)\n", modName, fn)
			return false
		} else if !eqValues(typ.params, sig.params) {
			log.Printf("param sig dismatch %v want %v\n", sig.params,
				typ.params)
			return false
		} else if !eqValues(typ.results, sig.results) {
			log.Printf("result sig dismatch %v want %v\n", sig.results,
				typ.results)
			return false
		}
		return true
	}
	if modName == "debug" {
		return verify(dbgMap)
	} else if modName != "ethereum" {
		log.Printf("unknown module: %s\n", modName)
		return false
	}
	return verify(ethMap)
	// return true
}

func (vm *ValModule) Validate() error {
	if vm.startEntry {
		return errHasStart
	}
	if vm.OnlyValidate && vm.bCustom {
		return errHasCustom
	}
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
	// shall we validate import
	for _, imp := range vm.imp.Imports {
		if imp.Kind != FunctionKind {
			return errImportNotFunc
		}
		if idx, ok := imp.Typ.(uint32); !ok {
			log.Printf("func idx not uint32: %v\n", imp.Typ)
			return errImportFunc
		} else if int(idx) >= len(vm.typ.Types) {
			log.Printf("no func sig for idx: %d\n", idx)
			return errImportFunc
		} else if !solveImport(imp.Module, imp.Field, &vm.typ.Types[idx]) {
			return errImportFunc
		}
	}
	return nil
}

func (vm *ValModule) Bytes() []byte {
	return vm.buff
}
