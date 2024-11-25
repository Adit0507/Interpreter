package vm

import (
	"fmt"
	"monkey/code"
	"monkey/compiler"
	"monkey/object"
)

const StackSize = 2048

const GlobalSize = 65536

const MaxFrames = 1024

var True = &object.Boolean{Value: true}
var False = &object.Boolean{Value: false}
var Null = &object.Null{}

type VM struct {
	constants   []object.Object
	stack       []object.Object
	sp          int
	globals     []object.Object
	frames      []*Frame
	framesIndex int
}

func (vm *VM) currentFrame() *Frame {
	return vm.frames[vm.framesIndex-1]
}
func (vm *VM) pushFrame(f *Frame) {
	vm.frames[vm.framesIndex] = f
	vm.framesIndex++
}
func (vm *VM) popFrames() *Frame {
	vm.framesIndex--
	return vm.frames[vm.framesIndex]
}

func NewWithGlobalStore(bytecode *compiler.Bytecode, s []object.Object) *VM {
	vm := New(bytecode)
	vm.globals = s
	return vm
}

func New(bytecode *compiler.Bytecode) *VM {
	mainFn := &object.CompiledFunction{Instructions: bytecode.Instructions}
	mainFrame := NewFrame(mainFn, 0)

	frames := make([]*Frame, MaxFrames)
	frames[0] = mainFrame

	return &VM{
		constants:   bytecode.Constants,
		stack:       make([]object.Object, StackSize),
		sp:          0,
		globals:     make([]object.Object, GlobalSize),
		frames:      frames,
		framesIndex: 1,
	}
}

func (vm *VM) LastPoppedStackElem() object.Object {
	return vm.stack[vm.sp]
}

func (vm *VM) StackTop() object.Object {
	if vm.sp == 0 {
		return nil
	}

	return vm.stack[vm.sp-1]
}

func (vm *VM) buildArray(startIndex, endIndex int) object.Object {
	elems := make([]object.Object, endIndex-startIndex)
	for i := startIndex; i < endIndex; i++ {
		elems[i-startIndex] = vm.stack[i]
	}

	return &object.Array{Elements: elems}
}

func (vm *VM) buildHash(startIndex, endIndex int) (object.Object, error) {
	hashedPairs := make(map[object.HashKey]object.HashPair)

	for i := startIndex; i < endIndex; i += 2 {
		key := vm.stack[i]
		value := vm.stack[i+1]

		pair := object.HashPair{Key: key, Value: value}

		hashKey, ok := key.(object.Hashable)
		if !ok {
			return nil, fmt.Errorf("unusable as hash key: %s", key.Type())
		}

		hashedPairs[hashKey.HashKey()] = pair
	}

	return &object.Hash{Pairs: hashedPairs}, nil
}

func (vm *VM) executeArrayIndex(array, index object.Object) error {
	arrayObject := array.(*object.Array)
	i := index.(*object.Integer).Value
	max := int64(len(arrayObject.Elements) - 1)

	if i < 0 || i > max {
		return vm.push(Null)
	}

	return vm.push(arrayObject.Elements[i])
}

func (vm *VM) executeHashIndex(hash, index object.Object) error {
	hashObj := hash.(*object.Hash)
	key, ok := index.(object.Hashable)
	if !ok {
		return fmt.Errorf("unusable as hash key: %s", index.Type())
	}

	pair, ok := hashObj.Pairs[key.HashKey()]
	if !ok {
		return vm.push(Null)
	}

	return vm.push(pair.Value)
}

func (vm *VM) executeIndexExpression(left, index object.Object) error {
	switch {
	case left.Type() == object.ARRAY_OBJ && index.Type() == object.INTEGER_OBJ:
		return vm.executeArrayIndex(left, index)

	case left.Type() == object.HASH_OBJ:
		return vm.executeHashIndex(left, index)

	default:
		return fmt.Errorf("index operator not supported: %s", left.Type())

	}
}

