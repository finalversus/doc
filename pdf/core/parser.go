

package core

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/codefinio/doc/common"
	"github.com/codefinio/doc/pdf/core/security"
)


var rePdfVersion = regexp.MustCompile(`%PDF-(\d)\.(\d)`)
var reEOF = regexp.MustCompile("%%EOF")
var reXrefTable = regexp.MustCompile(`\s*xref\s*`)
var reStartXref = regexp.MustCompile(`startx?ref\s*(\d+)`)
var reNumeric = regexp.MustCompile(`^[\+-.]*([0-9.]+)`)
var reExponential = regexp.MustCompile(`^[\+-.]*([0-9.]+)[eE][\+-.]*([0-9.]+)`)
var reReference = regexp.MustCompile(`^\s*(\d+)\s+(\d+)\s+R`)
var reIndirectObject = regexp.MustCompile(`(\d+)\s+(\d+)\s+obj`)
var reXrefSubsection = regexp.MustCompile(`(\d+)\s+(\d+)\s*$`)
var reXrefEntry = regexp.MustCompile(`(\d+)\s+(\d+)\s+([nf])\s*$`)


type PdfParser struct {
	version Version

	rs               io.ReadSeeker
	reader           *bufio.Reader
	fileSize         int64
	xrefs            XrefTable
	objstms          objectStreams
	trailer          *PdfObjectDictionary
	crypter          *PdfCrypt
	repairsAttempted bool 

	ObjCache objectCache

	
	
	
	
	streamLengthReferenceLookupInProgress map[int64]bool
}


type Version struct {
	Major int
	Minor int
}


func (v Version) String() string {
	return fmt.Sprintf("%0d.%0d", v.Major, v.Minor)
}


func (parser *PdfParser) PdfVersion() Version {
	return parser.version
}


func (parser *PdfParser) GetCrypter() *PdfCrypt {
	return parser.crypter
}


func (parser *PdfParser) IsAuthenticated() bool {
	return parser.crypter.authenticated
}



func (parser *PdfParser) GetTrailer() *PdfObjectDictionary {
	return parser.trailer
}


func (parser *PdfParser) GetXrefTable() XrefTable {
	return parser.xrefs
}


func (parser *PdfParser) skipSpaces() (int, error) {
	cnt := 0
	for {
		b, err := parser.reader.ReadByte()
		if err != nil {
			return 0, err
		}
		if IsWhiteSpace(b) {
			cnt++
		} else {
			parser.reader.UnreadByte()
			break
		}
	}

	return cnt, nil
}


func (parser *PdfParser) skipComments() error {
	if _, err := parser.skipSpaces(); err != nil {
		return err
	}

	isFirst := true
	for {
		bb, err := parser.reader.Peek(1)
		if err != nil {
			common.Log.Debug("Error %s", err.Error())
			return err
		}

		if isFirst && bb[0] != '%' {
			
			return nil
		}
		isFirst = false

		if (bb[0] != '\r') && (bb[0] != '\n') {
			parser.reader.ReadByte()
		} else {
			break
		}
	}

	
	return parser.skipComments()
}


func (parser *PdfParser) readComment() (string, error) {
	var r bytes.Buffer

	_, err := parser.skipSpaces()
	if err != nil {
		return r.String(), err
	}

	isFirst := true
	for {
		bb, err := parser.reader.Peek(1)
		if err != nil {
			common.Log.Debug("Error %s", err.Error())
			return r.String(), err
		}
		if isFirst && bb[0] != '%' {
			return r.String(), errors.New("comment should start with %")
		}
		isFirst = false

		if (bb[0] != '\r') && (bb[0] != '\n') {
			b, _ := parser.reader.ReadByte()
			r.WriteByte(b)
		} else {
			break
		}
	}
	return r.String(), nil
}


func (parser *PdfParser) readTextLine() (string, error) {
	var r bytes.Buffer
	for {
		bb, err := parser.reader.Peek(1)
		if err != nil {
			common.Log.Debug("Error %s", err.Error())
			return r.String(), err
		}
		if (bb[0] != '\r') && (bb[0] != '\n') {
			b, _ := parser.reader.ReadByte()
			r.WriteByte(b)
		} else {
			break
		}
	}
	return r.String(), nil
}


func (parser *PdfParser) parseName() (PdfObjectName, error) {
	var r bytes.Buffer
	nameStarted := false
	for {
		bb, err := parser.reader.Peek(1)
		if err == io.EOF {
			break 
		}
		if err != nil {
			return PdfObjectName(r.String()), err
		}

		if !nameStarted {
			
			if bb[0] == '/' {
				nameStarted = true
				parser.reader.ReadByte()
			} else if bb[0] == '%' {
				parser.readComment()
				parser.skipSpaces()
			} else {
				common.Log.Debug("ERROR Name starting with %s (% x)", bb, bb)
				return PdfObjectName(r.String()), fmt.Errorf("invalid name: (%c)", bb[0])
			}
		} else {
			if IsWhiteSpace(bb[0]) {
				break
			} else if (bb[0] == '/') || (bb[0] == '[') || (bb[0] == '(') || (bb[0] == ']') || (bb[0] == '<') || (bb[0] == '>') {
				break 
			} else if bb[0] == '#' {
				hexcode, err := parser.reader.Peek(3)
				if err != nil {
					return PdfObjectName(r.String()), err
				}
				parser.reader.Discard(3)

				code, err := hex.DecodeString(string(hexcode[1:3]))
				if err != nil {
					common.Log.Debug("ERROR: Invalid hex following '#', continuing using literal - Output may be incorrect")
					r.WriteByte('#') 
					continue
				}
				r.Write(code)
			} else {
				b, _ := parser.reader.ReadByte()
				r.WriteByte(b)
			}
		}
	}
	return PdfObjectName(r.String()), nil
}





















