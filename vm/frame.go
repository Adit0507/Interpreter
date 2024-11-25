package vm

import (
	"monkey/code"
	"monkey/object"
)

type Frame struct {
	fn          *object.CompiledFunction //compiled function referenced by frame
	ip          int                      //instruction poointer
	basePointer int
}

func NewFrame(fn *object.CompiledFunction, basePointer int) *Frame {
	f := &Frame{fn: fn, ip: -1, basePointer: basePointer}

	return f
}

func (f *Frame) Instructions() code.Instructions {
	return f.fn.Instructions
}
