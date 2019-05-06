package core

import (
	"testing"
)

func init() {

}

func TestFuzzParserTrace1(t *testing.T) {
	parser := PdfParser{}
	parser.rs, parser.reader, parser.fileSize = makeReaderForText(" /Name")

	ref := &PdfObjectReference{ObjectNumber: -1}
	obj, err := parser.Resolve(ref)

	if err != nil {
		t.Errorf("Fail, err != nil (%v)", err)
	}

	if _, isNil := obj.(*PdfObjectNull); !isNil {
		t.Errorf("Fail, obj != PdfObjectNull (%T)", obj)
	}
}

func TestFuzzSelfReference1(t *testing.T) {
	rawText := `13 0 obj
<< /Length 13 0 R >>
stream
xxx
endstream
`

	parser := PdfParser{}
	parser.xrefs.ObjectMap = make(map[int]XrefObject)
	parser.objstms = make(objectStreams)
	parser.rs, parser.reader, parser.fileSize = makeReaderForText(rawText)
	parser.streamLengthReferenceLookupInProgress = map[int64]bool{}

	parser.xrefs.ObjectMap[13] = XrefObject{
		XrefTypeTableEntry,
		13,
		0,
		0,
		0,
		0,
	}

	_, err := parser.ParseIndirectObject()
	if err == nil {
		t.Errorf("Should fail with an error")
	}
}

func TestFuzzSelfReference2(t *testing.T) {

	rawText := `13 0 obj
<< /Length 12 0 R >>
stream
xxx
endstream
`

	parser := PdfParser{}
	parser.xrefs.ObjectMap = make(map[int]XrefObject)
	parser.objstms = make(objectStreams)
	parser.rs, parser.reader, parser.fileSize = makeReaderForText(rawText)
	parser.streamLengthReferenceLookupInProgress = map[int64]bool{}

	parser.xrefs.ObjectMap[12] = XrefObject{
		XrefTypeTableEntry,
		12,
		0,
		0,
		0,
		0,
	}

	_, err := parser.ParseIndirectObject()
	if err == nil {
		t.Errorf("Should fail with an error")
	}
}

func TestFuzzIsEncryptedFail1(t *testing.T) {
	parser := PdfParser{}
	parser.rs, parser.reader, parser.fileSize = makeReaderForText(" /Name")

	ref := &PdfObjectReference{ObjectNumber: -1}

	parser.trailer = MakeDict()
	parser.trailer.Set("Encrypt", ref)

	_, err := parser.IsEncrypted()
	if err == nil {
		t.Errorf("err == nil: %v.  Should fail.", err)
		return
	}
}

func TestFuzzInvalidXrefPrev1(t *testing.T) {
	parser := PdfParser{}
	parser.rs, parser.reader, parser.fileSize = makeReaderForText(`
xref
0 1
0000000000 65535 f
0000000001 00000 n
trailer
<</Info 1 0 R/Root 2 0 R/Size 17/Prev /Invalid>>
startxref
0
%%EOF
`)

	_, err := parser.loadXrefs()
	if err != nil {
		t.Errorf("Should not error - just log a debug message regarding an invalid Prev")
		t.Errorf("Err: %v", err)
		return
	}

}
