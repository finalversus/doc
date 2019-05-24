package core

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/finalversus/doc/common"
	"github.com/finalversus/doc/pdf/internal/strutils"
)

type PdfObject interface {
	String() string

	WriteString() string
}

type PdfObjectBool bool

type PdfObjectInteger int64

type PdfObjectFloat float64

type PdfObjectString struct {
	val   string
	isHex bool
}

type PdfObjectName string

type PdfObjectArray struct {
	vec []PdfObject
}

type PdfObjectDictionary struct {
	dict map[PdfObjectName]PdfObject
	keys []PdfObjectName

	parser *PdfParser
}

type PdfObjectNull struct{}

type PdfObjectReference struct {
	parser           *PdfParser
	ObjectNumber     int64
	GenerationNumber int64
}

type PdfIndirectObject struct {
	PdfObjectReference
	PdfObject
}

type PdfObjectStream struct {
	PdfObjectReference
	*PdfObjectDictionary
	Stream []byte
}

type PdfObjectStreams struct {
	PdfObjectReference
	vec []PdfObject
}

func MakeDict() *PdfObjectDictionary {
	d := &PdfObjectDictionary{}
	d.dict = map[PdfObjectName]PdfObject{}
	d.keys = []PdfObjectName{}
	return d
}

func MakeName(s string) *PdfObjectName {
	name := PdfObjectName(s)
	return &name
}

func MakeInteger(val int64) *PdfObjectInteger {
	num := PdfObjectInteger(val)
	return &num
}

func MakeBool(val bool) *PdfObjectBool {
	bval := PdfObjectBool(val)
	return &bval
}

func MakeArray(objects ...PdfObject) *PdfObjectArray {
	array := &PdfObjectArray{}
	array.vec = []PdfObject{}
	for _, obj := range objects {
		array.vec = append(array.vec, obj)
	}
	return array
}

func MakeArrayFromIntegers(vals []int) *PdfObjectArray {
	array := MakeArray()
	for _, val := range vals {
		array.Append(MakeInteger(int64(val)))
	}
	return array
}

func MakeArrayFromIntegers64(vals []int64) *PdfObjectArray {
	array := MakeArray()
	for _, val := range vals {
		array.Append(MakeInteger(val))
	}
	return array
}

func MakeArrayFromFloats(vals []float64) *PdfObjectArray {
	array := MakeArray()
	for _, val := range vals {
		array.Append(MakeFloat(val))
	}
	return array
}

func MakeFloat(val float64) *PdfObjectFloat {
	num := PdfObjectFloat(val)
	return &num
}

func MakeString(s string) *PdfObjectString {
	str := PdfObjectString{val: s}
	return &str
}

func MakeStringFromBytes(data []byte) *PdfObjectString {
	return MakeString(string(data))
}

func MakeHexString(s string) *PdfObjectString {
	str := PdfObjectString{val: s, isHex: true}
	return &str
}

func MakeEncodedString(s string, utf16BE bool) *PdfObjectString {
	if utf16BE {
		var buf bytes.Buffer
		buf.Write([]byte{0xFE, 0xFF})
		buf.WriteString(strutils.StringToUTF16(s))
		return &PdfObjectString{val: buf.String(), isHex: true}
	}

	return &PdfObjectString{val: string(strutils.StringToPDFDocEncoding(s)), isHex: false}
}

func MakeNull() *PdfObjectNull {
	null := PdfObjectNull{}
	return &null
}

func MakeIndirectObject(obj PdfObject) *PdfIndirectObject {
	ind := &PdfIndirectObject{}
	ind.PdfObject = obj
	return ind
}

func MakeStream(contents []byte, encoder StreamEncoder) (*PdfObjectStream, error) {
	stream := &PdfObjectStream{}

	if encoder == nil {
		encoder = NewRawEncoder()
	}

	stream.PdfObjectDictionary = encoder.MakeStreamDict()

	encoded, err := encoder.EncodeBytes(contents)
	if err != nil {
		return nil, err
	}
	stream.PdfObjectDictionary.Set("Length", MakeInteger(int64(len(encoded))))

	stream.Stream = encoded
	return stream, nil
}

func MakeObjectStreams(objects ...PdfObject) *PdfObjectStreams {
	streams := &PdfObjectStreams{}
	streams.vec = []PdfObject{}
	for _, obj := range objects {
		streams.vec = append(streams.vec, obj)
	}
	return streams
}

