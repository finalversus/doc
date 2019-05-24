package contentstream

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/finalversus/doc/common"
	"github.com/finalversus/doc/pdf/core"
	"github.com/finalversus/doc/pdf/model"
)

type ContentStreamInlineImage struct {
	BitsPerComponent core.PdfObject
	ColorSpace       core.PdfObject
	Decode           core.PdfObject
	DecodeParms      core.PdfObject
	Filter           core.PdfObject
	Height           core.PdfObject
	ImageMask        core.PdfObject
	Intent           core.PdfObject
	Interpolate      core.PdfObject
	Width            core.PdfObject
	stream           []byte
}

func NewInlineImageFromImage(img model.Image, encoder core.StreamEncoder) (*ContentStreamInlineImage, error) {
	if encoder == nil {
		encoder = core.NewRawEncoder()
	}

	inlineImage := ContentStreamInlineImage{}
	if img.ColorComponents == 1 {
		inlineImage.ColorSpace = core.MakeName("G")
	} else if img.ColorComponents == 3 {
		inlineImage.ColorSpace = core.MakeName("RGB")
	} else if img.ColorComponents == 4 {
		inlineImage.ColorSpace = core.MakeName("CMYK")
	} else {
		common.Log.Debug("Invalid number of color components for inline image: %d", img.ColorComponents)
		return nil, errors.New("invalid number of color components")
	}
	inlineImage.BitsPerComponent = core.MakeInteger(img.BitsPerComponent)
	inlineImage.Width = core.MakeInteger(img.Width)
	inlineImage.Height = core.MakeInteger(img.Height)

	encoded, err := encoder.EncodeBytes(img.Data)
	if err != nil {
		return nil, err
	}

	inlineImage.stream = encoded

	filterName := encoder.GetFilterName()
	if filterName != core.StreamEncodingFilterNameRaw {
		inlineImage.Filter = core.MakeName(filterName)
	}

	return &inlineImage, nil
}

func (img *ContentStreamInlineImage) String() string {
	s := fmt.Sprintf("InlineImage(len=%d)\n", len(img.stream))
	if img.BitsPerComponent != nil {
		s += "- BPC " + img.BitsPerComponent.WriteString() + "\n"
	}
	if img.ColorSpace != nil {
		s += "- CS " + img.ColorSpace.WriteString() + "\n"
	}
	if img.Decode != nil {
		s += "- D " + img.Decode.WriteString() + "\n"
	}
	if img.DecodeParms != nil {
		s += "- DP " + img.DecodeParms.WriteString() + "\n"
	}
	if img.Filter != nil {
		s += "- F " + img.Filter.WriteString() + "\n"
	}
	if img.Height != nil {
		s += "- H " + img.Height.WriteString() + "\n"
	}
	if img.ImageMask != nil {
		s += "- IM " + img.ImageMask.WriteString() + "\n"
	}
	if img.Intent != nil {
		s += "- Intent " + img.Intent.WriteString() + "\n"
	}
	if img.Interpolate != nil {
		s += "- I " + img.Interpolate.WriteString() + "\n"
	}
	if img.Width != nil {
		s += "- W " + img.Width.WriteString() + "\n"
	}
	return s
}

func (img *ContentStreamInlineImage) WriteString() string {
	var output bytes.Buffer

	s := ""

	if img.BitsPerComponent != nil {
		s += "/BPC " + img.BitsPerComponent.WriteString() + "\n"
	}
	if img.ColorSpace != nil {
		s += "/CS " + img.ColorSpace.WriteString() + "\n"
	}
	if img.Decode != nil {
		s += "/D " + img.Decode.WriteString() + "\n"
	}
	if img.DecodeParms != nil {
		s += "/DP " + img.DecodeParms.WriteString() + "\n"
	}
	if img.Filter != nil {
		s += "/F " + img.Filter.WriteString() + "\n"
	}
	if img.Height != nil {
		s += "/H " + img.Height.WriteString() + "\n"
	}
	if img.ImageMask != nil {
		s += "/IM " + img.ImageMask.WriteString() + "\n"
	}
	if img.Intent != nil {
		s += "/Intent " + img.Intent.WriteString() + "\n"
	}
	if img.Interpolate != nil {
		s += "/I " + img.Interpolate.WriteString() + "\n"
	}
	if img.Width != nil {
		s += "/W " + img.Width.WriteString() + "\n"
	}
	output.WriteString(s)

	output.WriteString("ID ")
	output.Write(img.stream)
	output.WriteString("\nEI\n")

	return output.String()
}