// turns VM into a virtual machine
func (vm *VM) Run() error {
	var ip int
	var ins code.Instructions
	var op code.OpCode

	for vm.currentFrame().ip < len(vm.currentFrame().Instructions())-1 {
		vm.currentFrame().ip++
		ip = vm.currentFrame().ip
		ins = vm.currentFrame().Instructions()
		op = code.OpCode(ins[ip])

		switch op {
		case code.OpIndex:
			index := vm.pop()
			left := vm.pop()

			err := vm.executeIndexExpression(left, index)
			if err != nil {
				return err
			}

		case code.OpHash:
			numElemes := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2

			hash, err := vm.buildHash(vm.sp-numElemes, vm.sp)
			if err != nil {
				return err
			}

			vm.sp = vm.sp - numElemes
			err = vm.push(hash)
			if err != nil {
				return err
			}

		case code.OpConstant:
			constIndex := code.ReadUint16(ins[ip+1:])
			vm.currentFrame().ip += 2

			err := vm.push(vm.constants[constIndex])
			if err != nil {
				return err
			}

		case code.OpArray:
			numElems := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			array := vm.buildArray(vm.sp-numElems, vm.sp)
			vm.sp = vm.sp - numElems
			err := vm.push(array)
			if err != nil {
				return err
			}

		case code.OpPop:
			vm.pop()

		case code.OpJump:
			pos := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip = pos - 1

		case code.OpJumpNotTruthy:
			pos := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			condition := vm.pop()
			if !isTruthy(condition) {
				vm.currentFrame().ip = pos - 1
			}

		case code.OpSetGlobal:
			globalIndex := code.ReadUint16(ins[ip+1:])
			vm.currentFrame().ip += 2
			vm.globals[globalIndex] = vm.pop()

		case code.OpGetGlobal:
			globalIndex := code.ReadUint16(ins[ip+1:])
			vm.currentFrame().ip += 2

			err := vm.push(vm.globals[globalIndex])
			if err != nil {
				return err
			}

		case code.OpSetLocal:
			localIndex := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1
			frame := vm.currentFrame()
			vm.stack[frame.basePointer+int(localIndex)] = vm.pop()

		case code.OpGetLocal:
			localIndex := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1
			frame := vm.currentFrame()
			err := vm.push(vm.stack[frame.basePointer+int(localIndex)])
			if err != nil {
				return err
			}

		case code.OpCall:
			fn, ok := vm.stack[vm.sp-1].(*object.CompiledFunction)
			if !ok {
				return fmt.Errorf("calling non function")
			}
			frame := NewFrame(fn, vm.sp)
			vm.currentFrame().ip += 1
			vm.pushFrame(frame)
			vm.sp = frame.basePointer + fn.NumLocals

		case code.OpReturn:
			frame := vm.popFrames()
			vm.sp = frame.basePointer - 1
			err := vm.push(Null)
			if err != nil {
				return err
			}

		case code.OpReturnValue:
			rValue := vm.pop()
			frame := vm.popFrames()
			vm.sp = frame.basePointer - 1

			err := vm.push(rValue)
			if err != nil {
				return err
			}

		case code.OpTrue:
			err := vm.push(True)
			if err != nil {
				return err
			}

		case code.OpFalse:
			err := vm.push(False)
			if err != nil {
				return err
			}

		case code.OpNull:
			err := vm.push(Null)
			if err != nil {
				return err
			}

		case code.OpAdd, code.OpSub, code.OpMul, code.OpDiv:
			err := vm.executeBinaryOperation(op)
			if err != nil {
				return err
			}

		case code.OpEqual, code.OpNotEqual, code.OpGreaterThan:
			err := vm.executeComparision(op)
			if err != nil {
				return err
			}

		case code.OpBang:
			err := vm.executeBangOperator()
			if err != nil {
				return err
			}

		case code.OpMinus:
			err := vm.executeMinusOperator()
			if err != nil {
				return err
			}

		}
	}

	return nil
}