func (ref *PdfObjectReference) GetParser() *PdfParser {
	return ref.parser
}

func (ref *PdfObjectReference) Resolve() PdfObject {
	if ref.parser == nil {
		return MakeNull()
	}
	obj, _, err := ref.parser.resolveReference(ref)
	if err != nil {
		common.Log.Debug("ERROR resolving reference: %v - returning null object", err)
		return MakeNull()
	}
	if obj == nil {
		common.Log.Debug("ERROR resolving reference: nil object - returning a null object")
		return MakeNull()
	}
	return obj
}

func (bool *PdfObjectBool) String() string {
	if *bool {
		return "true"
	}
	return "false"
}

func (bool *PdfObjectBool) WriteString() string {
	if *bool {
		return "true"
	}
	return "false"
}

func (int *PdfObjectInteger) String() string {
	return fmt.Sprintf("%d", *int)
}

func (int *PdfObjectInteger) WriteString() string {
	return strconv.FormatInt(int64(*int), 10)
}

func (float *PdfObjectFloat) String() string {
	return fmt.Sprintf("%f", *float)
}

func (float *PdfObjectFloat) WriteString() string {
	return strconv.FormatFloat(float64(*float), 'f', -1, 64)
}

func (str *PdfObjectString) String() string {
	return str.val
}

func (str *PdfObjectString) Str() string {
	return str.val
}

func (str *PdfObjectString) Decoded() string {
	if str == nil {
		return ""
	}
	b := []byte(str.val)
	if len(b) >= 2 && b[0] == 0xFE && b[1] == 0xFF {

		return strutils.UTF16ToString(b[2:])
	}

	return strutils.PDFDocEncodingToString(b)
}

func (str *PdfObjectString) Bytes() []byte {
	return []byte(str.val)
}

func (str *PdfObjectString) WriteString() string {
	var output bytes.Buffer

	if str.isHex {
		shex := hex.EncodeToString(str.Bytes())
		output.WriteString("<")
		output.WriteString(shex)
		output.WriteString(">")
		return output.String()
	}

	escapeSequences := map[byte]string{
		'\n': "\\n",
		'\r': "\\r",
		'\t': "\\t",
		'\b': "\\b",
		'\f': "\\f",
		'(':  "\\(",
		')':  "\\)",
		'\\': "\\\\",
	}

	output.WriteString("(")
	for i := 0; i < len(str.val); i++ {
		char := str.val[i]
		if escStr, useEsc := escapeSequences[char]; useEsc {
			output.WriteString(escStr)
		} else {
			output.WriteByte(char)
		}
	}
	output.WriteString(")")
	return output.String()
}

func (name *PdfObjectName) String() string {
	return string(*name)
}

func (name *PdfObjectName) WriteString() string {
	var output bytes.Buffer

	if len(*name) > 127 {
		common.Log.Debug("ERROR: Name too long (%s)", *name)
	}

	output.WriteString("/")
	for i := 0; i < len(*name); i++ {
		char := (*name)[i]
		if !IsPrintable(char) || char == '#' || IsDelimiter(char) {
			output.WriteString(fmt.Sprintf("#%.2x", char))
		} else {
			output.WriteByte(char)
		}
	}

	return output.String()
}

func (array *PdfObjectArray) Elements() []PdfObject {
	if array == nil {
		return nil
	}
	return array.vec
}

func (array *PdfObjectArray) Len() int {
	if array == nil {
		return 0
	}
	return len(array.vec)
}

func (array *PdfObjectArray) Get(i int) PdfObject {
	if array == nil || i >= len(array.vec) || i < 0 {
		return nil
	}
	return array.vec[i]
}

func (array *PdfObjectArray) Set(i int, obj PdfObject) error {
	if i < 0 || i >= len(array.vec) {
		return errors.New("outside bounds")
	}
	array.vec[i] = obj
	return nil
}

func (array *PdfObjectArray) Append(objects ...PdfObject) {
	if array == nil {
		common.Log.Debug("Warn - Attempt to append to a nil array")
		return
	}
	if array.vec == nil {
		array.vec = []PdfObject{}
	}

	for _, obj := range objects {
		array.vec = append(array.vec, obj)
	}
}

func (array *PdfObjectArray) Clear() {
	array.vec = []PdfObject{}
}