func (parser *PdfParser) parseNumber() (PdfObject, error) {
	isFloat := false
	allowSigns := true
	var r bytes.Buffer
	for {
		common.Log.Trace("Parsing number \"%s\"", r.String())
		bb, err := parser.reader.Peek(1)
		if err == io.EOF {
			
			
			
			break 
		}
		if err != nil {
			common.Log.Debug("ERROR %s", err)
			return nil, err
		}
		if allowSigns && (bb[0] == '-' || bb[0] == '+') {
			
			b, _ := parser.reader.ReadByte()
			r.WriteByte(b)
			allowSigns = false 
		} else if IsDecimalDigit(bb[0]) {
			b, _ := parser.reader.ReadByte()
			r.WriteByte(b)
		} else if bb[0] == '.' {
			b, _ := parser.reader.ReadByte()
			r.WriteByte(b)
			isFloat = true
		} else if bb[0] == 'e' || bb[0] == 'E' {
			
			b, _ := parser.reader.ReadByte()
			r.WriteByte(b)
			isFloat = true
			allowSigns = true
		} else {
			break
		}
	}

	var o PdfObject
	if isFloat {
		fVal, err := strconv.ParseFloat(r.String(), 64)
		if err != nil {
			common.Log.Debug("Error parsing number %v err=%v. Using 0.0. Output may be incorrect", r.String(), err)
			fVal = 0.0
			err = nil
		}

		objFloat := PdfObjectFloat(fVal)
		o = &objFloat
	} else {
		intVal, err := strconv.ParseInt(r.String(), 10, 64)
		if err != nil {
			common.Log.Debug("Error parsing number %v err=%v. Using 0. Output may be incorrect", r.String(), err)
			intVal = 0
			err = nil
		}

		objInt := PdfObjectInteger(intVal)
		o = &objInt
	}

	return o, nil
}


func (parser *PdfParser) parseString() (*PdfObjectString, error) {
	parser.reader.ReadByte()

	var r bytes.Buffer
	count := 1
	for {
		bb, err := parser.reader.Peek(1)
		if err != nil {
			return MakeString(r.String()), err
		}

		if bb[0] == '\\' { 
			parser.reader.ReadByte() 
			b, err := parser.reader.ReadByte()
			if err != nil {
				return MakeString(r.String()), err
			}

			
			if IsOctalDigit(b) {
				bb, err := parser.reader.Peek(2)
				if err != nil {
					return MakeString(r.String()), err
				}

				var numeric []byte
				numeric = append(numeric, b)
				for _, val := range bb {
					if IsOctalDigit(val) {
						numeric = append(numeric, val)
					} else {
						break
					}
				}
				parser.reader.Discard(len(numeric) - 1)

				common.Log.Trace("Numeric string \"%s\"", numeric)
				code, err := strconv.ParseUint(string(numeric), 8, 32)
				if err != nil {
					return MakeString(r.String()), err
				}
				r.WriteByte(byte(code))
				continue
			}

			switch b {
			case 'n':
				r.WriteRune('\n')
			case 'r':
				r.WriteRune('\r')
			case 't':
				r.WriteRune('\t')
			case 'b':
				r.WriteRune('\b')
			case 'f':
				r.WriteRune('\f')
			case '(':
				r.WriteRune('(')
			case ')':
				r.WriteRune(')')
			case '\\':
				r.WriteRune('\\')
			}

			continue
		} else if bb[0] == '(' {
			count++
		} else if bb[0] == ')' {
			count--
			if count == 0 {
				parser.reader.ReadByte()
				break
			}
		}

		b, _ := parser.reader.ReadByte()
		r.WriteByte(b)
	}

	return MakeString(r.String()), nil
}



func (parser *PdfParser) parseHexString() (*PdfObjectString, error) {
	parser.reader.ReadByte()

	var r bytes.Buffer
	for {
		bb, err := parser.reader.Peek(1)
		if err != nil {
			return MakeString(""), err
		}

		if bb[0] == '>' {
			parser.reader.ReadByte()
			break
		}

		b, _ := parser.reader.ReadByte()
		if !IsWhiteSpace(b) {
			r.WriteByte(b)
		}
	}

	if r.Len()%2 == 1 {
		r.WriteRune('0')
	}

	buf, _ := hex.DecodeString(r.String())
	return MakeHexString(string(buf)), nil
}


func (parser *PdfParser) parseArray() (*PdfObjectArray, error) {
	arr := MakeArray()

	parser.reader.ReadByte()

	for {
		parser.skipSpaces()

		bb, err := parser.reader.Peek(1)
		if err != nil {
			return arr, err
		}

		if bb[0] == ']' {
			parser.reader.ReadByte()
			break
		}

		obj, err := parser.parseObject()
		if err != nil {
			return arr, err
		}
		arr.Append(obj)
	}

	return arr, nil
}


func (parser *PdfParser) parseBool() (PdfObjectBool, error) {
	bb, err := parser.reader.Peek(4)
	if err != nil {
		return PdfObjectBool(false), err
	}
	if (len(bb) >= 4) && (string(bb[:4]) == "true") {
		parser.reader.Discard(4)
		return PdfObjectBool(true), nil
	}

	bb, err = parser.reader.Peek(5)
	if err != nil {
		return PdfObjectBool(false), err
	}
	if (len(bb) >= 5) && (string(bb[:5]) == "false") {
		parser.reader.Discard(5)
		return PdfObjectBool(false), nil
	}

	return PdfObjectBool(false), errors.New("unexpected boolean string")
}


func parseReference(refStr string) (PdfObjectReference, error) {
	objref := PdfObjectReference{}

	result := reReference.FindStringSubmatch(string(refStr))
	if len(result) < 3 {
		common.Log.Debug("Error parsing reference")
		return objref, errors.New("unable to parse reference")
	}

	objNum, _ := strconv.Atoi(result[1])
	genNum, _ := strconv.Atoi(result[2])
	objref.ObjectNumber = int64(objNum)
	objref.GenerationNumber = int64(genNum)

	return objref, nil
}


