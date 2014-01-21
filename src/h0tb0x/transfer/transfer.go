package transfer

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"reflect"
)

//  0aaaaaaa -> 7 bit unsigned number aaaaaaa
//  1aaaaaaa 0bbbbbbb -> 14 bit unsigned number  bbbbbbbaaaaaaa

func writePanic(out io.Writer, data []byte) {
	_, err := out.Write(data)
	if err != nil {
		panic(err)
	}
}

func readPanic(in io.Reader, data []byte) {
	_, err := io.ReadFull(in, data)
	if err != nil {
		panic(err)
	}
}

func readBytes(in io.Reader, size int) (out []byte) {
	out = make([]byte, size, size)
	readPanic(in, out)
	return
}

func writeByte(out io.Writer, b byte) {
	writePanic(out, []byte{b})
}

func readByte(in io.Reader) byte {
	return readBytes(in, 1)[0]
}

func writeUint(out io.Writer, v uint64) {
	for v > 0x7f {
		writeByte(out, byte(v&0x7f|0x80))
		v >>= 7
	}
	writeByte(out, byte(v))
}

func readUint(in io.Reader) (v uint64) {
	offset := uint(0)
	c := readByte(in)
	for (c & 0x80) != 0 {
		v |= (uint64(c&0x7f) << offset)
		offset += 7
		c = readByte(in)
	}
	v |= (uint64(c) << offset)
	return
}

// Some tricky stuff to keep the full range of int64 intact
// Basically I'm moving the sign bit to the *end* of the integer

func writeInt(out io.Writer, s int64) {
	if s < 0 {
		writeUint(out, 2*(uint64(-s)-1)+1)
	} else {
		writeUint(out, 2*(uint64(s)))
	}
}

func readInt(in io.Reader) int64 {
	v := readUint(in)
	if v&1 == 0 {
		return int64(v / 2)
	} else {
		return -int64(v/2) - 1
	}
}

func callSpecial(f reflect.Value, obj reflect.Value, io reflect.Value) {
	t := f.Type()
	if t.NumIn() != 2 {
		panic(fmt.Errorf("Encode/Decode function has wrong number of params"))
	}
	if t.NumOut() != 1 {
		panic(fmt.Errorf("Encode/Decode function has wrong number of outputs"))
	}
	if t.In(0) != obj.Type() {
		panic(fmt.Errorf("Internal inconsistency"))
	}
	if !io.Type().AssignableTo(t.In(1)) {
		panic(fmt.Errorf("Encode/Decode function must take io.Reader/Writer, got %s", t.In(1)))
	}
	var errptr *error
	if t.Out(0) != reflect.TypeOf(errptr).Elem() {
		panic(fmt.Errorf("Encode/Decode function must return error, returned %s", t.Out(0)))
	}
	outs := f.Call([]reflect.Value{obj, io})
	err, _ := outs[0].Interface().(error)
	if err != nil {
		panic(err)
	}
}

func writeStruct(out io.Writer, obj reflect.Value) {
	for i := 0; i < obj.Type().NumField(); i++ {
		if obj.Type().Field(i).PkgPath == "" {
			writeAny(out, obj.Field(i))
		}
	}
}

func readStruct(in io.Reader, obj reflect.Value) {
	for i := 0; i < obj.Type().NumField(); i++ {
		if obj.Type().Field(i).PkgPath == "" {
			readAny(in, obj.Field(i))
		}
	}
}

func writeArray(out io.Writer, obj reflect.Value) {
	for i := 0; i < obj.Len(); i++ {
		writeAny(out, obj.Index(i))
	}
}

func readArray(in io.Reader, obj reflect.Value) {
	for i := 0; i < obj.Len(); i++ {
		readAny(in, obj.Index(i))
	}
}

func writeMap(out io.Writer, obj reflect.Value) {
	keys := obj.MapKeys()
	writeUint(out, uint64(len(keys)))
	for i := 0; i < len(keys); i++ {
		writeAny(out, keys[i])
		writeAny(out, obj.MapIndex(keys[i]))
	}
}

func readMap(in io.Reader, obj reflect.Value) {
	len := int(readUint(in))
	key := reflect.New(obj.Type().Key())
	value := reflect.New(obj.Type().Elem())
	obj.Set(reflect.MakeMap(obj.Type()))
	for i := 0; i < len; i++ {
		readAny(in, key)
		readAny(in, value)
		obj.SetMapIndex(key.Elem(), value.Elem())
	}
}

func writeSlice(out io.Writer, obj reflect.Value) {
	writeUint(out, uint64(obj.Len()))
	for i := 0; i < obj.Len(); i++ {
		writeAny(out, obj.Index(i))
	}
}

func readSlice(in io.Reader, obj reflect.Value) {
	len := int(readUint(in))
	obj.Set(reflect.MakeSlice(obj.Type(), len, len))
	for i := 0; i < len; i++ {
		readAny(in, obj.Index(i))
	}
}

