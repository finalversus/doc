package core

import (
	"fmt"

	"github.com/codefinio/doc/common"
)

func NewEncoderFromStream(streamObj *PdfObjectStream) (StreamEncoder, error) {
	filterObj := TraceToDirectObject(streamObj.PdfObjectDictionary.Get("Filter"))
	if filterObj == nil {

		return NewRawEncoder(), nil
	}

	if _, isNull := filterObj.(*PdfObjectNull); isNull {

		return NewRawEncoder(), nil
	}

	method, ok := filterObj.(*PdfObjectName)
	if !ok {
		array, ok := filterObj.(*PdfObjectArray)
		if !ok {
			return nil, fmt.Errorf("filter not a Name or Array object")
		}
		if array.Len() == 0 {

			return NewRawEncoder(), nil
		}

		if array.Len() != 1 {
			menc, err := newMultiEncoderFromStream(streamObj)
			if err != nil {
				common.Log.Error("Failed creating multi encoder: %v", err)
				return nil, err
			}

			common.Log.Trace("Multi enc: %s\n", menc)
			return menc, nil
		}

		filterObj = array.Get(0)
		method, ok = filterObj.(*PdfObjectName)
		if !ok {
			return nil, fmt.Errorf("filter array member not a Name object")
		}
	}

	if *method == StreamEncodingFilterNameFlate {
		return newFlateEncoderFromStream(streamObj, nil)
	} else if *method == StreamEncodingFilterNameLZW {
		return newLZWEncoderFromStream(streamObj, nil)
	} else if *method == StreamEncodingFilterNameDCT {
		return newDCTEncoderFromStream(streamObj, nil)
	} else if *method == StreamEncodingFilterNameRunLength {
		return newRunLengthEncoderFromStream(streamObj, nil)
	} else if *method == StreamEncodingFilterNameASCIIHex {
		return NewASCIIHexEncoder(), nil
	} else if *method == StreamEncodingFilterNameASCII85 || *method == "A85" {
		return NewASCII85Encoder(), nil
	} else if *method == StreamEncodingFilterNameCCITTFax {
		return newCCITTFaxEncoderFromStream(streamObj, nil)
	} else if *method == StreamEncodingFilterNameJBIG2 {
		return NewJBIG2Encoder(), nil
	} else if *method == StreamEncodingFilterNameJPX {
		return NewJPXEncoder(), nil
	} else {
		common.Log.Debug("ERROR: Unsupported encoding method!")
		return nil, fmt.Errorf("unsupported encoding method (%s)", *method)
	}
}

func DecodeStream(streamObj *PdfObjectStream) ([]byte, error) {
	common.Log.Trace("Decode stream")

	encoder, err := NewEncoderFromStream(streamObj)
	if err != nil {
		common.Log.Debug("ERROR: Stream decoding failed: %v", err)
		return nil, err
	}
	common.Log.Trace("Encoder: %#v\n", encoder)

	decoded, err := encoder.DecodeStream(streamObj)
	if err != nil {
		common.Log.Debug("ERROR: Stream decoding failed: %v", err)
		return nil, err
	}

	return decoded, nil
}

func EncodeStream(streamObj *PdfObjectStream) error {
	common.Log.Trace("Encode stream")

	encoder, err := NewEncoderFromStream(streamObj)
	if err != nil {
		common.Log.Debug("Stream decoding failed: %v", err)
		return err
	}

	if lzwenc, is := encoder.(*LZWEncoder); is {

		lzwenc.EarlyChange = 0
		streamObj.PdfObjectDictionary.Set("EarlyChange", MakeInteger(0))
	}

	common.Log.Trace("Encoder: %+v\n", encoder)
	encoded, err := encoder.EncodeBytes(streamObj.Stream)
	if err != nil {
		common.Log.Debug("Stream encoding failed: %v", err)
		return err
	}

	streamObj.Stream = encoded

	streamObj.PdfObjectDictionary.Set("Length", MakeInteger(int64(len(encoded))))

	return nil
}