func (parser *PdfParser) parseNull() (PdfObjectNull, error) {
	_, err := parser.reader.Discard(4)
	return PdfObjectNull{}, err
}



func (parser *PdfParser) parseObject() (PdfObject, error) {
	common.Log.Trace("Read direct object")
	parser.skipSpaces()
	for {
		bb, err := parser.reader.Peek(2)
		if err != nil {
			
			if err != io.EOF || len(bb) == 0 {
				return nil, err
			}
			if len(bb) == 1 {
				
				bb = append(bb, ' ')
			}
		}

		common.Log.Trace("Peek string: %s", string(bb))
		
		if bb[0] == '/' {
			name, err := parser.parseName()
			common.Log.Trace("->Name: '%s'", name)
			return &name, err
		} else if bb[0] == '(' {
			common.Log.Trace("->String!")
			str, err := parser.parseString()
			return str, err
		} else if bb[0] == '[' {
			common.Log.Trace("->Array!")
			arr, err := parser.parseArray()
			return arr, err
		} else if (bb[0] == '<') && (bb[1] == '<') {
			common.Log.Trace("->Dict!")
			dict, err := parser.ParseDict()
			return dict, err
		} else if bb[0] == '<' {
			common.Log.Trace("->Hex string!")
			str, err := parser.parseHexString()
			return str, err
		} else if bb[0] == '%' {
			parser.readComment()
			parser.skipSpaces()
		} else {
			common.Log.Trace("->Number or ref?")
			
			
			bb, _ = parser.reader.Peek(15)
			peekStr := string(bb)
			common.Log.Trace("Peek str: %s", peekStr)

			if (len(peekStr) > 3) && (peekStr[:4] == "null") {
				null, err := parser.parseNull()
				return &null, err
			} else if (len(peekStr) > 4) && (peekStr[:5] == "false") {
				b, err := parser.parseBool()
				return &b, err
			} else if (len(peekStr) > 3) && (peekStr[:4] == "true") {
				b, err := parser.parseBool()
				return &b, err
			}

			
			result1 := reReference.FindStringSubmatch(string(peekStr))
			if len(result1) > 1 {
				bb, _ = parser.reader.ReadBytes('R')
				common.Log.Trace("-> !Ref: '%s'", string(bb[:]))
				ref, err := parseReference(string(bb))
				ref.parser = parser
				return &ref, err
			}

			result2 := reNumeric.FindStringSubmatch(string(peekStr))
			if len(result2) > 1 {
				
				common.Log.Trace("-> Number!")
				num, err := parser.parseNumber()
				return num, err
			}

			result2 = reExponential.FindStringSubmatch(string(peekStr))
			if len(result2) > 1 {
				
				common.Log.Trace("-> Exponential Number!")
				common.Log.Trace("% s", result2)
				num, err := parser.parseNumber()
				return num, err
			}

			common.Log.Debug("ERROR Unknown (peek \"%s\")", peekStr)
			return nil, errors.New("object parsing error - unexpected pattern")
		}
	}
}


func (parser *PdfParser) ParseDict() (*PdfObjectDictionary, error) {
	common.Log.Trace("Reading PDF Dict!")

	dict := MakeDict()
	dict.parser = parser

	
	c, _ := parser.reader.ReadByte()
	if c != '<' {
		return nil, errors.New("invalid dict")
	}
	c, _ = parser.reader.ReadByte()
	if c != '<' {
		return nil, errors.New("invalid dict")
	}

	for {
		parser.skipSpaces()
		parser.skipComments()

		bb, err := parser.reader.Peek(2)
		if err != nil {
			return nil, err
		}

		common.Log.Trace("Dict peek: %s (% x)!", string(bb), string(bb))
		if (bb[0] == '>') && (bb[1] == '>') {
			common.Log.Trace("EOF dictionary")
			parser.reader.ReadByte()
			parser.reader.ReadByte()
			break
		}
		common.Log.Trace("Parse the name!")

		keyName, err := parser.parseName()
		common.Log.Trace("Key: %s", keyName)
		if err != nil {
			common.Log.Debug("ERROR Returning name err %s", err)
			return nil, err
		}

		if len(keyName) > 4 && keyName[len(keyName)-4:] == "null" {
			
			
			newKey := keyName[0 : len(keyName)-4]
			common.Log.Debug("Taking care of null bug (%s)", keyName)
			common.Log.Debug("New key \"%s\" = null", newKey)
			parser.skipSpaces()
			bb, _ := parser.reader.Peek(1)
			if bb[0] == '/' {
				dict.Set(newKey, MakeNull())
				continue
			}
		}

		parser.skipSpaces()

		val, err := parser.parseObject()
		if err != nil {
			return nil, err
		}
		dict.Set(keyName, val)

		if common.Log.IsLogLevel(common.LogLevelTrace) {
			
			common.Log.Trace("dict[%s] = %s", keyName, val.String())
		}
	}
	common.Log.Trace("returning PDF Dict!")

	return dict, nil
}




func (parser *PdfParser) parsePdfVersion() (int, int, error) {
	var offset int64 = 20
	b := make([]byte, offset)
	parser.rs.Seek(0, os.SEEK_SET)
	parser.rs.Read(b)

	
	
	
	var err error
	var major, minor int

	if match := rePdfVersion.FindStringSubmatch(string(b)); len(match) < 3 {
		if major, minor, err = parser.seekPdfVersionTopDown(); err != nil {
			common.Log.Debug("Failed recovery - unable to find version")
			return 0, 0, err
		}

		
		
		
		parser.rs, err = newOffsetReader(parser.rs, parser.GetFileOffset()-8)
		if err != nil {
			return 0, 0, err
		}
	} else {
		if major, err = strconv.Atoi(match[1]); err != nil {
			return 0, 0, err
		}
		if minor, err = strconv.Atoi(match[2]); err != nil {
			return 0, 0, err
		}

		
		parser.SetFileOffset(0)
	}
	parser.reader = bufio.NewReader(parser.rs)

	common.Log.Debug("Pdf version %d.%d", major, minor)
	return major, minor, nil
}


