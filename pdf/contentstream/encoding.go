package contentstream

import (
	"bytes"
	"errors"
	"fmt"
	gocolor "image/color"
	"image/jpeg"

	"github.com/finalversus/doc/common"
	"github.com/finalversus/doc/pdf/core"
)

func newEncoderFromInlineImage(inlineImage *ContentStreamInlineImage) (core.StreamEncoder, error) {
	if inlineImage.Filter == nil {

		return core.NewRawEncoder(), nil
	}

	filterName, ok := inlineImage.Filter.(*core.PdfObjectName)
	if !ok {
		array, ok := inlineImage.Filter.(*core.PdfObjectArray)
		if !ok {
			return nil, fmt.Errorf("filter not a Name or Array object")
		}
		if array.Len() == 0 {

			return core.NewRawEncoder(), nil
		}

		if array.Len() != 1 {
			menc, err := newMultiEncoderFromInlineImage(inlineImage)
			if err != nil {
				common.Log.Error("Failed creating multi encoder: %v", err)
				return nil, err
			}

			common.Log.Trace("Multi enc: %s\n", menc)
			return menc, nil
		}

		filterObj := array.Get(0)
		filterName, ok = filterObj.(*core.PdfObjectName)
		if !ok {
			return nil, fmt.Errorf("filter array member not a Name object")
		}
	}

	switch *filterName {
	case "AHx", "ASCIIHexDecode":
		return core.NewASCIIHexEncoder(), nil
	case "A85", "ASCII85Decode":
		return core.NewASCII85Encoder(), nil
	case "DCT", "DCTDecode":
		return newDCTEncoderFromInlineImage(inlineImage)
	case "Fl", "FlateDecode":
		return newFlateEncoderFromInlineImage(inlineImage, nil)
	case "LZW", "LZWDecode":
		return newLZWEncoderFromInlineImage(inlineImage, nil)
	case "CCF", "CCITTFaxDecode":
		return core.NewCCITTFaxEncoder(), nil
	case "RL", "RunLengthDecode":
		return core.NewRunLengthEncoder(), nil
	default:
		common.Log.Debug("Unsupported inline image encoding filter name : %s", *filterName)
		return nil, errors.New("unsupported inline encoding method")
	}
}

func newFlateEncoderFromInlineImage(inlineImage *ContentStreamInlineImage, decodeParams *core.PdfObjectDictionary) (*core.FlateEncoder, error) {
	encoder := core.NewFlateEncoder()

	if decodeParams == nil {
		obj := inlineImage.DecodeParms
		if obj != nil {
			dp, isDict := obj.(*core.PdfObjectDictionary)
			if !isDict {
				common.Log.Debug("Error: DecodeParms not a dictionary (%T)", obj)
				return nil, fmt.Errorf("invalid DecodeParms")
			}
			decodeParams = dp
		}
	}
	if decodeParams == nil {

		return encoder, nil
	}

	common.Log.Trace("decode params: %s", decodeParams.String())
	obj := decodeParams.Get("Predictor")
	if obj == nil {
		common.Log.Debug("Error: Predictor missing from DecodeParms - Continue with default (1)")
	} else {
		predictor, ok := obj.(*core.PdfObjectInteger)
		if !ok {
			common.Log.Debug("Error: Predictor specified but not numeric (%T)", obj)
			return nil, fmt.Errorf("invalid Predictor")
		}
		encoder.Predictor = int(*predictor)
	}

	obj = decodeParams.Get("BitsPerComponent")
	if obj != nil {
		bpc, ok := obj.(*core.PdfObjectInteger)
		if !ok {
			common.Log.Debug("ERROR: Invalid BitsPerComponent")
			return nil, fmt.Errorf("invalid BitsPerComponent")
		}
		encoder.BitsPerComponent = int(*bpc)
	}

	if encoder.Predictor > 1 {

		encoder.Columns = 1
		obj = decodeParams.Get("Columns")
		if obj != nil {
			columns, ok := obj.(*core.PdfObjectInteger)
			if !ok {
				return nil, fmt.Errorf("predictor column invalid")
			}

			encoder.Columns = int(*columns)
		}

		encoder.Colors = 1
		obj := decodeParams.Get("Colors")
		if obj != nil {
			colors, ok := obj.(*core.PdfObjectInteger)
			if !ok {
				return nil, fmt.Errorf("predictor colors not an integer")
			}
			encoder.Colors = int(*colors)
		}
	}

	return encoder, nil
}