func writeString(out io.Writer, obj reflect.Value) {
	flat := []byte(obj.String())
	writeUint(out, uint64(len(flat)))
	writePanic(out, flat)
}

func readString(in io.Reader, obj reflect.Value) {
	len := int(readUint(in))
	obj.SetString(string(readBytes(in, len)))
}

func writeAny(out io.Writer, obj reflect.Value) {
	//fmt.Printf("Doing write of %s\n", obj.Type())
	meth_val, has_spec_val := obj.Type().MethodByName("Encode")
	if has_spec_val {
		//fmt.Printf("GOT BY VALUE\n")
		callSpecial(meth_val.Func, obj, reflect.ValueOf(out))
		return
	}
	meth, has_spec := reflect.PtrTo(obj.Type()).MethodByName("Encode")
	if has_spec {
		callSpecial(meth.Func, obj.Addr(), reflect.ValueOf(out))
		return
	}
	switch obj.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		writeInt(out, obj.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		writeUint(out, obj.Uint())
	case reflect.Bool:
		// Seriously, no ternary?
		var lameo uint64
		if obj.Bool() {
			lameo = 1
		} else {
			lameo = 0
		}
		writeUint(out, lameo)
	case reflect.Ptr:
		writeAny(out, obj.Elem())
	case reflect.Struct:
		writeStruct(out, obj)
	case reflect.String:
		writeString(out, obj)
	case reflect.Array:
		writeArray(out, obj)
	case reflect.Map:
		writeMap(out, obj)
	case reflect.Slice:
		writeSlice(out, obj)
	default:
		panic(fmt.Errorf("Unable to write value of type: %s", obj.Type()))
	}
}

func readAny(in io.Reader, obj reflect.Value) {
	//fmt.Printf("Doing read of %s\n", obj.Type())
	if !obj.CanSet() {
		if obj.Kind() == reflect.Ptr {
			readAny(in, obj.Elem())
			return
		} else {
			panic(fmt.Errorf("Cant decode into an unsettable object"))
		}
	}
	meth, has_spec := reflect.PtrTo(obj.Type()).MethodByName("Decode")
	if has_spec {
		callSpecial(meth.Func, obj.Addr(), reflect.ValueOf(in))
		return
	}
	switch obj.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		obj.SetInt(readInt(in))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		obj.SetUint(readUint(in))
	case reflect.Bool:
		obj.SetBool(readUint(in) != 0)
	case reflect.Ptr:
		obj.Set(reflect.New(obj.Type().Elem()))
		readAny(in, obj.Elem())
	case reflect.Struct:
		readStruct(in, obj)
	case reflect.Array:
		readArray(in, obj)
	case reflect.Map:
		readMap(in, obj)
	case reflect.Slice:
		readSlice(in, obj)
	case reflect.String:
		readString(in, obj)
	default:
		panic(fmt.Errorf("Unable to read value of type: %s", obj.Type()))
	}
}

// Encodes an object or objects to a byte stream.
func Encode(out io.Writer, objs ...interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()
	for i := 0; i < len(objs); i++ {
		writeAny(out, reflect.ValueOf(objs[i]))
	}
	err = nil
	return
}

// Decodes an object or object from a byte stream.
func Decode(in io.Reader, objs ...interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()
	for i := 0; i < len(objs); i++ {
		readAny(in, reflect.ValueOf(objs[i]))
	}
	err = nil
	return
}

// Encodes an object or objects into a byte array
func EncodeBytes(objs ...interface{}) (out []byte, err error) {
	var buf bytes.Buffer
	err = Encode(&buf, objs...)
	if err != nil {
		return
	}
	out = buf.Bytes()
	return
}

// Decords an object or objects from a byte array
func DecodeBytes(in []byte, objs ...interface{}) (err error) {
	buf := bytes.NewBuffer(in)
	err = Decode(buf, objs...)
	return
}

// Encodes an object or objects as base64 string.
func EncodeString(objs ...interface{}) (out string, err error) {
	bbuf, err := EncodeBytes(objs...)
	if err != nil {
		return
	}
	out = base64.URLEncoding.EncodeToString(bbuf)
	return
}

// Decords an object or objects from a base64 string.
func DecodeString(in string, objs ...interface{}) (err error) {
	bbuf, err := base64.URLEncoding.DecodeString(in)
	if err != nil {
		return
	}
	err = DecodeBytes(bbuf, objs...)
	return
}

// Encodes an object or objects as byte slice, and panics on error.
// Useful for inline usage when failure to encode is an assert.
func AsBytes(objs ...interface{}) []byte {
	bbuf, err := EncodeBytes(objs...)
	if err != nil {
		panic(err)
	}
	return bbuf
}

// Encodes an object or objects as base64 string, and panics on error.
// Useful for inline usage when failure to encode is an assert.
func AsString(objs ...interface{}) string {
	str, err := EncodeString(objs...)
	if err != nil {
		panic(err)
	}
	return str
}