func (parser *PdfParser) parseXrefTable() (*PdfObjectDictionary, error) {
	var trailer *PdfObjectDictionary

	txt, err := parser.readTextLine()
	if err != nil {
		return nil, err
	}

	common.Log.Trace("xref first line: %s", txt)
	curObjNum := -1
	secObjects := 0
	insideSubsection := false
	for {
		parser.skipSpaces()
		_, err := parser.reader.Peek(1)
		if err != nil {
			return nil, err
		}

		txt, err = parser.readTextLine()
		if err != nil {
			return nil, err
		}

		result1 := reXrefSubsection.FindStringSubmatch(txt)
		if len(result1) == 3 {
			
			first, _ := strconv.Atoi(result1[1])
			second, _ := strconv.Atoi(result1[2])
			curObjNum = first
			secObjects = second
			insideSubsection = true
			common.Log.Trace("xref subsection: first object: %d objects: %d", curObjNum, secObjects)
			continue
		}
		result2 := reXrefEntry.FindStringSubmatch(txt)
		if len(result2) == 4 {
			if insideSubsection == false {
				common.Log.Debug("ERROR Xref invalid format!\n")
				return nil, errors.New("xref invalid format")
			}

			first, _ := strconv.ParseInt(result2[1], 10, 64)
			gen, _ := strconv.Atoi(result2[2])
			third := result2[3]

			if strings.ToLower(third) == "n" && first > 1 {
				
				
				
				
				
				
				
				
				
				
				
				
				
				
				x, ok := parser.xrefs.ObjectMap[curObjNum]
				if !ok || gen > x.Generation {
					obj := XrefObject{ObjectNumber: curObjNum,
						XType:  XrefTypeTableEntry,
						Offset: first, Generation: gen}
					parser.xrefs.ObjectMap[curObjNum] = obj
				}
			}

			curObjNum++
			continue
		}
		if (len(txt) > 6) && (txt[:7] == "trailer") {
			common.Log.Trace("Found trailer - %s", txt)
			
			
			if len(txt) > 9 {
				offset := parser.GetFileOffset()
				parser.SetFileOffset(offset - int64(len(txt)) + 7)
			}

			parser.skipSpaces()
			parser.skipComments()
			common.Log.Trace("Reading trailer dict!")
			common.Log.Trace("peek: \"%s\"", txt)
			trailer, err = parser.ParseDict()
			common.Log.Trace("EOF reading trailer dict!")
			if err != nil {
				common.Log.Debug("Error parsing trailer dict (%s)", err)
				return nil, err
			}
			break
		}

		if txt == "%%EOF" {
			common.Log.Debug("ERROR: end of file - trailer not found - error!")
			return nil, errors.New("end of file - trailer not found")
		}

		common.Log.Trace("xref more : %s", txt)
	}
	common.Log.Trace("EOF parsing xref table!")

	return trailer, nil
}