func isTruthy(obj object.Object) bool {
	switch obj := obj.(type) {
	case *object.Boolean:
		return obj.Value

	case *object.Null:
		return false

	default:
		return true
	}
}

func (vm *VM) executeMinusOperator() error {
	operand := vm.pop()

	if operand.Type() != object.INTEGER_OBJ {
		return fmt.Errorf("unsupported type for negation: %s", operand.Type())
	}

	value := operand.(*object.Integer).Value

	return vm.push(&object.Integer{Value: -value})
}

func (vm *VM) executeBangOperator() error {
	operand := vm.pop()

	switch operand {
	case True:
		return vm.push(False)

	case False:
		return vm.push(True)

	case Null:
		return vm.push(True)

	default:
		return vm.push(False)
	}
}

func (vm *VM) executeComparision(op code.OpCode) error {
	right := vm.pop()
	left := vm.pop()

	if left.Type() == object.INTEGER_OBJ && right.Type() == object.INTEGER_OBJ {
		return vm.executeIntegerComparision(op, left, right)
	}

	switch op {
	case code.OpEqual:
		return vm.push(nativeBoolToBooleanObject(right == left))

	case code.OpNotEqual:
		return vm.push(nativeBoolToBooleanObject(right != left))

	default:
		return fmt.Errorf("unknown operator: %d (%s %s)", op, left.Type(), right.Type())
	}
}

func (vm *VM) executeIntegerComparision(op code.OpCode, left, right object.Object) error {
	lValue := left.(*object.Integer).Value
	rValue := right.(*object.Integer).Value

	switch op {
	case code.OpEqual:
		return vm.push(nativeBoolToBooleanObject(rValue == lValue))
	case code.OpNotEqual:
		return vm.push(nativeBoolToBooleanObject(rValue != lValue))
	case code.OpGreaterThan:
		return vm.push(nativeBoolToBooleanObject(lValue > rValue))
	default:
		return fmt.Errorf("unknown operator: %d", op)
	}

}

func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return True
	}
	return False
}

func (vm *VM) executeBinaryOperation(op code.OpCode) error {
	right := vm.pop()
	left := vm.pop()

	leftType := left.Type()
	rightType := right.Type()

	switch {
	case leftType == object.INTEGER_OBJ && rightType == object.INTEGER_OBJ:
		return vm.executeBinaryIntegerOperation(op, left, right)

	case leftType == object.STRING_OBJ && rightType == object.STRING_OBJ:
		return vm.executeBinaryStringOperation(op, left, right)

	default:
		return fmt.Errorf("unsupported types for binary operation: %s %s", leftType, rightType)
	}
}

func (vm *VM) executeBinaryStringOperation(op code.OpCode, left, right object.Object) error {
	if op != code.OpAdd {
		return fmt.Errorf("unknown string operator: %d", op)
	}

	leftValue := left.(*object.String).Value
	rightValue := right.(*object.String).Value

	return vm.push(&object.String{Value: leftValue + rightValue})
}

func (vm *VM) executeBinaryIntegerOperation(op code.OpCode, left, right object.Object) error {
	lValue := left.(*object.Integer).Value
	rValue := right.(*object.Integer).Value

	var res int64

	switch op {
	case code.OpAdd:
		res = lValue + rValue
	case code.OpSub:
		res = lValue - rValue
	case code.OpMul:
		res = lValue * rValue
	case code.OpDiv:
		res = lValue / rValue

	default:
		return fmt.Errorf("unknowm integer operator: %d", op)
	}

	return vm.push(&object.Integer{Value: res})
}

func (vm *VM) push(o object.Object) error {
	if vm.sp >= StackSize {
		return fmt.Errorf("stack overflow")
	}

	vm.stack[vm.sp] = o
	vm.sp++

	return nil
}

func (vm *VM) pop() object.Object {
	o := vm.stack[vm.sp-1]
	vm.sp--
	return o
}