func (array *PdfObjectArray) ToFloat64Array() ([]float64, error) {
	var vals []float64

	for _, obj := range array.Elements() {
		switch t := obj.(type) {
		case *PdfObjectInteger:
			vals = append(vals, float64(*t))
		case *PdfObjectFloat:
			vals = append(vals, float64(*t))
		default:
			return nil, ErrTypeError
		}
	}

	return vals, nil
}

func (array *PdfObjectArray) ToIntegerArray() ([]int, error) {
	var vals []int

	for _, obj := range array.Elements() {
		if number, is := obj.(*PdfObjectInteger); is {
			vals = append(vals, int(*number))
		} else {
			return nil, ErrTypeError
		}
	}

	return vals, nil
}

func (array *PdfObjectArray) ToInt64Slice() ([]int64, error) {
	var vals []int64

	for _, obj := range array.Elements() {
		if number, is := obj.(*PdfObjectInteger); is {
			vals = append(vals, int64(*number))
		} else {
			return nil, ErrTypeError
		}
	}

	return vals, nil
}

func (array *PdfObjectArray) String() string {
	outStr := "["
	for ind, o := range array.Elements() {
		outStr += o.String()
		if ind < (array.Len() - 1) {
			outStr += ", "
		}
	}
	outStr += "]"
	return outStr
}

func (array *PdfObjectArray) WriteString() string {
	var b strings.Builder
	b.WriteString("[")

	for ind, o := range array.Elements() {
		b.WriteString(o.WriteString())
		if ind < (array.Len() - 1) {
			b.WriteString(" ")
		}
	}

	b.WriteString("]")
	return b.String()
}

func GetNumberAsFloat(obj PdfObject) (float64, error) {
	switch t := obj.(type) {
	case *PdfObjectFloat:
		return float64(*t), nil
	case *PdfObjectInteger:
		return float64(*t), nil
	}
	return 0, ErrNotANumber
}

func IsNullObject(obj PdfObject) bool {
	_, isNull := obj.(*PdfObjectNull)
	return isNull
}

func GetNumbersAsFloat(objects []PdfObject) (floats []float64, err error) {
	for _, obj := range objects {
		val, err := GetNumberAsFloat(obj)
		if err != nil {
			return nil, err
		}
		floats = append(floats, val)
	}
	return floats, nil
}

func GetNumberAsInt64(obj PdfObject) (int64, error) {
	switch t := obj.(type) {
	case *PdfObjectFloat:
		common.Log.Debug("Number expected as integer was stored as float (type casting used)")
		return int64(*t), nil
	case *PdfObjectInteger:
		return int64(*t), nil
	}
	return 0, ErrNotANumber
}

func getNumberAsFloatOrNull(obj PdfObject) (*float64, error) {
	switch t := obj.(type) {
	case *PdfObjectFloat:
		val := float64(*t)
		return &val, nil
	case *PdfObjectInteger:
		val := float64(*t)
		return &val, nil
	case *PdfObjectNull:
		return nil, nil
	}
	return nil, ErrNotANumber
}

func (array *PdfObjectArray) GetAsFloat64Slice() ([]float64, error) {
	var slice []float64

	for _, obj := range array.Elements() {
		number, err := GetNumberAsFloat(TraceToDirectObject(obj))
		if err != nil {
			return nil, fmt.Errorf("array element not a number")
		}
		slice = append(slice, number)
	}

	return slice, nil
}

func (d *PdfObjectDictionary) Merge(another *PdfObjectDictionary) {
	if another != nil {
		for _, key := range another.Keys() {
			val := another.Get(key)
			d.Set(key, val)
		}
	}
}

func (d *PdfObjectDictionary) String() string {
	var b strings.Builder
	b.WriteString("Dict(")
	for _, k := range d.keys {
		v := d.dict[k]
		b.WriteString(`"` + k.String() + `": `)
		b.WriteString(v.String())
		b.WriteString(`, `)
	}
	b.WriteString(")")
	return b.String()
}

func (d *PdfObjectDictionary) WriteString() string {
	var b strings.Builder

	b.WriteString("<<")
	for _, k := range d.keys {
		v := d.dict[k]
		b.WriteString(k.WriteString())
		b.WriteString(" ")
		b.WriteString(v.WriteString())
	}

	b.WriteString(">>")
	return b.String()
}

