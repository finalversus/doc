package core

import (
	"encoding/base64"
	"testing"

	"github.com/finalversus/doc/common"
)

func init() {
	common.SetLogger(common.ConsoleLogger{})
}

func TestFlateEncodingPredictor1(t *testing.T) {
	rawStream := []byte("this is a dummy text with some \x01\x02\x03 binary data")

	encoder := NewFlateEncoder()
	encoder.Predictor = 1

	encoded, err := encoder.EncodeBytes(rawStream)
	if err != nil {
		t.Errorf("Failed to encode data: %v", err)
		return
	}

	decoded, err := encoder.DecodeBytes(encoded)
	if err != nil {
		t.Errorf("Failed to decode data: %v", err)
		return
	}

	if !compareSlices(decoded, rawStream) {
		t.Errorf("Slices not matching")
		t.Errorf("Decoded (%d): % x", len(encoded), encoded)
		t.Errorf("Raw     (%d): % x", len(rawStream), rawStream)
		return
	}
}

func TestPostDecodingPredictors(t *testing.T) {

	testcases := []struct {
		BitsPerComponent int
		Colors           int
		Columns          int
		Predictor        int
		Input            []byte
		Expected         []byte
	}{

		{
			BitsPerComponent: 8,
			Colors:           3,
			Columns:          3,
			Predictor:        15,
			Input: []byte{
				pfNone, 1, 2, 3, 1, 2, 3, 1, 2, 3,
				pfNone, 3, 2, 1, 3, 2, 1, 3, 2, 1,
				pfNone, 1, 2, 3, 1, 2, 3, 1, 2, 3,
			},
			Expected: []byte{
				1, 2, 3, 1, 2, 3, 1, 2, 3,
				3, 2, 1, 3, 2, 1, 3, 2, 1,
				1, 2, 3, 1, 2, 3, 1, 2, 3,
			},
		},

		{
			BitsPerComponent: 8,
			Colors:           3,
			Columns:          3,
			Predictor:        15,
			Input: []byte{
				pfSub, 1, 2, 3, 1, 2, 3, 1, 2, 3,
				pfSub, 3, 2, 1, 3, 2, 1, 3, 2, 1,
				pfSub, 1, 2, 3, 1, 2, 3, 1, 2, 3,
			},
			Expected: []byte{
				1, 2, 3, 1 + 1, 2 + 2, 3 + 3, 1 + 1 + 1, 2 + 2 + 2, 3 + 3 + 3,
				3, 2, 1, 3 + 3, 2 + 2, 1 + 1, 3 + 3 + 3, 2 + 2 + 2, 1 + 1 + 1,
				1, 2, 3, 1 + 1, 2 + 2, 3 + 3, 1 + 1 + 1, 2 + 2 + 2, 3 + 3 + 3,
			},
		},

		{
			BitsPerComponent: 8,
			Colors:           3,
			Columns:          3,
			Predictor:        15,
			Input: []byte{
				pfUp, 1, 2, 3, 1, 2, 3, 1, 2, 3,
				pfUp, 3, 2, 1, 3, 2, 1, 3, 2, 1,
				pfUp, 1, 2, 3, 1, 2, 3, 1, 2, 3,
			},
			Expected: []byte{
				1, 2, 3, 1, 2, 3, 1, 2, 3,
				3 + 1, 2 + 2, 1 + 3, 3 + 1, 2 + 2, 1 + 3, 3 + 1, 2 + 2, 1 + 3,
				1 + 3 + 1, 2 + 2 + 2, 3 + 1 + 3, 1 + 3 + 1, 2 + 2 + 2, 3 + 1 + 3, 1 + 3 + 1, 2 + 2 + 2, 3 + 1 + 3,
			},
		},

		{
			BitsPerComponent: 8,
			Colors:           3,
			Columns:          3,
			Predictor:        15,
			Input: []byte{
				pfAvg, 1, 2, 3, 1, 2, 3, 1, 2, 3,
				pfAvg, 3, 2, 1, 3, 2, 1, 3, 2, 1,
				pfAvg, 1, 2, 3, 1, 2, 3, 1, 2, 3,
			},
			Expected: []byte{
				1, 2, 3, 1, 3, 4, 1, 3, 5,
				3, 3, 2, 5, 5, 4, 6, 6, 5,
				2, 3, 4, 4, 6, 7, 6, 8, 9,
			},
		},

		{
			BitsPerComponent: 8,
			Colors:           3,
			Columns:          3,
			Predictor:        15,
			Input: []byte{
				pfPaeth, 1, 2, 3, 1, 2, 3, 1, 2, 3,
				pfPaeth, 3, 2, 1, 3, 2, 1, 3, 2, 1,
				pfPaeth, 1, 2, 3, 1, 2, 3, 1, 2, 3,
			},
			Expected: []byte{
				1, 2, 3, 2, 4, 6, 3, 6, 9,
				4, 4, 4, 7, 6, 7, 10, 8, 10,
				5, 6, 7, 8, 8, 10, 11, 10, 13,
			},
		},
	}

	for i, tcase := range testcases {
		encoder := &FlateEncoder{
			BitsPerComponent: tcase.BitsPerComponent,
			Colors:           tcase.Colors,
			Columns:          tcase.Columns,
			Predictor:        tcase.Predictor,
		}

		predicted, err := encoder.postDecodePredict(tcase.Input)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}

		t.Logf("%d: % d\n", i, predicted)
		if !compareSlices(predicted, tcase.Expected) {
			t.Errorf("Slices not matching (i = %d)", i)
			t.Errorf("Predicted (%d): % d", len(predicted), predicted)
			t.Fatalf("Expected  (%d): % d", len(tcase.Expected), tcase.Expected)
		}
	}
}