func (parser *PdfParser) parseXrefStream(xstm *PdfObjectInteger) (*PdfObjectDictionary, error) {
	if xstm != nil {
		common.Log.Trace("XRefStm xref table object at %d", xstm)
		parser.rs.Seek(int64(*xstm), io.SeekStart)
		parser.reader = bufio.NewReader(parser.rs)
	}

	xsOffset := parser.GetFileOffset()

	xrefObj, err := parser.ParseIndirectObject()
	if err != nil {
		common.Log.Debug("ERROR: Failed to read xref object")
		return nil, errors.New("failed to read xref object")
	}

	common.Log.Trace("XRefStm object: %s", xrefObj)
	xs, ok := xrefObj.(*PdfObjectStream)
	if !ok {
		common.Log.Debug("ERROR: XRefStm pointing to non-stream object!")
		return nil, errors.New("XRefStm pointing to a non-stream object")
	}

	trailerDict := xs.PdfObjectDictionary

	sizeObj, ok := xs.PdfObjectDictionary.Get("Size").(*PdfObjectInteger)
	if !ok {
		common.Log.Debug("ERROR: Missing size from xref stm")
		return nil, errors.New("missing Size from xref stm")
	}
	
	if int64(*sizeObj) > 8388607 {
		common.Log.Debug("ERROR: xref Size exceeded limit, over 8388607 (%d)", *sizeObj)
		return nil, errors.New("range check error")
	}

	wObj := xs.PdfObjectDictionary.Get("W")
	wArr, ok := wObj.(*PdfObjectArray)
	if !ok {
		return nil, errors.New("invalid W in xref stream")
	}

	wLen := wArr.Len()
	if wLen != 3 {
		common.Log.Debug("ERROR: Unsupported xref stm (len(W) != 3 - %d)", wLen)
		return nil, errors.New("unsupported xref stm len(W) != 3")
	}

	var b []int64
	for i := 0; i < 3; i++ {
		wVal, ok := GetInt(wArr.Get(i))
		if !ok {
			return nil, errors.New("invalid w object type")
		}

		b = append(b, int64(*wVal))
	}

	ds, err := DecodeStream(xs)
	if err != nil {
		common.Log.Debug("ERROR: Unable to decode stream: %v", err)
		return nil, err
	}

	s0 := int(b[0])
	s1 := int(b[0] + b[1])
	s2 := int(b[0] + b[1] + b[2])
	deltab := int(b[0] + b[1] + b[2])

	if s0 < 0 || s1 < 0 || s2 < 0 {
		common.Log.Debug("Error s value < 0 (%d,%d,%d)", s0, s1, s2)
		return nil, errors.New("range check error")
	}
	if deltab == 0 {
		common.Log.Debug("No xref objects in stream (deltab == 0)")
		return trailerDict, nil
	}

	
	entries := len(ds) / deltab

	

	objCount := 0
	indexObj := xs.PdfObjectDictionary.Get("Index")
	
	
	
	
	
	
	
	
	
	var indexList []int
	if indexObj != nil {
		common.Log.Trace("Index: %b", indexObj)
		indicesArray, ok := indexObj.(*PdfObjectArray)
		if !ok {
			common.Log.Debug("Invalid Index object (should be an array)")
			return nil, errors.New("invalid Index object")
		}

		
		if indicesArray.Len()%2 != 0 {
			common.Log.Debug("WARNING Failure loading xref stm index not multiple of 2.")
			return nil, errors.New("range check error")
		}

		objCount = 0

		indices, err := indicesArray.ToIntegerArray()
		if err != nil {
			common.Log.Debug("Error getting index array as integers: %v", err)
			return nil, err
		}

		for i := 0; i < len(indices); i += 2 {
			

			startIdx := indices[i]
			numObjs := indices[i+1]
			for j := 0; j < numObjs; j++ {
				indexList = append(indexList, startIdx+j)
			}
			objCount += numObjs
		}
	} else {
		
		for i := 0; i < int(*sizeObj); i++ {
			indexList = append(indexList, i)
		}
		objCount = int(*sizeObj)
	}

	if entries == objCount+1 {
		
		common.Log.Debug("Incompatibility: Index missing coverage of 1 object - appending one - May lead to problems")
		maxIndex := objCount - 1
		for _, ind := range indexList {
			if ind > maxIndex {
				maxIndex = ind
			}
		}
		indexList = append(indexList, maxIndex+1)
		objCount++
	}

	if entries != len(indexList) {
		
		common.Log.Debug("ERROR: xref stm: num entries != len(indices) (%d != %d)", entries, len(indexList))
		return nil, errors.New("xref stm num entries != len(indices)")
	}

	common.Log.Trace("Objects count %d", objCount)
	common.Log.Trace("Indices: % d", indexList)

	
	convertBytes := func(v []byte) int64 {
		var tmp int64
		for i := 0; i < len(v); i++ {
			tmp += int64(v[i]) * (1 << uint(8*(len(v)-i-1)))
		}
		return tmp
	}

	common.Log.Trace("Decoded stream length: %d", len(ds))
	objIndex := 0
	for i := 0; i < len(ds); i += deltab {
		err := checkBounds(len(ds), i, i+s0)
		if err != nil {
			common.Log.Debug("Invalid slice range: %v", err)
			return nil, err
		}
		p1 := ds[i : i+s0]

		err = checkBounds(len(ds), i+s0, i+s1)
		if err != nil {
			common.Log.Debug("Invalid slice range: %v", err)
			return nil, err
		}
		p2 := ds[i+s0 : i+s1]

		err = checkBounds(len(ds), i+s1, i+s2)
		if err != nil {
			common.Log.Debug("Invalid slice range: %v", err)
			return nil, err
		}
		p3 := ds[i+s1 : i+s2]

		ftype := convertBytes(p1)
		n2 := convertBytes(p2)
		n3 := convertBytes(p3)

		if b[0] == 0 {
			
			
			ftype = 1
		}

		if objIndex >= len(indexList) {
			common.Log.Debug("XRef stream - Trying to access index out of bounds - breaking")
			break
		}
		objNum := indexList[objIndex]
		objIndex++

		common.Log.Trace("%d. p1: % x", objNum, p1)
		common.Log.Trace("%d. p2: % x", objNum, p2)
		common.Log.Trace("%d. p3: % x", objNum, p3)

		common.Log.Trace("%d. xref: %d %d %d", objNum, ftype, n2, n3)
		if ftype == 0 {
			common.Log.Trace("- Free object - can probably ignore")
		} else if ftype == 1 {
			common.Log.Trace("- In use - uncompressed via offset %b", p2)
			
			
			
			if n2 == xsOffset {
				common.Log.Debug("Updating object number for XRef table %d -> %d", objNum, xs.ObjectNumber)
				objNum = int(xs.ObjectNumber)
			}

			
			
			if xr, ok := parser.xrefs.ObjectMap[objNum]; !ok || int(n3) > xr.Generation {
				
				
				obj := XrefObject{ObjectNumber: objNum,
					XType: XrefTypeTableEntry, Offset: n2, Generation: int(n3)}
				parser.xrefs.ObjectMap[objNum] = obj
			}
		} else if ftype == 2 {
			
			common.Log.Trace("- In use - compressed object")
			if _, ok := parser.xrefs.ObjectMap[objNum]; !ok {
				obj := XrefObject{ObjectNumber: objNum,
					XType: XrefTypeObjectStream, OsObjNumber: int(n2), OsObjIndex: int(n3)}
				parser.xrefs.ObjectMap[objNum] = obj
				common.Log.Trace("entry: %+v", obj)
			}
		} else {
			common.Log.Debug("ERROR: --------INVALID TYPE XrefStm invalid?-------")
			
			
			
			
			
			
			
			continue
		}
	}

	return trailerDict, nil
}



func (parser *PdfParser) parseXref() (*PdfObjectDictionary, error) {
	
	
	
	
	const bufLen = 20
	bb, _ := parser.reader.Peek(bufLen)
	for i := 0; i < 2; i++ {
		if reIndirectObject.Match(bb) {
			common.Log.Trace("xref points to an object. Probably xref object")
			common.Log.Debug("starting with \"%s\"", string(bb))
			return parser.parseXrefStream(nil)
		}
		if reXrefTable.Match(bb) {
			common.Log.Trace("Standard xref section table!")
			return parser.parseXrefTable()
		}

		
		
		
		offset := parser.GetFileOffset()
		parser.SetFileOffset(offset - bufLen)
		defer parser.SetFileOffset(offset)

		lbb, _ := parser.reader.Peek(bufLen)
		bb = append(lbb, bb...)
	}

	common.Log.Debug("Warning: Unable to find xref table or stream. Repair attempted: Looking for earliest xref from bottom.")
	if err := parser.repairSeekXrefMarker(); err != nil {
		common.Log.Debug("Repair failed - %v", err)
		return nil, err
	}
	return parser.parseXrefTable()
}