func (d *PdfObjectDictionary) Set(key PdfObjectName, val PdfObject) {
	_, found := d.dict[key]
	if !found {
		d.keys = append(d.keys, key)
	}
	d.dict[key] = val
}

func (d *PdfObjectDictionary) Get(key PdfObjectName) PdfObject {
	val, has := d.dict[key]
	if !has {
		return nil
	}
	return val
}

func (d *PdfObjectDictionary) GetString(key PdfObjectName) (string, bool) {
	val, ok := d.dict[key].(*PdfObjectString)
	if !ok {
		return "", false
	}
	return val.Str(), true
}

func (d *PdfObjectDictionary) Keys() []PdfObjectName {
	if d == nil {
		return nil
	}
	return d.keys
}

func (d *PdfObjectDictionary) Clear() {
	d.keys = []PdfObjectName{}
	d.dict = map[PdfObjectName]PdfObject{}
}

func (d *PdfObjectDictionary) Remove(key PdfObjectName) {
	idx := -1
	for i, k := range d.keys {
		if k == key {
			idx = i
			break
		}
	}

	if idx >= 0 {

		d.keys = append(d.keys[:idx], d.keys[idx+1:]...)
		delete(d.dict, key)
	}
}

func (d *PdfObjectDictionary) SetIfNotNil(key PdfObjectName, val PdfObject) {
	if val != nil {
		switch t := val.(type) {
		case *PdfObjectName:
			if t != nil {
				d.Set(key, val)
			}
		case *PdfObjectDictionary:
			if t != nil {
				d.Set(key, val)
			}
		case *PdfObjectStream:
			if t != nil {
				d.Set(key, val)
			}
		case *PdfObjectString:
			if t != nil {
				d.Set(key, val)
			}
		case *PdfObjectNull:
			if t != nil {
				d.Set(key, val)
			}
		case *PdfObjectInteger:
			if t != nil {
				d.Set(key, val)
			}
		case *PdfObjectArray:
			if t != nil {
				d.Set(key, val)
			}
		case *PdfObjectBool:
			if t != nil {
				d.Set(key, val)
			}
		case *PdfObjectFloat:
			if t != nil {
				d.Set(key, val)
			}
		case *PdfObjectReference:
			if t != nil {
				d.Set(key, val)
			}
		case *PdfIndirectObject:
			if t != nil {
				d.Set(key, val)
			}
		default:
			common.Log.Error("ERROR: Unknown type: %T - should never happen!", val)
		}
	}
}

func (ref *PdfObjectReference) String() string {
	return fmt.Sprintf("Ref(%d %d)", ref.ObjectNumber, ref.GenerationNumber)
}

func (ref *PdfObjectReference) WriteString() string {
	var b strings.Builder
	b.WriteString(strconv.FormatInt(ref.ObjectNumber, 10))
	b.WriteString(" ")
	b.WriteString(strconv.FormatInt(ref.GenerationNumber, 10))
	b.WriteString(" R")
	return b.String()
}

func (ind *PdfIndirectObject) String() string {

	return fmt.Sprintf("IObject:%d", (*ind).ObjectNumber)
}

func (ind *PdfIndirectObject) WriteString() string {
	var b strings.Builder
	b.WriteString(strconv.FormatInt(ind.ObjectNumber, 10))
	b.WriteString(" 0 R")
	return b.String()
}

func (stream *PdfObjectStream) String() string {
	return fmt.Sprintf("Object stream %d: %s", stream.ObjectNumber, stream.PdfObjectDictionary)
}

func (stream *PdfObjectStream) WriteString() string {
	var b strings.Builder
	b.WriteString(strconv.FormatInt(stream.ObjectNumber, 10))
	b.WriteString(" 0 R")
	return b.String()
}

func (null *PdfObjectNull) String() string {
	return "null"
}

func (null *PdfObjectNull) WriteString() string {
	return "null"
}

const traceMaxDepth = 10

func TraceToDirectObject(obj PdfObject) PdfObject {
	if ref, isRef := obj.(*PdfObjectReference); isRef {
		obj = ref.Resolve()
	}

	iobj, isIndirectObj := obj.(*PdfIndirectObject)
	depth := 0
	for isIndirectObj {
		obj = iobj.PdfObject
		iobj, isIndirectObj = GetIndirect(obj)
		depth++
		if depth > traceMaxDepth {
			common.Log.Error("ERROR: Trace depth level beyond %d - not going deeper!", traceMaxDepth)
			return nil
		}
	}
	return obj
}