func TestLZWEncoding(t *testing.T) {
	rawStream := []byte("this is a dummy text with some \x01\x02\x03 binary data")

	encoder := NewLZWEncoder()

	encoder.EarlyChange = 0

	encoded, err := encoder.EncodeBytes(rawStream)
	if err != nil {
		t.Errorf("Failed to encode data: %v", err)
		return
	}

	decoded, err := encoder.DecodeBytes(encoded)
	if err != nil {
		t.Errorf("Failed to decode data: %v", err)
		return
	}

	if !compareSlices(decoded, rawStream) {
		t.Errorf("Slices not matching")
		t.Errorf("Decoded (%d): % x", len(encoded), encoded)
		t.Errorf("Raw     (%d): % x", len(rawStream), rawStream)
		return
	}
}

func TestRunLengthEncoding(t *testing.T) {
	rawStream := []byte("this is a dummy text with some \x01\x02\x03 binary data")
	encoder := NewRunLengthEncoder()
	encoded, err := encoder.EncodeBytes(rawStream)
	if err != nil {
		t.Errorf("Failed to RunLength encode data: %v", err)
		return
	}
	decoded, err := encoder.DecodeBytes(encoded)
	if err != nil {
		t.Errorf("Failed to RunLength decode data: %v", err)
		return
	}
	if !compareSlices(decoded, rawStream) {
		t.Errorf("Slices not matching. RunLength")
		t.Errorf("Decoded (%d): % x", len(encoded), encoded)
		t.Errorf("Raw     (%d): % x", len(rawStream), rawStream)
		return
	}
}

func TestASCIIHexEncoding(t *testing.T) {
	byteData := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	expected := []byte("DE AD BE EF >")

	encoder := NewASCIIHexEncoder()
	encoded, err := encoder.EncodeBytes(byteData)
	if err != nil {
		t.Errorf("Failed to encode data: %v", err)
		return
	}

	if !compareSlices(encoded, expected) {
		t.Errorf("Slices not matching")
		t.Errorf("Expected (%d): %s", len(expected), expected)
		t.Errorf("Encoded  (%d): %s", len(encoded), encoded)
		return
	}
}

