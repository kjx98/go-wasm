// Copyright 2016 The wasm Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wasm

import (
	"bytes"
	"fmt"
	"testing"
)

func TestOpen(t *testing.T) {
	mod, err := Open("testdata/hello.wasm")
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("module header: %v\n", mod.Header)
	fmt.Printf("#sections: %d\n", len(mod.Sections))
}

func TestEnVar(t *testing.T) {
	tests := []struct {
		arg  varuint32
		want []byte
	}{
		{20, []byte{20}},
		{129, []byte{129, 1}},
		{65536, []byte{128, 128, 4}},
		{259, []byte{131, 2}},
	}

	for _, tt := range tests {
		got := tt.arg.bytes()
		if bytes.Compare(got, tt.want) != 0 {
			t.Errorf("encode varuint32.bytes() = %v, want %v", got, tt.want)
		}
	}
}