func (parser *PdfParser) seekToEOFMarker(fSize int64) error {
	
	var offset int64

	
	var buflen int64 = 1000

	for offset < fSize {
		if fSize <= (buflen + offset) {
			buflen = fSize - offset
		}

		
		_, err := parser.rs.Seek(-offset-buflen, io.SeekEnd)
		if err != nil {
			return err
		}

		
		b1 := make([]byte, buflen)
		parser.rs.Read(b1)
		common.Log.Trace("Looking for EOF marker: \"%s\"", string(b1))
		ind := reEOF.FindAllStringIndex(string(b1), -1)
		if ind != nil {
			
			lastInd := ind[len(ind)-1]
			common.Log.Trace("Ind: % d", ind)
			parser.rs.Seek(-offset-buflen+int64(lastInd[0]), io.SeekEnd)
			return nil
		}

		common.Log.Debug("Warning: EOF marker not found! - continue seeking")
		offset += buflen
	}

	common.Log.Debug("Error: EOF marker was not found.")
	return errors.New("EOF not found")
}




















func (parser *PdfParser) loadXrefs() (*PdfObjectDictionary, error) {
	parser.xrefs.ObjectMap = make(map[int]XrefObject)
	parser.objstms = make(objectStreams)

	
	fSize, err := parser.rs.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}
	common.Log.Trace("fsize: %d", fSize)
	parser.fileSize = fSize

	
	err = parser.seekToEOFMarker(fSize)
	if err != nil {
		common.Log.Debug("Failed seek to eof marker: %v", err)
		return nil, err
	}

	
	curOffset, err := parser.rs.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}

	
	var numBytes int64 = 64
	offset := curOffset - numBytes
	if offset < 0 {
		offset = 0
	}
	_, err = parser.rs.Seek(offset, io.SeekStart)
	if err != nil {
		return nil, err
	}

	b2 := make([]byte, numBytes)
	_, err = parser.rs.Read(b2)
	if err != nil {
		common.Log.Debug("Failed reading while looking for startxref: %v", err)
		return nil, err
	}

	result := reStartXref.FindStringSubmatch(string(b2))
	if len(result) < 2 {
		common.Log.Debug("Error: startxref not found!")
		return nil, errors.New("startxref not found")
	}
	if len(result) > 2 {
		common.Log.Debug("ERROR: Multiple startxref (%s)!", b2)
		return nil, errors.New("multiple startxref entries?")
	}
	offsetXref, _ := strconv.ParseInt(result[1], 10, 64)
	common.Log.Trace("startxref at %d", offsetXref)

	if offsetXref > fSize {
		common.Log.Debug("ERROR: Xref offset outside of file")
		common.Log.Debug("Attempting repair")
		offsetXref, err = parser.repairLocateXref()
		if err != nil {
			common.Log.Debug("ERROR: Repair attempt failed (%s)")
			return nil, err
		}
	}
	
	parser.rs.Seek(int64(offsetXref), io.SeekStart)
	parser.reader = bufio.NewReader(parser.rs)

	trailerDict, err := parser.parseXref()
	if err != nil {
		return nil, err
	}

	
	xx := trailerDict.Get("XRefStm")
	if xx != nil {
		xo, ok := xx.(*PdfObjectInteger)
		if !ok {
			return nil, errors.New("XRefStm != int")
		}
		_, err = parser.parseXrefStream(xo)
		if err != nil {
			return nil, err
		}
	}

	
	var prevList []int64
	intInSlice := func(val int64, list []int64) bool {
		for _, b := range list {
			if b == val {
				return true
			}
		}
		return false
	}

	
	
	xx = trailerDict.Get("Prev")
	for xx != nil {
		prevInt, ok := xx.(*PdfObjectInteger)
		if !ok {
			
			
			common.Log.Debug("Invalid Prev reference: Not a *PdfObjectInteger (%T)", xx)
			return trailerDict, nil
		}

		off := *prevInt
		common.Log.Trace("Another Prev xref table object at %d", off)

		
		parser.rs.Seek(int64(off), os.SEEK_SET)
		parser.reader = bufio.NewReader(parser.rs)

		ptrailerDict, err := parser.parseXref()
		if err != nil {
			common.Log.Debug("Warning: Error - Failed loading another (Prev) trailer")
			common.Log.Debug("Attempting to continue by ignoring it")
			break
		}

		xx = ptrailerDict.Get("Prev")
		if xx != nil {
			prevoff := *(xx.(*PdfObjectInteger))
			if intInSlice(int64(prevoff), prevList) {
				
				common.Log.Debug("Preventing circular xref referencing")
				break
			}
			prevList = append(prevList, int64(prevoff))
		}
	}

	return trailerDict, nil
}


func (parser *PdfParser) xrefNextObjectOffset(offset int64) int64 {
	nextOffset := int64(0)

	if len(parser.xrefs.ObjectMap) == 0 {
		return 0
	}

	if len(parser.xrefs.sortedObjects) == 0 {
		count := 0
		for _, xref := range parser.xrefs.ObjectMap {
			if xref.Offset > 0 {
				count++
			}
		}
		if count == 0 {
			
			return 0
		}
		parser.xrefs.sortedObjects = make([]XrefObject, count)

		i := 0
		for _, xref := range parser.xrefs.ObjectMap {
			if xref.Offset > 0 {
				parser.xrefs.sortedObjects[i] = xref
				i++
			}
		}

		
		sort.Slice(parser.xrefs.sortedObjects, func(i, j int) bool {
			return parser.xrefs.sortedObjects[i].Offset < parser.xrefs.sortedObjects[j].Offset
		})
	}

	i := sort.Search(len(parser.xrefs.sortedObjects), func(i int) bool {
		return parser.xrefs.sortedObjects[i].Offset >= offset
	})
	if i < len(parser.xrefs.sortedObjects) {
		nextOffset = parser.xrefs.sortedObjects[i].Offset
	}

	return nextOffset
}