func TestASCII85EncodingWikipediaExample(t *testing.T) {
	expected := `Man is distinguished, not only by his reason, but by this singular passion from other animals, which is a lust of the mind, that by a perseverance of delight in the continued and indefatigable generation of knowledge, exceeds the short vehemence of any carnal pleasure.`

	encodedInBase64 := `OWpxb15CbGJELUJsZUIxREorKitGKGYscS8wSmhLRjxHTD5DakAuNEdwJGQ3RiEsTDdAPDZAKS8wSkRFRjxHJTwrRVY6MkYhLE88REorKi5APCpLMEA8NkwoRGYtXDBFYzVlO0RmZlooRVplZS5CbC45cEYiQUdYQlBDc2krREdtPkAzQkIvRiomT0NBZnUyL0FLWWkoREliOkBGRCwqKStDXVU9QDNCTiNFY1lmOEFURDNzQHE/ZCRBZnRWcUNoW05xRjxHOjgrRVY6LitDZj4tRkQ1VzhBUmxvbERJYWwoRElkPGpAPD8zckA6RiVhK0Q1OCdBVEQ0JEJsQGwzRGU6LC1ESnNgOEFSb0ZiLzBKTUtAcUI0XkYhLFI8QUtaJi1EZlRxQkclRz51RC5SVHBBS1lvJytDVC81K0NlaSNESUk/KEUsOSlvRioyTTcvY34+`
	encoded, _ := base64.StdEncoding.DecodeString(encodedInBase64)

	encoder := NewASCII85Encoder()
	enc1, err := encoder.EncodeBytes([]byte(expected))
	if err != nil {
		t.Errorf("Fail")
		return
	}
	if string(enc1) != string(encoded) {
		t.Errorf("ASCII85 encoding wiki example fail")
		return
	}

	decoded, err := encoder.DecodeBytes([]byte(encoded))
	if err != nil {
		t.Errorf("Fail, error: %v", err)
		return
	}
	if expected != string(decoded) {
		t.Errorf("Mismatch! '%s' vs '%s'", decoded, expected)
		return
	}
}

func TestASCII85Encoding(t *testing.T) {
	encoded := `FD,B0+EVmJAKYo'+D#G#De*R"B-:o0+E_a:A0>T(+AbuZ@;]Tu:ddbqAnc'mEr~>`
	expected := "this type of encoding is used in PS and PDF files"

	encoder := NewASCII85Encoder()

	enc1, err := encoder.EncodeBytes([]byte(expected))
	if err != nil {
		t.Errorf("Fail")
		return
	}
	if encoded != string(enc1) {
		t.Errorf("Encoding error")
		return
	}

	decoded, err := encoder.DecodeBytes([]byte(encoded))
	if err != nil {
		t.Errorf("Fail, error: %v", err)
		return
	}
	if expected != string(decoded) {
		t.Errorf("Mismatch! '%s' vs '%s'", decoded, expected)
		return
	}
}

type TestASCII85DecodingTestCase struct {
	Encoded  string
	Expected string
}

func TestASCII85Decoding(t *testing.T) {

	testcases := []TestASCII85DecodingTestCase{
		{"z~>", "\x00\x00\x00\x00"},
		{"z ~>", "\x00\x00\x00\x00"},
		{"zz~>", "\x00\x00\x00\x00\x00\x00\x00\x00"},
		{" zz~>", "\x00\x00\x00\x00\x00\x00\x00\x00"},
		{" z z~>", "\x00\x00\x00\x00\x00\x00\x00\x00"},
		{" z z ~>", "\x00\x00\x00\x00\x00\x00\x00\x00"},
		{"+T~>", `!`},
		{"+`d~>", `!s`},
		{"+`hr~>", `!sz`},
		{"+`hsS~>", `!szx`},
		{"+`hsS+T~>", `!szx!`},
		{"+ `hs S +T ~>", `!szx!`},
	}

	encoder := NewASCII85Encoder()

	for _, testcase := range testcases {
		encoded := testcase.Encoded
		expected := testcase.Expected
		decoded, err := encoder.DecodeBytes([]byte(encoded))
		if err != nil {
			t.Errorf("Fail, error: %v", err)
			return
		}
		if expected != string(decoded) {
			t.Errorf("Mismatch! '%s' vs '%s'", decoded, expected)
			return
		}
	}
}

func TestMultiEncoder(t *testing.T) {
	rawStream := []byte("this is a dummy text with some \x01\x02\x03 binary data")

	encoder := NewMultiEncoder()

	enc1 := NewFlateEncoder()
	enc1.Predictor = 1
	encoder.AddEncoder(enc1)

	enc2 := NewASCIIHexEncoder()
	encoder.AddEncoder(enc2)

	encoded, err := encoder.EncodeBytes(rawStream)
	if err != nil {
		t.Errorf("Failed to encode data: %v", err)
		return
	}
	common.Log.Debug("Multi Encoded: %s", encoded)

	decoded, err := encoder.DecodeBytes(encoded)
	if err != nil {
		t.Errorf("Failed to decode data: %v", err)
		return
	}

	if !compareSlices(decoded, rawStream) {
		t.Errorf("Slices not matching")
		t.Errorf("Decoded (%d): % x", len(encoded), encoded)
		t.Errorf("Raw     (%d): % x", len(rawStream), rawStream)
		return
	}
}