func newLZWEncoderFromInlineImage(inlineImage *ContentStreamInlineImage, decodeParams *core.PdfObjectDictionary) (*core.LZWEncoder, error) {

	encoder := core.NewLZWEncoder()

	if decodeParams == nil {
		if inlineImage.DecodeParms != nil {
			dp, isDict := inlineImage.DecodeParms.(*core.PdfObjectDictionary)
			if !isDict {
				common.Log.Debug("Error: DecodeParms not a dictionary (%T)", inlineImage.DecodeParms)
				return nil, fmt.Errorf("invalid DecodeParms")
			}
			decodeParams = dp
		}
	}

	if decodeParams == nil {

		return encoder, nil
	}

	obj := decodeParams.Get("EarlyChange")
	if obj != nil {
		earlyChange, ok := obj.(*core.PdfObjectInteger)
		if !ok {
			common.Log.Debug("Error: EarlyChange specified but not numeric (%T)", obj)
			return nil, fmt.Errorf("invalid EarlyChange")
		}
		if *earlyChange != 0 && *earlyChange != 1 {
			return nil, fmt.Errorf("invalid EarlyChange value (not 0 or 1)")
		}

		encoder.EarlyChange = int(*earlyChange)
	} else {
		encoder.EarlyChange = 1
	}

	obj = decodeParams.Get("Predictor")
	if obj != nil {
		predictor, ok := obj.(*core.PdfObjectInteger)
		if !ok {
			common.Log.Debug("Error: Predictor specified but not numeric (%T)", obj)
			return nil, fmt.Errorf("invalid Predictor")
		}
		encoder.Predictor = int(*predictor)
	}

	obj = decodeParams.Get("BitsPerComponent")
	if obj != nil {
		bpc, ok := obj.(*core.PdfObjectInteger)
		if !ok {
			common.Log.Debug("ERROR: Invalid BitsPerComponent")
			return nil, fmt.Errorf("invalid BitsPerComponent")
		}
		encoder.BitsPerComponent = int(*bpc)
	}

	if encoder.Predictor > 1 {

		encoder.Columns = 1
		obj = decodeParams.Get("Columns")
		if obj != nil {
			columns, ok := obj.(*core.PdfObjectInteger)
			if !ok {
				return nil, fmt.Errorf("predictor column invalid")
			}

			encoder.Columns = int(*columns)
		}

		encoder.Colors = 1
		obj = decodeParams.Get("Colors")
		if obj != nil {
			colors, ok := obj.(*core.PdfObjectInteger)
			if !ok {
				return nil, fmt.Errorf("predictor colors not an integer")
			}
			encoder.Colors = int(*colors)
		}
	}

	common.Log.Trace("decode params: %s", decodeParams.String())
	return encoder, nil
}

func newDCTEncoderFromInlineImage(inlineImage *ContentStreamInlineImage) (*core.DCTEncoder, error) {

	encoder := core.NewDCTEncoder()

	bufReader := bytes.NewReader(inlineImage.stream)

	cfg, err := jpeg.DecodeConfig(bufReader)

	if err != nil {
		common.Log.Debug("Error decoding file: %s", err)
		return nil, err
	}

	switch cfg.ColorModel {
	case gocolor.RGBAModel:
		encoder.BitsPerComponent = 8
		encoder.ColorComponents = 3
	case gocolor.RGBA64Model:
		encoder.BitsPerComponent = 16
		encoder.ColorComponents = 3
	case gocolor.GrayModel:
		encoder.BitsPerComponent = 8
		encoder.ColorComponents = 1
	case gocolor.Gray16Model:
		encoder.BitsPerComponent = 16
		encoder.ColorComponents = 1
	case gocolor.CMYKModel:
		encoder.BitsPerComponent = 8
		encoder.ColorComponents = 4
	case gocolor.YCbCrModel:

		encoder.BitsPerComponent = 8
		encoder.ColorComponents = 3
	default:
		return nil, errors.New("unsupported color model")
	}
	encoder.Width = cfg.Width
	encoder.Height = cfg.Height
	common.Log.Trace("DCT Encoder: %+v", encoder)

	return encoder, nil
}

func newMultiEncoderFromInlineImage(inlineImage *ContentStreamInlineImage) (*core.MultiEncoder, error) {
	mencoder := core.NewMultiEncoder()

	var decodeParamsDict *core.PdfObjectDictionary
	var decodeParamsArray []core.PdfObject
	if obj := inlineImage.DecodeParms; obj != nil {

		dict, isDict := obj.(*core.PdfObjectDictionary)
		if isDict {
			decodeParamsDict = dict
		}

		arr, isArray := obj.(*core.PdfObjectArray)
		if isArray {
			for _, dictObj := range arr.Elements() {
				if dict, is := dictObj.(*core.PdfObjectDictionary); is {
					decodeParamsArray = append(decodeParamsArray, dict)
				} else {
					decodeParamsArray = append(decodeParamsArray, nil)
				}
			}
		}
	}

	obj := inlineImage.Filter
	if obj == nil {
		return nil, fmt.Errorf("filter missing")
	}

	array, ok := obj.(*core.PdfObjectArray)
	if !ok {
		return nil, fmt.Errorf("multi filter can only be made from array")
	}

	for idx, obj := range array.Elements() {
		name, ok := obj.(*core.PdfObjectName)
		if !ok {
			return nil, fmt.Errorf("multi filter array element not a name")
		}

		var dp core.PdfObject

		if decodeParamsDict != nil {
			dp = decodeParamsDict
		} else {

			if len(decodeParamsArray) > 0 {
				if idx >= len(decodeParamsArray) {
					return nil, fmt.Errorf("missing elements in decode params array")
				}
				dp = decodeParamsArray[idx]
			}
		}

		var dParams *core.PdfObjectDictionary
		if dict, is := dp.(*core.PdfObjectDictionary); is {
			dParams = dict
		}

		if *name == core.StreamEncodingFilterNameFlate || *name == "Fl" {

			encoder, err := newFlateEncoderFromInlineImage(inlineImage, dParams)
			if err != nil {
				return nil, err
			}
			mencoder.AddEncoder(encoder)
		} else if *name == core.StreamEncodingFilterNameLZW {
			encoder, err := newLZWEncoderFromInlineImage(inlineImage, dParams)
			if err != nil {
				return nil, err
			}
			mencoder.AddEncoder(encoder)
		} else if *name == core.StreamEncodingFilterNameASCIIHex {
			encoder := core.NewASCIIHexEncoder()
			mencoder.AddEncoder(encoder)
		} else if *name == core.StreamEncodingFilterNameASCII85 || *name == "A85" {
			encoder := core.NewASCII85Encoder()
			mencoder.AddEncoder(encoder)
		} else {
			common.Log.Error("Unsupported filter %s", *name)
			return nil, fmt.Errorf("invalid filter in multi filter array")
		}
	}

	return mencoder, nil
}