func (img *ContentStreamInlineImage) GetColorSpace(resources *model.PdfPageResources) (model.PdfColorspace, error) {
	if img.ColorSpace == nil {

		common.Log.Debug("Inline image not having specified colorspace, assuming Gray")
		return model.NewPdfColorspaceDeviceGray(), nil
	}

	if arr, isArr := img.ColorSpace.(*core.PdfObjectArray); isArr {
		return newIndexedColorspaceFromPdfObject(arr)
	}

	name, ok := img.ColorSpace.(*core.PdfObjectName)
	if !ok {
		common.Log.Debug("Error: Invalid object type (%T;%+v)", img.ColorSpace, img.ColorSpace)
		return nil, errors.New("type check error")
	}

	if *name == "G" || *name == "DeviceGray" {
		return model.NewPdfColorspaceDeviceGray(), nil
	} else if *name == "RGB" || *name == "DeviceRGB" {
		return model.NewPdfColorspaceDeviceRGB(), nil
	} else if *name == "CMYK" || *name == "DeviceCMYK" {
		return model.NewPdfColorspaceDeviceCMYK(), nil
	} else if *name == "I" || *name == "Indexed" {
		return nil, errors.New("unsupported Index colorspace")
	} else {
		if resources.ColorSpace == nil {

			common.Log.Debug("Error, unsupported inline image colorspace: %s", *name)
			return nil, errors.New("unknown colorspace")
		}

		cs, has := resources.GetColorspaceByName(*name)
		if !has {

			common.Log.Debug("Error, unsupported inline image colorspace: %s", *name)
			return nil, errors.New("unknown colorspace")
		}

		return cs, nil
	}

}

func (img *ContentStreamInlineImage) GetEncoder() (core.StreamEncoder, error) {
	return newEncoderFromInlineImage(img)
}

func (img *ContentStreamInlineImage) IsMask() (bool, error) {
	if img.ImageMask != nil {
		imMask, ok := img.ImageMask.(*core.PdfObjectBool)
		if !ok {
			common.Log.Debug("Image mask not a boolean")
			return false, errors.New("invalid object type")
		}

		return bool(*imMask), nil
	}

	return false, nil
}

func (img *ContentStreamInlineImage) ToImage(resources *model.PdfPageResources) (*model.Image, error) {

	encoder, err := newEncoderFromInlineImage(img)
	if err != nil {
		return nil, err
	}
	common.Log.Trace("encoder: %+v %T", encoder, encoder)
	common.Log.Trace("inline image: %+v", img)

	decoded, err := encoder.DecodeBytes(img.stream)
	if err != nil {
		return nil, err
	}

	image := &model.Image{}

	if img.Height == nil {
		return nil, errors.New("height attribute missing")
	}
	height, ok := img.Height.(*core.PdfObjectInteger)
	if !ok {
		return nil, errors.New("invalid height")
	}
	image.Height = int64(*height)

	if img.Width == nil {
		return nil, errors.New("width attribute missing")
	}
	width, ok := img.Width.(*core.PdfObjectInteger)
	if !ok {
		return nil, errors.New("invalid width")
	}
	image.Width = int64(*width)

	isMask, err := img.IsMask()
	if err != nil {
		return nil, err
	}

	if isMask {

		image.BitsPerComponent = 1
		image.ColorComponents = 1
	} else {

		if img.BitsPerComponent == nil {
			common.Log.Debug("Inline Bits per component missing - assuming 8")
			image.BitsPerComponent = 8
		} else {
			bpc, ok := img.BitsPerComponent.(*core.PdfObjectInteger)
			if !ok {
				common.Log.Debug("Error invalid bits per component value, type %T", img.BitsPerComponent)
				return nil, errors.New("BPC Type error")
			}
			image.BitsPerComponent = int64(*bpc)
		}

		if img.ColorSpace != nil {
			cs, err := img.GetColorSpace(resources)
			if err != nil {
				return nil, err
			}
			image.ColorComponents = cs.GetNumComponents()
		} else {

			common.Log.Debug("Inline Image colorspace not specified - assuming 1 color component")
			image.ColorComponents = 1
		}
	}

	image.Data = decoded

	return image, nil
}

