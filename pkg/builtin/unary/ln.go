package unary

import (
	"fmt"

	"github.com/matrixorigin/matrixone/pkg/builtin"
	"github.com/matrixorigin/matrixone/pkg/container/nulls"
	"github.com/matrixorigin/matrixone/pkg/container/types"
	"github.com/matrixorigin/matrixone/pkg/container/vector"
	"github.com/matrixorigin/matrixone/pkg/encoding"
	"github.com/matrixorigin/matrixone/pkg/logutil"
	"github.com/matrixorigin/matrixone/pkg/sql/colexec/extend"
	"github.com/matrixorigin/matrixone/pkg/sql/colexec/extend/overload"
	"github.com/matrixorigin/matrixone/pkg/vectorize/ln"
	"github.com/matrixorigin/matrixone/pkg/vm/process"
)

func init() {
	negative_number_error := "Invalid argument for logarithm"

	extend.FunctionRegistry["ln"] = builtin.Ln
	extend.UnaryReturnTypes[builtin.Ln] = func(extend extend.Extend) types.T {
		return types.T_float64
	}

	extend.UnaryStrings[builtin.Ln] = func(e extend.Extend) string {
		return fmt.Sprintf("ln(%s)", e)
	}

	overload.OpTypes[builtin.Ln] = overload.Unary
	overload.UnaryOps[builtin.Ln] = []*overload.UnaryOp{
		{ // T_uint8
			Typ:        types.T_uint8,
			ReturnType: types.T_float64,
			Fn: func(origVec *vector.Vector, proc *process.Process, _ bool) (*vector.Vector, error) {
				origVecCol := origVec.Col.([]uint8)
				resultVector, err := process.Get(proc, 8*int64(len(origVecCol)), types.Type{Oid: types.T_float64, Size: 8})
				if err != nil {
					return nil, err
				}
				results := encoding.DecodeFloat64Slice(resultVector.Data)
				results = results[:len(origVecCol)]
				resultVector.Col = results
				nulls.Set(resultVector.Nsp, origVec.Nsp)
				lnResult := ln.LnUint8(origVecCol, results)
				if nulls.Any(lnResult.Nsp) {
					logutil.Warn(negative_number_error)
					resultVector.Nsp.Or(lnResult.Nsp)
				}
				vector.SetCol(resultVector, results)
				return resultVector, nil
			},
		},
		{ // T_uint16
			Typ:        types.T_uint16,
			ReturnType: types.T_float64,
			Fn: func(origVec *vector.Vector, proc *process.Process, _ bool) (*vector.Vector, error) {
				origVecCol := origVec.Col.([]uint16)
				resultVector, err := process.Get(proc, 8*int64(len(origVecCol)), types.Type{Oid: types.T_float64, Size: 8})
				if err != nil {
					return nil, err
				}
				results := encoding.DecodeFloat64Slice(resultVector.Data)
				results = results[:len(origVecCol)]
				resultVector.Col = results
				nulls.Set(resultVector.Nsp, origVec.Nsp)
				lnResult := ln.LnUint16(origVecCol, results)
				if nulls.Any(lnResult.Nsp) {
					logutil.Warn(negative_number_error)
					resultVector.Nsp.Or(lnResult.Nsp)
				}
				vector.SetCol(resultVector, results)
				return resultVector, nil
			},
		},
		{ // T_uint32
			Typ:        types.T_uint32,
			ReturnType: types.T_float64,
			Fn: func(origVec *vector.Vector, proc *process.Process, _ bool) (*vector.Vector, error) {
				origVecCol := origVec.Col.([]uint32)
				resultVector, err := process.Get(proc, 8*int64(len(origVecCol)), types.Type{Oid: types.T_float64, Size: 8})
				if err != nil {
					return nil, err
				}
				results := encoding.DecodeFloat64Slice(resultVector.Data)
				results = results[:len(origVecCol)]
				resultVector.Col = results
				nulls.Set(resultVector.Nsp, origVec.Nsp)
				lnResult := ln.LnUint32(origVecCol, results)
				if nulls.Any(lnResult.Nsp) {
					logutil.Warn(negative_number_error)
					resultVector.Nsp.Or(lnResult.Nsp)
				}
				vector.SetCol(resultVector, results)
				return resultVector, nil
			},
		},
		{ // T_uint64
			Typ:        types.T_uint64,
			ReturnType: types.T_float64,
			Fn: func(origVec *vector.Vector, proc *process.Process, _ bool) (*vector.Vector, error) {
				origVecCol := origVec.Col.([]uint64)
				resultVector, err := process.Get(proc, 8*int64(len(origVecCol)), types.Type{Oid: types.T_float64, Size: 8})
				if err != nil {
					return nil, err
				}
				results := encoding.DecodeFloat64Slice(resultVector.Data)
				results = results[:len(origVecCol)]
				resultVector.Col = results
				nulls.Set(resultVector.Nsp, origVec.Nsp)
				lnResult := ln.LnUint64(origVecCol, results)
				if nulls.Any(lnResult.Nsp) {
					logutil.Warn(negative_number_error)
					resultVector.Nsp.Or(lnResult.Nsp)
				}
				vector.SetCol(resultVector, results)
				return resultVector, nil
			},
		},
		{ // T_int8
			Typ:        types.T_int8,
			ReturnType: types.T_float64,
			Fn: func(origVec *vector.Vector, proc *process.Process, _ bool) (*vector.Vector, error) {
				origVecCol := origVec.Col.([]int8)
				resultVector, err := process.Get(proc, 8*int64(len(origVecCol)), types.Type{Oid: types.T_float64, Size: 8})
				if err != nil {
					return nil, err
				}
				results := encoding.DecodeFloat64Slice(resultVector.Data)
				results = results[:len(origVecCol)]
				resultVector.Col = results
				nulls.Set(resultVector.Nsp, origVec.Nsp)
				lnResult := ln.LnInt8(origVecCol, results)
				if nulls.Any(lnResult.Nsp) {
					logutil.Warn(negative_number_error)
					resultVector.Nsp.Or(lnResult.Nsp)
				}
				vector.SetCol(resultVector, results)
				return resultVector, nil
			},
		},
		{ // T_int16
			Typ:        types.T_int16,
			ReturnType: types.T_float64,
			Fn: func(origVec *vector.Vector, proc *process.Process, _ bool) (*vector.Vector, error) {
				origVecCol := origVec.Col.([]int16)
				resultVector, err := process.Get(proc, 8*int64(len(origVecCol)), types.Type{Oid: types.T_float64, Size: 8})
				if err != nil {
					return nil, err
				}
				results := encoding.DecodeFloat64Slice(resultVector.Data)
				results = results[:len(origVecCol)]
				resultVector.Col = results
				nulls.Set(resultVector.Nsp, origVec.Nsp)
				lnResult := ln.LnInt16(origVecCol, results)
				if nulls.Any(lnResult.Nsp) {
					logutil.Warn(negative_number_error)
					resultVector.Nsp.Or(lnResult.Nsp)
				}
				vector.SetCol(resultVector, results)
				return resultVector, nil
			},
		},
		{ // T_int32
			Typ:        types.T_int32,
			ReturnType: types.T_float64,
			Fn: func(origVec *vector.Vector, proc *process.Process, _ bool) (*vector.Vector, error) {
				origVecCol := origVec.Col.([]int32)
				resultVector, err := process.Get(proc, 8*int64(len(origVecCol)), types.Type{Oid: types.T_float64, Size: 8})
				if err != nil {
					return nil, err
				}
				results := encoding.DecodeFloat64Slice(resultVector.Data)
				results = results[:len(origVecCol)]
				resultVector.Col = results
				nulls.Set(resultVector.Nsp, origVec.Nsp)
				lnResult := ln.LnInt32(origVecCol, results)
				if nulls.Any(lnResult.Nsp) {
					logutil.Warn(negative_number_error)
					resultVector.Nsp.Or(lnResult.Nsp)
				}
				vector.SetCol(resultVector, results)
				return resultVector, nil
			},
		},
		{ // T_int64
			Typ:        types.T_int64,
			ReturnType: types.T_float64,
			Fn: func(origVec *vector.Vector, proc *process.Process, _ bool) (*vector.Vector, error) {
				origVecCol := origVec.Col.([]int64)
				resultVector, err := process.Get(proc, 8*int64(len(origVecCol)), types.Type{Oid: types.T_float64, Size: 8})
				if err != nil {
					return nil, err
				}
				results := encoding.DecodeFloat64Slice(resultVector.Data)
				results = results[:len(origVecCol)]
				resultVector.Col = results
				nulls.Set(resultVector.Nsp, origVec.Nsp)
				lnResult := ln.LnInt64(origVecCol, results)
				if nulls.Any(lnResult.Nsp) {
					logutil.Warn(negative_number_error)
					resultVector.Nsp.Or(lnResult.Nsp)
				}
				vector.SetCol(resultVector, results)
				return resultVector, nil
			},
		},
		{ // T_float32
			Typ:        types.T_float32,
			ReturnType: types.T_float64,
			Fn: func(origVec *vector.Vector, proc *process.Process, _ bool) (*vector.Vector, error) {
				origVecCol := origVec.Col.([]float32)
				resultVector, err := process.Get(proc, 8*int64(len(origVecCol)), types.Type{Oid: types.T_float64, Size: 8})
				if err != nil {
					return nil, err
				}
				results := encoding.DecodeFloat64Slice(resultVector.Data)
				results = results[:len(origVecCol)]
				resultVector.Col = results
				nulls.Set(resultVector.Nsp, origVec.Nsp)
				lnResult := ln.LnFloat32(origVecCol, results)
				if nulls.Any(lnResult.Nsp) {
					logutil.Warn(negative_number_error)
					resultVector.Nsp.Or(lnResult.Nsp)
				}
				vector.SetCol(resultVector, results)
				return resultVector, nil
			},
		},
		{ // T_float64
			Typ:        types.T_float64,
			ReturnType: types.T_float64,
			Fn: func(origVec *vector.Vector, proc *process.Process, _ bool) (*vector.Vector, error) {
				origVecCol := origVec.Col.([]float64)
				resultVector, err := process.Get(proc, 8*int64(len(origVecCol)), types.Type{Oid: types.T_float64, Size: 8})
				if err != nil {
					return nil, err
				}
				results := encoding.DecodeFloat64Slice(resultVector.Data)
				results = results[:len(origVecCol)]
				resultVector.Col = results
				nulls.Set(resultVector.Nsp, origVec.Nsp)
				lnResult := ln.LnFloat64(origVecCol, results)
				if nulls.Any(lnResult.Nsp) {
					logutil.Warn(negative_number_error)
					resultVector.Nsp.Or(lnResult.Nsp)
				}
				vector.SetCol(resultVector, results)
				return resultVector, nil
			},
		},
	}
}
