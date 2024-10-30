package compiler

import (
	"monkey/parser"
	"monkey/ast"
	"monkey/code"
	"monkey/lexer"
	"monkey/object"
)

type Compiler struct {
	instructions code.Instructions
	constants []object.Object
}

func New() *Compiler {
	return &Compiler{
		instructions: code.Instructions{},
		constants: []object.Object{},
	}
}

func (c *Compiler) Compile(node ast.Node) error {
	return nil
}

type Bytecode struct {
	Instructions code.Instructions
	Constants []object.Object
}

func (c *Compiler) Bytecode() *Bytecode {
	return &Bytecode{
		Instructions: c.instructions,
		Constants: c.constants,
	}
}

func parse(input string) *ast.Program {
	l := lexer.New(input)
	p := parser.New(l)

	return p.ParseProgram()
}