func (parser *PdfParser) traceStreamLength(lengthObj PdfObject) (PdfObject, error) {
	lengthRef, isRef := lengthObj.(*PdfObjectReference)
	if isRef {
		lookupInProgress, has := parser.streamLengthReferenceLookupInProgress[lengthRef.ObjectNumber]
		if has && lookupInProgress {
			common.Log.Debug("Stream Length reference unresolved (illegal)")
			return nil, errors.New("illegal recursive loop")
		}
		
		parser.streamLengthReferenceLookupInProgress[lengthRef.ObjectNumber] = true
	}

	slo, err := parser.Resolve(lengthObj)
	if err != nil {
		return nil, err
	}
	common.Log.Trace("Stream length? %s", slo)

	if isRef {
		
		parser.streamLengthReferenceLookupInProgress[lengthRef.ObjectNumber] = false
	}

	return slo, nil
}



func (parser *PdfParser) ParseIndirectObject() (PdfObject, error) {
	indirect := PdfIndirectObject{}
	indirect.parser = parser
	common.Log.Trace("-Read indirect obj")
	bb, err := parser.reader.Peek(20)
	if err != nil {
		if err != io.EOF {
			common.Log.Debug("ERROR: Fail to read indirect obj")
			return &indirect, err
		}
	}
	common.Log.Trace("(indirect obj peek \"%s\"", string(bb))

	indices := reIndirectObject.FindStringSubmatchIndex(string(bb))
	if len(indices) < 6 {
		if err == io.EOF {
			
			
			return nil, err
		}
		common.Log.Debug("ERROR: Unable to find object signature (%s)", string(bb))
		return &indirect, errors.New("unable to detect indirect object signature")
	}
	parser.reader.Discard(indices[0]) 
	common.Log.Trace("Offsets % d", indices)

	
	hlen := indices[1] - indices[0]
	hb := make([]byte, hlen)
	_, err = parser.ReadAtLeast(hb, hlen)
	if err != nil {
		common.Log.Debug("ERROR: unable to read - %s", err)
		return nil, err
	}
	common.Log.Trace("textline: %s", hb)

	result := reIndirectObject.FindStringSubmatch(string(hb))
	if len(result) < 3 {
		common.Log.Debug("ERROR: Unable to find object signature (%s)", string(hb))
		return &indirect, errors.New("unable to detect indirect object signature")
	}

	on, _ := strconv.Atoi(result[1])
	gn, _ := strconv.Atoi(result[2])
	indirect.ObjectNumber = int64(on)
	indirect.GenerationNumber = int64(gn)

	for {
		bb, err := parser.reader.Peek(2)
		if err != nil {
			return &indirect, err
		}
		common.Log.Trace("Ind. peek: %s (% x)!", string(bb), string(bb))

		if IsWhiteSpace(bb[0]) {
			parser.skipSpaces()
		} else if bb[0] == '%' {
			parser.skipComments()
		} else if (bb[0] == '<') && (bb[1] == '<') {
			common.Log.Trace("Call ParseDict")
			indirect.PdfObject, err = parser.ParseDict()
			common.Log.Trace("EOF Call ParseDict: %v", err)
			if err != nil {
				return &indirect, err
			}
			common.Log.Trace("Parsed dictionary... finished.")
		} else if (bb[0] == '/') || (bb[0] == '(') || (bb[0] == '[') || (bb[0] == '<') {
			indirect.PdfObject, err = parser.parseObject()
			if err != nil {
				return &indirect, err
			}
			common.Log.Trace("Parsed object ... finished.")
		} else {
			if bb[0] == 'e' {
				lineStr, err := parser.readTextLine()
				if err != nil {
					return nil, err
				}
				if len(lineStr) >= 6 && lineStr[0:6] == "endobj" {
					break
				}
			} else if bb[0] == 's' {
				bb, _ = parser.reader.Peek(10)
				if string(bb[:6]) == "stream" {
					discardBytes := 6
					if len(bb) > 6 {
						if IsWhiteSpace(bb[discardBytes]) && bb[discardBytes] != '\r' && bb[discardBytes] != '\n' {
							
							
							common.Log.Debug("Non-conformant PDF not ending stream line properly with EOL marker")
							discardBytes++
						}
						if bb[discardBytes] == '\r' {
							discardBytes++
							if bb[discardBytes] == '\n' {
								discardBytes++
							}
						} else if bb[discardBytes] == '\n' {
							discardBytes++
						}
					}

					parser.reader.Discard(discardBytes)

					dict, isDict := indirect.PdfObject.(*PdfObjectDictionary)
					if !isDict {
						return nil, errors.New("stream object missing dictionary")
					}
					common.Log.Trace("Stream dict %s", dict)

					
					slo, err := parser.traceStreamLength(dict.Get("Length"))
					if err != nil {
						common.Log.Debug("Fail to trace stream length: %v", err)
						return nil, err
					}
					common.Log.Trace("Stream length? %s", slo)

					pstreamLength, ok := slo.(*PdfObjectInteger)
					if !ok {
						return nil, errors.New("stream length needs to be an integer")
					}
					streamLength := *pstreamLength
					if streamLength < 0 {
						return nil, errors.New("stream needs to be longer than 0")
					}

					
					
					
					streamStartOffset := parser.GetFileOffset()
					nextObjectOffset := parser.xrefNextObjectOffset(streamStartOffset)
					if streamStartOffset+int64(streamLength) > nextObjectOffset && nextObjectOffset > streamStartOffset {
						common.Log.Debug("Expected ending at %d", streamStartOffset+int64(streamLength))
						common.Log.Debug("Next object starting at %d", nextObjectOffset)
						
						newLength := nextObjectOffset - streamStartOffset - 17
						if newLength < 0 {
							return nil, errors.New("invalid stream length, going past boundaries")
						}

						common.Log.Debug("Attempting a length correction to %d...", newLength)
						streamLength = PdfObjectInteger(newLength)
						dict.Set("Length", MakeInteger(newLength))
					}

					
					if int64(streamLength) > parser.fileSize {
						common.Log.Debug("ERROR: Stream length cannot be larger than file size")
						return nil, errors.New("invalid stream length, larger than file size")
					}

					stream := make([]byte, streamLength)
					_, err = parser.ReadAtLeast(stream, int(streamLength))
					if err != nil {
						common.Log.Debug("ERROR stream (%d): %X", len(stream), stream)
						common.Log.Debug("ERROR: %v", err)
						return nil, err
					}

					streamobj := PdfObjectStream{}
					streamobj.Stream = stream
					streamobj.PdfObjectDictionary = indirect.PdfObject.(*PdfObjectDictionary)
					streamobj.ObjectNumber = indirect.ObjectNumber
					streamobj.GenerationNumber = indirect.GenerationNumber
					streamobj.PdfObjectReference.parser = parser

					parser.skipSpaces()
					parser.reader.Discard(9) 
					parser.skipSpaces()
					return &streamobj, nil
				}
			}

			indirect.PdfObject, err = parser.parseObject()
			if indirect.PdfObject == nil {
				common.Log.Debug("INCOMPATIBILITY: Indirect object not containing an object - assuming null object")
				indirect.PdfObject = MakeNull()
			}
			return &indirect, err
		}
	}
	if indirect.PdfObject == nil {
		common.Log.Debug("INCOMPATIBILITY: Indirect object not containing an object - assuming null object")
		indirect.PdfObject = MakeNull()
	}
	common.Log.Trace("Returning indirect!")
	return &indirect, nil
}