func GetBool(obj PdfObject) (bo *PdfObjectBool, found bool) {
	bo, found = TraceToDirectObject(obj).(*PdfObjectBool)
	return bo, found
}

func GetBoolVal(obj PdfObject) (b bool, found bool) {
	bo, found := TraceToDirectObject(obj).(*PdfObjectBool)
	if found {
		return bool(*bo), true
	}
	return false, false
}

func GetInt(obj PdfObject) (into *PdfObjectInteger, found bool) {
	into, found = TraceToDirectObject(obj).(*PdfObjectInteger)
	return into, found
}

func GetIntVal(obj PdfObject) (val int, found bool) {
	into, found := TraceToDirectObject(obj).(*PdfObjectInteger)
	if found && into != nil {
		return int(*into), true
	}
	return 0, false
}

func GetFloat(obj PdfObject) (fo *PdfObjectFloat, found bool) {
	fo, found = TraceToDirectObject(obj).(*PdfObjectFloat)
	return fo, found
}

func GetFloatVal(obj PdfObject) (val float64, found bool) {
	fo, found := TraceToDirectObject(obj).(*PdfObjectFloat)
	if found {
		return float64(*fo), true
	}
	return 0, false
}

func GetString(obj PdfObject) (so *PdfObjectString, found bool) {
	so, found = TraceToDirectObject(obj).(*PdfObjectString)
	return so, found
}

func GetStringVal(obj PdfObject) (val string, found bool) {
	so, found := TraceToDirectObject(obj).(*PdfObjectString)
	if found {
		return so.Str(), true
	}
	return
}

func GetStringBytes(obj PdfObject) (val []byte, found bool) {
	so, found := TraceToDirectObject(obj).(*PdfObjectString)
	if found {
		return so.Bytes(), true
	}
	return
}

func GetName(obj PdfObject) (name *PdfObjectName, found bool) {
	name, found = TraceToDirectObject(obj).(*PdfObjectName)
	return name, found
}

func GetNameVal(obj PdfObject) (val string, found bool) {
	name, found := TraceToDirectObject(obj).(*PdfObjectName)
	if found {
		return string(*name), true
	}
	return
}

func GetArray(obj PdfObject) (arr *PdfObjectArray, found bool) {
	arr, found = TraceToDirectObject(obj).(*PdfObjectArray)
	return arr, found
}

func GetDict(obj PdfObject) (dict *PdfObjectDictionary, found bool) {
	dict, found = TraceToDirectObject(obj).(*PdfObjectDictionary)
	return dict, found
}

func GetIndirect(obj PdfObject) (ind *PdfIndirectObject, found bool) {
	obj = ResolveReference(obj)
	ind, found = obj.(*PdfIndirectObject)
	return ind, found
}

func GetStream(obj PdfObject) (stream *PdfObjectStream, found bool) {
	obj = ResolveReference(obj)
	stream, found = obj.(*PdfObjectStream)
	return stream, found
}

func GetObjectStreams(obj PdfObject) (objStream *PdfObjectStreams, found bool) {
	objStream, found = obj.(*PdfObjectStreams)
	return objStream, found
}

func (streams *PdfObjectStreams) Append(objects ...PdfObject) {
	if streams == nil {
		common.Log.Debug("Warn - Attempt to append to a nil streams")
		return
	}
	if streams.vec == nil {
		streams.vec = []PdfObject{}
	}

	for _, obj := range objects {
		streams.vec = append(streams.vec, obj)
	}
}

func (streams *PdfObjectStreams) Set(i int, obj PdfObject) error {
	if i < 0 || i >= len(streams.vec) {
		return errors.New("Outside bounds")
	}
	streams.vec[i] = obj
	return nil
}

func (streams *PdfObjectStreams) Elements() []PdfObject {
	if streams == nil {
		return nil
	}
	return streams.vec
}

func (streams *PdfObjectStreams) String() string {
	return fmt.Sprintf("Object stream %d", streams.ObjectNumber)
}

func (streams *PdfObjectStreams) Len() int {
	if streams == nil {
		return 0
	}
	return len(streams.vec)
}

func (streams *PdfObjectStreams) WriteString() string {
	var b strings.Builder
	b.WriteString(strconv.FormatInt(streams.ObjectNumber, 10))
	b.WriteString(" 0 R")
	return b.String()
}