func (csp *ContentStreamParser) ParseInlineImage() (*ContentStreamInlineImage, error) {

	im := ContentStreamInlineImage{}

	for {
		csp.skipSpaces()
		obj, isOperand, err := csp.parseObject()
		if err != nil {
			return nil, err
		}

		if !isOperand {

			param, ok := obj.(*core.PdfObjectName)
			if !ok {
				common.Log.Debug("Invalid inline image property (expecting name) - %T", obj)
				return nil, fmt.Errorf("invalid inline image property (expecting name) - %T", obj)
			}

			valueObj, isOperand, err := csp.parseObject()
			if err != nil {
				return nil, err
			}
			if isOperand {
				return nil, fmt.Errorf("not expecting an operand")
			}

			switch *param {
			case "BPC", "BitsPerComponent":
				im.BitsPerComponent = valueObj
			case "CS", "ColorSpace":
				im.ColorSpace = valueObj
			case "D", "Decode":
				im.Decode = valueObj
			case "DP", "DecodeParms":
				im.DecodeParms = valueObj
			case "F", "Filter":
				im.Filter = valueObj
			case "H", "Height":
				im.Height = valueObj
			case "IM", "ImageMask":
				im.ImageMask = valueObj
			case "Intent":
				im.Intent = valueObj
			case "I", "Interpolate":
				im.Interpolate = valueObj
			case "W", "Width":
				im.Width = valueObj
			default:
				return nil, fmt.Errorf("unknown inline image parameter %s", *param)
			}
		}

		if isOperand {
			operand, ok := obj.(*core.PdfObjectString)
			if !ok {
				return nil, fmt.Errorf("failed to read inline image - invalid operand")
			}

			if operand.Str() == "EI" {

				common.Log.Trace("Inline image finished...")
				return &im, nil
			} else if operand.Str() == "ID" {

				common.Log.Trace("ID start")

				b, err := csp.reader.Peek(1)
				if err != nil {
					return nil, err
				}
				if core.IsWhiteSpace(b[0]) {
					csp.reader.Discard(1)
				}

				im.stream = []byte{}
				state := 0
				var skipBytes []byte
				for {
					c, err := csp.reader.ReadByte()
					if err != nil {
						common.Log.Debug("Unable to find end of image EI in inline image data")
						return nil, err
					}

					if state == 0 {
						if core.IsWhiteSpace(c) {
							skipBytes = []byte{}
							skipBytes = append(skipBytes, c)
							state = 1
						} else {
							im.stream = append(im.stream, c)
						}
					} else if state == 1 {
						skipBytes = append(skipBytes, c)
						if c == 'E' {
							state = 2
						} else {
							im.stream = append(im.stream, skipBytes...)
							skipBytes = []byte{}

							if core.IsWhiteSpace(c) {
								state = 1
							} else {
								state = 0
							}
						}
					} else if state == 2 {
						skipBytes = append(skipBytes, c)
						if c == 'I' {
							state = 3
						} else {
							im.stream = append(im.stream, skipBytes...)
							skipBytes = []byte{}
							state = 0
						}
					} else if state == 3 {
						skipBytes = append(skipBytes, c)
						if core.IsWhiteSpace(c) {

							if len(im.stream) > 100 {
								common.Log.Trace("Image stream (%d): % x ...", len(im.stream), im.stream[:100])
							} else {
								common.Log.Trace("Image stream (%d): % x", len(im.stream), im.stream)
							}

							return &im, nil
						}

						im.stream = append(im.stream, skipBytes...)
						skipBytes = []byte{}
						state = 0
					}
				}

			}
		}
	}
}