func NewParserFromString(txt string) *PdfParser {
	bufReader := bytes.NewReader([]byte(txt))

	parser := &PdfParser{
		ObjCache:                              objectCache{},
		rs:                                    bufReader,
		reader:                                bufio.NewReader(bufReader),
		fileSize:                              int64(len(txt)),
		streamLengthReferenceLookupInProgress: map[int64]bool{},
	}
	parser.xrefs.ObjectMap = make(map[int]XrefObject)

	return parser
}



func NewParser(rs io.ReadSeeker) (*PdfParser, error) {
	parser := &PdfParser{
		rs:                                    rs,
		ObjCache:                              make(objectCache),
		streamLengthReferenceLookupInProgress: map[int64]bool{},
	}

	
	majorVersion, minorVersion, err := parser.parsePdfVersion()
	if err != nil {
		common.Log.Error("Unable to parse version: %v", err)
		return nil, err
	}
	parser.version.Major = majorVersion
	parser.version.Minor = minorVersion

	
	if parser.trailer, err = parser.loadXrefs(); err != nil {
		common.Log.Debug("ERROR: Failed to load xref table! %s", err)
		return nil, err
	}
	common.Log.Trace("Trailer: %s", parser.trailer)

	if len(parser.xrefs.ObjectMap) == 0 {
		return nil, fmt.Errorf("empty XREF table - Invalid")
	}

	return parser, nil
}


func (parser *PdfParser) resolveReference(ref *PdfObjectReference) (PdfObject, bool, error) {
	cachedObj, isCached := parser.ObjCache[int(ref.ObjectNumber)]
	if isCached {
		return cachedObj, true, nil
	}
	obj, err := parser.LookupByReference(*ref)
	if err != nil {
		return nil, false, err
	}
	parser.ObjCache[int(ref.ObjectNumber)] = obj
	return obj, false, nil
}





func (parser *PdfParser) IsEncrypted() (bool, error) {
	if parser.crypter != nil {
		return true, nil
	} else if parser.trailer == nil {
		return false, nil
	}

	common.Log.Trace("Checking encryption dictionary!")
	e := parser.trailer.Get("Encrypt")
	if e == nil {
		return false, nil
	}
	common.Log.Trace("Is encrypted!")
	var (
		dict *PdfObjectDictionary
	)
	switch e := e.(type) {
	case *PdfObjectDictionary:
		dict = e
	case *PdfObjectReference:
		common.Log.Trace("0: Look up ref %q", e)
		encObj, err := parser.LookupByReference(*e)
		common.Log.Trace("1: %q", encObj)
		if err != nil {
			return false, err
		}

		encIndObj, ok := encObj.(*PdfIndirectObject)
		if !ok {
			common.Log.Debug("Encryption object not an indirect object")
			return false, errors.New("type check error")
		}
		encDict, ok := encIndObj.PdfObject.(*PdfObjectDictionary)

		common.Log.Trace("2: %q", encDict)
		if !ok {
			return false, errors.New("trailer Encrypt object non dictionary")
		}
		dict = encDict
	default:
		return false, fmt.Errorf("unsupported type: %T", e)
	}

	crypter, err := PdfCryptNewDecrypt(parser, dict, parser.trailer)
	if err != nil {
		return false, err
	}
	
	for _, key := range []string{"Info", "Encrypt"} {
		f := parser.trailer.Get(PdfObjectName(key))
		if f == nil {
			continue
		}
		switch f := f.(type) {
		case *PdfObjectReference:
			crypter.decryptedObjNum[int(f.ObjectNumber)] = struct{}{}
		case *PdfIndirectObject:
			crypter.decryptedObjects[f] = true
			crypter.decryptedObjNum[int(f.ObjectNumber)] = struct{}{}
		}
	}
	parser.crypter = crypter
	common.Log.Trace("Crypter object %b", crypter)
	return true, nil
}




func (parser *PdfParser) Decrypt(password []byte) (bool, error) {
	
	if parser.crypter == nil {
		return false, errors.New("check encryption first")
	}

	authenticated, err := parser.crypter.authenticate(password)
	if err != nil {
		return false, err
	}

	if !authenticated {
		
		authenticated, err = parser.crypter.authenticate([]byte(""))
	}

	return authenticated, err
}







func (parser *PdfParser) CheckAccessRights(password []byte) (bool, security.Permissions, error) {
	
	if parser.crypter == nil {
		
		return true, security.PermOwner, nil
	}
	return parser.crypter.checkAccessRights(password)
}
