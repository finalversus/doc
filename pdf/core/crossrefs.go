package core

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"strings"

	"github.com/codefinio/doc/common"
)

type xrefType int

const (
	XrefTypeTableEntry xrefType = iota

	XrefTypeObjectStream xrefType = iota
)

type XrefObject struct {
	XType        xrefType
	ObjectNumber int
	Generation   int

	Offset int64

	OsObjNumber int
	OsObjIndex  int
}

type XrefTable struct {
	ObjectMap map[int]XrefObject

	sortedObjects []XrefObject
}

type objectStream struct {
	N       int
	ds      []byte
	offsets map[int]int64
}

type objectStreams map[int]objectStream

type objectCache map[int]PdfObject

func (parser *PdfParser) lookupObjectViaOS(sobjNumber int, objNum int) (PdfObject, error) {
	var bufReader *bytes.Reader
	var objstm objectStream
	var cached bool

	objstm, cached = parser.objstms[sobjNumber]
	if !cached {
		soi, err := parser.LookupByNumber(sobjNumber)
		if err != nil {
			common.Log.Debug("Missing object stream with number %d", sobjNumber)
			return nil, err
		}

		so, ok := soi.(*PdfObjectStream)
		if !ok {
			return nil, errors.New("invalid object stream")
		}

		if parser.crypter != nil && !parser.crypter.isDecrypted(so) {
			return nil, errors.New("need to decrypt the stream")
		}

		sod := so.PdfObjectDictionary
		common.Log.Trace("so d: %s\n", sod.String())
		name, ok := sod.Get("Type").(*PdfObjectName)
		if !ok {
			common.Log.Debug("ERROR: Object stream should always have a Type")
			return nil, errors.New("object stream missing Type")
		}
		if strings.ToLower(string(*name)) != "objstm" {
			common.Log.Debug("ERROR: Object stream type shall always be ObjStm !")
			return nil, errors.New("object stream type != ObjStm")
		}

		N, ok := sod.Get("N").(*PdfObjectInteger)
		if !ok {
			return nil, errors.New("invalid N in stream dictionary")
		}
		firstOffset, ok := sod.Get("First").(*PdfObjectInteger)
		if !ok {
			return nil, errors.New("invalid First in stream dictionary")
		}

		common.Log.Trace("type: %s number of objects: %d", name, *N)
		ds, err := DecodeStream(so)
		if err != nil {
			return nil, err
		}

		common.Log.Trace("Decoded: %s", ds)

		bakOffset := parser.GetFileOffset()
		defer func() { parser.SetFileOffset(bakOffset) }()

		bufReader = bytes.NewReader(ds)
		parser.reader = bufio.NewReader(bufReader)

		common.Log.Trace("Parsing offset map")

		offsets := map[int]int64{}

		for i := 0; i < int(*N); i++ {
			parser.skipSpaces()

			obj, err := parser.parseNumber()
			if err != nil {
				return nil, err
			}
			onum, ok := obj.(*PdfObjectInteger)
			if !ok {
				return nil, errors.New("invalid object stream offset table")
			}

			parser.skipSpaces()

			obj, err = parser.parseNumber()
			if err != nil {
				return nil, err
			}
			offset, ok := obj.(*PdfObjectInteger)
			if !ok {
				return nil, errors.New("invalid object stream offset table")
			}

			common.Log.Trace("obj %d offset %d", *onum, *offset)
			offsets[int(*onum)] = int64(*firstOffset + *offset)
		}

		objstm = objectStream{N: int(*N), ds: ds, offsets: offsets}
		parser.objstms[sobjNumber] = objstm
	} else {

		bakOffset := parser.GetFileOffset()
		defer func() { parser.SetFileOffset(bakOffset) }()

		bufReader = bytes.NewReader(objstm.ds)

		parser.reader = bufio.NewReader(bufReader)
	}

	offset := objstm.offsets[objNum]
	common.Log.Trace("ACTUAL offset[%d] = %d", objNum, offset)

	bufReader.Seek(offset, os.SEEK_SET)
	parser.reader = bufio.NewReader(bufReader)

	bb, _ := parser.reader.Peek(100)
	common.Log.Trace("OBJ peek \"%s\"", string(bb))

	val, err := parser.parseObject()
	if err != nil {
		common.Log.Debug("ERROR Fail to read object (%s)", err)
		return nil, err
	}
	if val == nil {
		return nil, errors.New("object cannot be null")
	}

	io := PdfIndirectObject{}
	io.ObjectNumber = int64(objNum)
	io.PdfObject = val

	return &io, nil
}

func (parser *PdfParser) LookupByNumber(objNumber int) (PdfObject, error) {

	obj, _, err := parser.lookupByNumberWrapper(objNumber, true)
	return obj, err
}

func (parser *PdfParser) lookupByNumberWrapper(objNumber int, attemptRepairs bool) (PdfObject, bool, error) {
	obj, inObjStream, err := parser.lookupByNumber(objNumber, attemptRepairs)
	if err != nil {
		return nil, inObjStream, err
	}

	if !inObjStream && parser.crypter != nil && !parser.crypter.isDecrypted(obj) {
		err := parser.crypter.Decrypt(obj, 0, 0)
		if err != nil {
			return nil, inObjStream, err
		}
	}

	return obj, inObjStream, nil
}

