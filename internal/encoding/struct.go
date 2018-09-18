package encoding

import (
	"fmt"
	"math"
	"reflect"

	"github.com/shamaton/msgpack/def"
	"github.com/shamaton/msgpack/internal/common"
)

type structCache struct {
	indexes []int
	names   []string
	common.Common
}

var cachemap = map[reflect.Type]*structCache{}

type structCalcFunc func(rv reflect.Value) (int, error)
type structWriteFunc func(rv reflect.Value, offset int) int

func (e *encoder) getStructCalc(typ reflect.Type) structCalcFunc {

	for j := range extCoders {
		if extCoders[j].Type() == typ {
			return extCoders[j].CalcByteSize
		}
	}
	if e.asArray {
		return e.calcStructArray
	}
	return e.calcStructMap

}

func (e *encoder) calcStruct(rv reflect.Value) (int, error) {

	//if isTime, tm := e.isDateTime(rv); isTime {
	//	size := e.calcTime(tm)
	//	return size, nil
	//}

	for i := range extCoders {
		if extCoders[i].Type() == rv.Type() {
			return extCoders[i].CalcByteSize(rv)
		}
	}

	if e.asArray {
		return e.calcStructArray(rv)
	}
	return e.calcStructMap(rv)
}

func (e *encoder) calcStructArray(rv reflect.Value) (int, error) {
	ret := 0
	t := rv.Type()
	c, find := cachemap[t]
	if !find {
		c = &structCache{}
		for i := 0; i < rv.NumField(); i++ {
			field := t.Field(i)
			if ok, name := e.CheckField(field); ok {
				size, err := e.calcSize(rv.Field(i))
				if err != nil {
					return 0, err
				}
				ret += size
				c.indexes = append(c.indexes, i)
				c.names = append(c.names, name)
			}
		}
		cachemap[t] = c
	} else {
		for i := 0; i < len(c.indexes); i++ {
			size, err := e.calcSize(rv.Field(c.indexes[i]))
			if err != nil {
				return 0, err
			}
			ret += size
		}
	}

	// format size
	l := len(c.indexes)
	if l <= 0x0f {
		// format code only
	} else if l <= math.MaxUint16 {
		ret += def.Byte2
	} else if l <= math.MaxUint32 {
		ret += def.Byte4
	} else {
		// not supported error
		return 0, fmt.Errorf("not support this array length : %d", l)
	}
	return ret, nil
}

func (e *encoder) calcStructMap(rv reflect.Value) (int, error) {
	ret := 0
	t := rv.Type()
	c, find := cachemap[t]
	if !find {
		c = &structCache{}
		for i := 0; i < rv.NumField(); i++ {
			if ok, name := e.CheckField(rv.Type().Field(i)); ok {
				keySize := def.Byte1 + e.calcString(name)
				valueSize, err := e.calcSize(rv.Field(i))
				if err != nil {
					return 0, err
				}
				ret += keySize + valueSize
				c.indexes = append(c.indexes, i)
				c.names = append(c.names, name)
			}
		}
		cachemap[t] = c
	} else {
		for i := 0; i < len(c.indexes); i++ {
			keySize := def.Byte1 + e.calcString(c.names[i])
			valueSize, err := e.calcSize(rv.Field(c.indexes[i]))
			if err != nil {
				return 0, err
			}
			ret += keySize + valueSize
		}
	}

	// format size
	l := len(c.indexes)
	if l <= 0x0f {
		// format code only
	} else if l <= math.MaxUint16 {
		ret += def.Byte2
	} else if l <= math.MaxUint32 {
		ret += def.Byte4
	} else {
		// not supported error
		return 0, fmt.Errorf("not support this array length : %d", l)
	}
	return ret, nil
}

func (e *encoder) getStructWriter(typ reflect.Type) structWriteFunc {

	for i := range extCoders {
		if extCoders[i].Type() == typ {
			return func(rv reflect.Value, offset int) int {
				return extCoders[i].WriteToBytes(rv, offset, &e.d)
			}
		}
	}

	if e.asArray {
		return e.writeStructArray
	}
	return e.writeStructMap
}

func (e *encoder) writeStruct(rv reflect.Value, offset int) int {
	/*
		if isTime, tm := e.isDateTime(rv); isTime {
			return e.writeTime(tm, offset)
		}
	*/

	for i := range extCoders {
		if extCoders[i].Type() == rv.Type() {
			return extCoders[i].WriteToBytes(rv, offset, &e.d)
		}
	}

	if e.asArray {
		return e.writeStructArray(rv, offset)
	}
	return e.writeStructMap(rv, offset)
}

func (e *encoder) writeStructArray(rv reflect.Value, offset int) int {

	c := cachemap[rv.Type()]

	// write format
	num := len(c.indexes)
	if num <= 0x0f {
		offset = e.setByte1Int(def.FixArray+num, offset)
	} else if num <= math.MaxUint16 {
		offset = e.setByte1Int(def.Array16, offset)
		offset = e.setByte2Int(num, offset)
	} else if num <= math.MaxUint32 {
		offset = e.setByte1Int(def.Array32, offset)
		offset = e.setByte4Int(num, offset)
	}

	for i := 0; i < num; i++ {
		offset = e.create(rv.Field(c.indexes[i]), offset)
	}
	return offset
}

func (e *encoder) writeStructMap(rv reflect.Value, offset int) int {

	c := cachemap[rv.Type()]

	// format size
	num := len(c.indexes)
	if num <= 0x0f {
		offset = e.setByte1Int(def.FixMap+num, offset)
	} else if num <= math.MaxUint16 {
		offset = e.setByte1Int(def.Map16, offset)
		offset = e.setByte2Int(num, offset)
	} else if num <= math.MaxUint32 {
		offset = e.setByte1Int(def.Map32, offset)
		offset = e.setByte4Int(num, offset)
	}

	for i := 0; i < num; i++ {
		offset = e.writeString(c.names[i], offset)
		offset = e.create(rv.Field(c.indexes[i]), offset)
	}
	return offset
}
