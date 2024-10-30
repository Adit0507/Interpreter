package compiler

import (
	"fmt"
	"monkey/code"
	"monkey/object"
	"testing"
)

type compilerTestCase struct {
	input                string
	expectedConstants    []interface{}
	expectedInstructions []code.Instructions
}

func TestIntegerArthimetic(t *testing.T) {
	tests := []compilerTestCase{
		{
			input:             "1 + 2",
			expectedConstants: []interface{}{1, 2},
			expectedInstructions: []code.Instructions{
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 1),
			},
		},
	}

	runCompilerTests(t, tests)
}

func runCompilerTests(t *testing.T, tests []compilerTestCase) {
	t.Helper()
	for _, tt := range tests {
		program := parse(tt.input)

		compiler := New()
		err := compiler.Compile(program)
		if err != nil {
			t.Fatalf("compiler error : %s", err)
		}

		bytecode := compiler.Bytecode()
		err = testInstructions(tt.expectedInstructions, bytecode.Instructions)
		if err != nil {
			t.Fatalf("testInstructions failed: %s", err)
		}
		err = testConstants(t, tt.expectedConstants, bytecode.Constants)
		if err != nil {
			t.Fatalf("testConstants failed: %s", err)
		}
	}
}

func testConstants(t *testing.T, expected []interface{}, actual []object.Object) error {
		if len(expected) != len(actual) {
			return fmt.Errorf("wrong no. of constants. got=%d, want=%d", len(actual), len(expected))
		}

		for i, constant := range expected {
			switch constant := constant.(type) {
			case int:
				err := testIntegerObject(int64(constant), actual[i])
				if err!= nil {
					return fmt.Errorf("constant %d- testIntegerObject failed: %s", i, err)
				}
			}
		}

	return nil
}

func testIntegerObject(expected int64, actual object.Object) error {
	res, ok := actual.(*object.Integer)
	if !ok {
		return fmt.Errorf("object is not integer, got=%T (%+v)", actual, actual)
	}

	if res.Value != expected {
		return fmt.Errorf("object has wrong value, got=%d, want=%d", res.Value, expected)
	}

	return nil
}

func testInstructions(expected []code.Instructions, actual code.Instructions) error {
	concated := concatInstructions(expected)

	if len(actual) != len(concated) {
		return fmt.Errorf("wrong instructions length.\nwant=%q\ngot =%q", concated, actual)
	}

	for i, ins := range concated {
		if actual[i] != ins {
			fmt.Errorf("wrong instruction at %d.\nwant=%q\ngot =%q", i, concated, actual)
		}
	}

	return nil
}

func concatInstructions(expected []code.Instructions) code.Instructions {
	out := code.Instructions{}

	for _, ins := range expected {
		out = append(out, ins...)
	}

	return out
}