func getObjectNumber(obj PdfObject) (int64, int64, error) {
	if io, isIndirect := obj.(*PdfIndirectObject); isIndirect {
		return io.ObjectNumber, io.GenerationNumber, nil
	}
	if so, isStream := obj.(*PdfObjectStream); isStream {
		return so.ObjectNumber, so.GenerationNumber, nil
	}
	return 0, 0, errors.New("not an indirect/stream object")
}

func (parser *PdfParser) lookupByNumber(objNumber int, attemptRepairs bool) (PdfObject, bool, error) {
	obj, ok := parser.ObjCache[objNumber]
	if ok {
		common.Log.Trace("Returning cached object %d", objNumber)
		return obj, false, nil
	}

	xref, ok := parser.xrefs.ObjectMap[objNumber]
	if !ok {

		common.Log.Trace("Unable to locate object in xrefs! - Returning null object")
		var nullObj PdfObjectNull
		return &nullObj, false, nil
	}

	common.Log.Trace("Lookup obj number %d", objNumber)
	if xref.XType == XrefTypeTableEntry {
		common.Log.Trace("xrefobj obj num %d", xref.ObjectNumber)
		common.Log.Trace("xrefobj gen %d", xref.Generation)
		common.Log.Trace("xrefobj offset %d", xref.Offset)

		parser.rs.Seek(xref.Offset, os.SEEK_SET)
		parser.reader = bufio.NewReader(parser.rs)

		obj, err := parser.ParseIndirectObject()
		if err != nil {
			common.Log.Debug("ERROR Failed reading xref (%s)", err)

			if attemptRepairs {
				common.Log.Debug("Attempting to repair xrefs (top down)")
				xrefTable, err := parser.repairRebuildXrefsTopDown()
				if err != nil {
					common.Log.Debug("ERROR Failed repair (%s)", err)
					return nil, false, err
				}
				parser.xrefs = *xrefTable
				return parser.lookupByNumber(objNumber, false)
			}
			return nil, false, err
		}

		if attemptRepairs {

			realObjNum, _, _ := getObjectNumber(obj)
			if int(realObjNum) != objNumber {
				common.Log.Debug("Invalid xrefs: Rebuilding")
				err := parser.rebuildXrefTable()
				if err != nil {
					return nil, false, err
				}

				parser.ObjCache = objectCache{}

				return parser.lookupByNumberWrapper(objNumber, false)
			}
		}

		common.Log.Trace("Returning obj")
		parser.ObjCache[objNumber] = obj
		return obj, false, nil
	} else if xref.XType == XrefTypeObjectStream {
		common.Log.Trace("xref from object stream!")
		common.Log.Trace(">Load via OS!")
		common.Log.Trace("Object stream available in object %d/%d", xref.OsObjNumber, xref.OsObjIndex)

		if xref.OsObjNumber == objNumber {
			common.Log.Debug("ERROR Circular reference!?!")
			return nil, true, errors.New("xref circular reference")
		}

		if _, exists := parser.xrefs.ObjectMap[xref.OsObjNumber]; exists {
			optr, err := parser.lookupObjectViaOS(xref.OsObjNumber, objNumber)
			if err != nil {
				common.Log.Debug("ERROR Returning ERR (%s)", err)
				return nil, true, err
			}
			common.Log.Trace("<Loaded via OS")
			parser.ObjCache[objNumber] = optr
			if parser.crypter != nil {

				parser.crypter.decryptedObjects[optr] = true
			}
			return optr, true, nil
		}

		common.Log.Debug("?? Belongs to a non-cross referenced object ...!")
		return nil, true, errors.New("os belongs to a non cross referenced object")
	}
	return nil, false, errors.New("unknown xref type")
}

func (parser *PdfParser) LookupByReference(ref PdfObjectReference) (PdfObject, error) {
	common.Log.Trace("Looking up reference %s", ref.String())
	return parser.LookupByNumber(int(ref.ObjectNumber))
}

func (parser *PdfParser) Resolve(obj PdfObject) (PdfObject, error) {
	ref, isRef := obj.(*PdfObjectReference)
	if !isRef {

		return obj, nil
	}

	bakOffset := parser.GetFileOffset()
	defer func() { parser.SetFileOffset(bakOffset) }()

	o, err := parser.LookupByReference(*ref)
	if err != nil {
		return nil, err
	}

	io, isInd := o.(*PdfIndirectObject)
	if !isInd {

		return o, nil
	}
	o = io.PdfObject
	_, isRef = o.(*PdfObjectReference)
	if isRef {
		return io, errors.New("multi depth trace pointer to pointer")
	}

	return o, nil
}

func printXrefTable(xrefTable XrefTable) {
	common.Log.Debug("=X=X=X=")
	common.Log.Debug("Xref table:")
	i := 0
	for _, xref := range xrefTable.ObjectMap {
		common.Log.Debug("i+1: %d (obj num: %d gen: %d) -> %d", i+1, xref.ObjectNumber, xref.Generation, xref.Offset)
		i++
	}
}
