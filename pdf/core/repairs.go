package core

import (
	"errors"
	"fmt"
	"os"
	"regexp"

	"bufio"
	"io"
	"strconv"

	"github.com/finalversus/doc/common"
)

var repairReXrefTable = regexp.MustCompile(`[\r\n]\s*(xref)\s*[\r\n]`)

func (parser *PdfParser) repairLocateXref() (int64, error) {
	readBuf := int64(1000)
	parser.rs.Seek(-readBuf, os.SEEK_CUR)

	curOffset, err := parser.rs.Seek(0, os.SEEK_CUR)
	if err != nil {
		return 0, err
	}
	b2 := make([]byte, readBuf)
	parser.rs.Read(b2)

	results := repairReXrefTable.FindAllStringIndex(string(b2), -1)
	if len(results) < 1 {
		common.Log.Debug("ERROR: Repair: xref not found!")
		return 0, errors.New("repair: xref not found")
	}

	localOffset := int64(results[len(results)-1][0])
	xrefOffset := curOffset + localOffset
	return xrefOffset, nil
}

func (parser *PdfParser) rebuildXrefTable() error {
	newXrefs := XrefTable{}
	newXrefs.ObjectMap = map[int]XrefObject{}
	for objNum, xref := range parser.xrefs.ObjectMap {
		obj, _, err := parser.lookupByNumberWrapper(objNum, false)
		if err != nil {
			common.Log.Debug("ERROR: Unable to look up object (%s)", err)
			common.Log.Debug("ERROR: Xref table completely broken - attempting to repair ")
			xrefTable, err := parser.repairRebuildXrefsTopDown()
			if err != nil {
				common.Log.Debug("ERROR: Failed xref rebuild repair (%s)", err)
				return err
			}
			parser.xrefs = *xrefTable
			common.Log.Debug("Repaired xref table built")
			return nil
		}
		actObjNum, actGenNum, err := getObjectNumber(obj)
		if err != nil {
			return err
		}

		xref.ObjectNumber = int(actObjNum)
		xref.Generation = int(actGenNum)
		newXrefs.ObjectMap[int(actObjNum)] = xref
	}

	parser.xrefs = newXrefs
	common.Log.Debug("New xref table built")
	printXrefTable(parser.xrefs)
	return nil
}

func parseObjectNumberFromString(str string) (int, int, error) {
	result := reIndirectObject.FindStringSubmatch(str)
	if len(result) < 3 {
		return 0, 0, errors.New("unable to detect indirect object signature")
	}

	on, _ := strconv.Atoi(result[1])
	gn, _ := strconv.Atoi(result[2])

	return on, gn, nil
}

func (parser *PdfParser) repairRebuildXrefsTopDown() (*XrefTable, error) {
	if parser.repairsAttempted {

		return nil, fmt.Errorf("repair failed")
	}
	parser.repairsAttempted = true

	parser.rs.Seek(0, os.SEEK_SET)
	parser.reader = bufio.NewReader(parser.rs)

	bufLen := 20
	last := make([]byte, bufLen)

	xrefTable := XrefTable{}
	xrefTable.ObjectMap = make(map[int]XrefObject)
	for {
		b, err := parser.reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return nil, err
			}
		}

		if b == 'j' && last[bufLen-1] == 'b' && last[bufLen-2] == 'o' && IsWhiteSpace(last[bufLen-3]) {
			i := bufLen - 4

			for IsWhiteSpace(last[i]) && i > 0 {
				i--
			}
			if i == 0 || !IsDecimalDigit(last[i]) {
				continue
			}

			for IsDecimalDigit(last[i]) && i > 0 {
				i--
			}
			if i == 0 || !IsWhiteSpace(last[i]) {
				continue
			}

			for IsWhiteSpace(last[i]) && i > 0 {
				i--
			}
			if i == 0 || !IsDecimalDigit(last[i]) {
				continue
			}

			for IsDecimalDigit(last[i]) && i > 0 {
				i--
			}
			if i == 0 {
				continue
			}

			objOffset := parser.GetFileOffset() - int64(bufLen-i)

			objstr := append(last[i+1:], b)
			objNum, genNum, err := parseObjectNumberFromString(string(objstr))
			if err != nil {
				common.Log.Debug("Unable to parse object number: %v", err)
				return nil, err
			}

			if curXref, has := xrefTable.ObjectMap[objNum]; !has || curXref.Generation < genNum {

				xrefEntry := XrefObject{}
				xrefEntry.XType = XrefTypeTableEntry
				xrefEntry.ObjectNumber = int(objNum)
				xrefEntry.Generation = int(genNum)
				xrefEntry.Offset = objOffset
				xrefTable.ObjectMap[objNum] = xrefEntry
			}
		}

		last = append(last[1:bufLen], b)
	}

	return &xrefTable, nil
}

func (parser *PdfParser) repairSeekXrefMarker() error {

	fSize, err := parser.rs.Seek(0, os.SEEK_END)
	if err != nil {
		return err
	}

	reXrefTableStart := regexp.MustCompile(`\sxref\s*`)

	var offset int64

	var buflen int64 = 1000

	for offset < fSize {
		if fSize <= (buflen + offset) {
			buflen = fSize - offset
		}

		_, err := parser.rs.Seek(-offset-buflen, os.SEEK_END)
		if err != nil {
			return err
		}

		b1 := make([]byte, buflen)
		parser.rs.Read(b1)

		common.Log.Trace("Looking for xref : \"%s\"", string(b1))
		ind := reXrefTableStart.FindAllStringIndex(string(b1), -1)
		if ind != nil {

			lastInd := ind[len(ind)-1]
			common.Log.Trace("Ind: % d", ind)
			parser.rs.Seek(-offset-buflen+int64(lastInd[0]), os.SEEK_END)
			parser.reader = bufio.NewReader(parser.rs)

			for {
				bb, err := parser.reader.Peek(1)
				if err != nil {
					return err
				}
				common.Log.Trace("B: %d %c", bb[0], bb[0])
				if !IsWhiteSpace(bb[0]) {
					break
				}
				parser.reader.Discard(1)
			}

			return nil
		}

		common.Log.Debug("Warning: EOF marker not found! - continue seeking")
		offset += buflen
	}

	common.Log.Debug("Error: Xref table marker was not found.")
	return errors.New("xref not found ")
}

func (parser *PdfParser) seekPdfVersionTopDown() (int, int, error) {

	parser.rs.Seek(0, os.SEEK_SET)
	parser.reader = bufio.NewReader(parser.rs)

	bufLen := 20
	last := make([]byte, bufLen)

	for {
		b, err := parser.reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return 0, 0, err
			}
		}

		if IsDecimalDigit(b) && last[bufLen-1] == '.' && IsDecimalDigit(last[bufLen-2]) && last[bufLen-3] == '-' &&
			last[bufLen-4] == 'F' && last[bufLen-5] == 'D' && last[bufLen-6] == 'P' {
			major := int(last[bufLen-2] - '0')
			minor := int(b - '0')
			return major, minor, nil
		}

		last = append(last[1:bufLen], b)
	}

	return 0, 0, errors.New("version not found")
}
