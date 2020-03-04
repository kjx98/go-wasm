// Copyright 2016 The wasm Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wasm

import (
	"encoding/binary"
	"errors"
	"io"
)

var order = binary.LittleEndian

var (
	errOverflow = errors.New("wasm: overflow")
	errInvOp    = errors.New("wasm: invalid Op code")
	errOpEnd    = errors.New("wasm: must be Op_End")
	errMalform  = errors.New("wasm: varint/varuint malformed")
)

type (
	varuint32 uint32
	varuint7  uint32
	varuint1  uint32

	varint64 int64
	varint32 int32
	varint7  int32
)

func (v *varuint32) read(r io.Reader) (int, error) {
	vv, n, err := uvarint(r)
	if err != nil {
		return n, err
	}
	*v = varuint32(vv)
	return n, nil
}

func (v *varuint32) write(w io.Writer) error {
	bb := v.bytes()
	_, err := w.Write(bb)
	return err
}

func (v *varuint7) read(r io.Reader) (int, error) {
	vv, n, err := uvarint(r)
	if err != nil {
		return n, err
	}
	*v = varuint7(vv)
	return n, nil
}

func (v *varuint7) write(w io.Writer) error {
	bb := make([]byte, 1)
	bb[0] = byte(*v & 0x7f)
	_, err := w.Write(bb)
	return err
}

func (vp *varuint32) bytes() []byte {
	v := uint32(*vp)
	ret := make([]byte, 5)
	var i int
	for i = 0; i < len(ret); i++ {
		ret[i] = byte(v & 0x7f)
		v >>= 7
		if v == 0 {
			i++
			break
		} else {
			ret[i] |= 0x80
		}
	}
	return ret[:i]
}

// uvarint for uvar1/uvar7/uvar32, no uvar64
func uvarint(r io.Reader) (uint32, int, error) {
	var x uint32
	var s uint
	var buf = make([]byte, 1)
	for i := 0; ; i++ {
		_, err := r.Read(buf)
		if err != nil {
			return 0, i, err
		}
		b := buf[0]
		if b < 0x80 {
			if i > 4 || i == 4 && b > 15 {
				return 0, i, errOverflow
			}
			return x | uint32(b)<<s, i + 1, nil
		}
		x |= uint32(b&0x7f) << s
		s += 7
	}
	panic("unreachable")
}

// varint for var7/var32/var64
func varint(r io.Reader) (int64, int, error) {
	var x int64
	var s uint
	var buf = make([]byte, 1)
	for i := 0; ; i++ {
		_, err := r.Read(buf)
		if err != nil {
			return 0, i, err
		}
		b := buf[0]
		if b < 0x80 {
			if i > 9 || i == 9 && b > 1 {
				return 0, i, errOverflow
			}
			if (b & 0x40) != 0 {
				b |= 0x80
			}
			return x | int64(int8(b))<<s, i + 1, nil
		}
		x |= int64(b&0x7f) << s
		s += 7
	}
	panic("unreachable")
}

type ValueType int8

// 0x7f: i32
// 0x7e: i64
// 0x7d: f32
// 0x7c: f64
// 0x70: anyfunc
// 0x60: func
// 0x40: pseudo type for an empty block_type
const (
	ValueI32     ValueType = -0x01
	ValueI64               = -0x02
	ValueF32               = -0x03
	ValueF64               = -0x04
	ValueAnyFunc           = -0x10
	ValueFunc              = -0x20
	ValueBlock             = -0x40
)

func (v ValueType) String() string {
	switch v {
	case ValueI32:
		return "i32"
	case ValueI64:
		return "i64"
	case ValueF32:
		return "f32"
	case ValueF64:
		return "f64"
	case ValueAnyFunc:
		return "anyfunc"
	case ValueFunc:
		return "func"
	case ValueBlock:
		return "block_type"
	}
	return "unknown"
}

type BlockType varint7
type ElemType varint7

type FuncType struct {
	form    ValueType   // value for the 'func' type constructor
	params  []ValueType // parameters of the function
	results []ValueType // results of the function
}

func (fn *FuncType) String() string {
	ret := "(" + fn.form.String()
	if len(fn.params) > 0 {
		ret += " (param"
		for _, tt := range fn.params {
			ret += " " + tt.String()
		}
		ret += ")"
	}
	if len(fn.results) > 0 {
		ret += " (result "
		ret += fn.results[0].String()
		ret += ")"
	}
	ret += ")"
	return ret
}

// GlobalType describes a global variable
type GlobalType struct {
	ContentType ValueType
	Mutability  varuint1 // 0:immutable, 1:mutable
}

// TableType describes a table
type TableType struct {
	ElemType ElemType // the type of elements
	Limits   ResizableLimits
}

// MemoryType describes a memory
type MemoryType struct {
	Limits ResizableLimits
}

// ExternalKind indicates the kind of definition being imported or defined:
// 0: indicates a Function import or definition
// 1: indicates a Table import or definition
// 2: indicates a Memory import or definition
// 3: indicates a Global import or definition
type ExternalKind byte

func (v ExternalKind) String() string {
	switch v {
	case FunctionKind:
		return "func"
	case TableKind:
		return "table"
	case MemoryKind:
		return "memory"
	case GlobalKind:
		return "global"
	}
	return "unknown"
}

// 0: indicates a Function import or definition
// 1: indicates a Table import or definition
// 2: indicates a Memory import or definition
// 3: indicates a Global import or definition
const (
	FunctionKind ExternalKind = 0
	TableKind                 = 1
	MemoryKind                = 2
	GlobalKind                = 3
)

// ResizableLimits describes the limits of a table or memory
type ResizableLimits struct {
	Flags   uint32 // bit 0x1 is set if the maximum field is present
	Initial uint32 // initial length (in units of table elements or wasm pages)
	Maximum uint32 // only present if specified by Flags
}

// InitExpr encodes an initializer expression.
// only i32.const support, i64.const convert to i32
// FIXME
type InitExpr struct {
	Value int64
	//Expr  []byte
}
