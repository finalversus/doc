package extractor

import (
	"github.com/finalversus/doc/common"
	"github.com/finalversus/doc/pdf/contentstream"
	"github.com/finalversus/doc/pdf/core"
	"github.com/finalversus/doc/pdf/model"
)

type ImageExtractOptions struct {
	IncludeInlineStencilMasks bool
}

func (e *Extractor) ExtractPageImages(options *ImageExtractOptions) (*PageImages, error) {
	ctx := &imageExtractContext{
		options: options,
	}

	err := ctx.extractContentStreamImages(e.contents, e.resources)
	if err != nil {
		return nil, err
	}

	return &PageImages{
		Images: ctx.extractedImages,
	}, nil
}

type PageImages struct {
	Images []ImageMark
}

type ImageMark struct {
	Image *model.Image

	Width  float64
	Height float64

	X float64
	Y float64

	Angle float64
}

type imageExtractContext struct {
	extractedImages []ImageMark
	inlineImages    int
	xObjectImages   int
	xObjectForms    int

	cacheXObjectImages map[*core.PdfObjectStream]*cachedImage

	options *ImageExtractOptions
}

type cachedImage struct {
	image *model.Image
	cs    model.PdfColorspace
}

func (ctx *imageExtractContext) extractContentStreamImages(contents string, resources *model.PdfPageResources) error {
	cstreamParser := contentstream.NewContentStreamParser(contents)
	operations, err := cstreamParser.Parse()
	if err != nil {
		return err
	}

	if ctx.cacheXObjectImages == nil {
		ctx.cacheXObjectImages = map[*core.PdfObjectStream]*cachedImage{}
	}
	if ctx.options == nil {
		ctx.options = &ImageExtractOptions{}
	}

	processor := contentstream.NewContentStreamProcessor(*operations)
	processor.AddHandler(contentstream.HandlerConditionEnumAllOperands, "",
		func(op *contentstream.ContentStreamOperation, gs contentstream.GraphicsState, resources *model.PdfPageResources) error {
			return ctx.processOperand(op, gs, resources)
		})

	return processor.Process(resources)
}

func (ctx *imageExtractContext) processOperand(op *contentstream.ContentStreamOperation, gs contentstream.GraphicsState, resources *model.PdfPageResources) error {
	if op.Operand == "BI" && len(op.Params) == 1 {

		iimg, ok := op.Params[0].(*contentstream.ContentStreamInlineImage)
		if !ok {
			return nil
		}

		if isImageMask, ok := core.GetBoolVal(iimg.ImageMask); ok {
			if isImageMask && !ctx.options.IncludeInlineStencilMasks {
				return nil
			}
		}

		return ctx.extractInlineImage(iimg, gs, resources)
	} else if op.Operand == "Do" && len(op.Params) == 1 {

		name, ok := core.GetName(op.Params[0])
		if !ok {
			common.Log.Debug("ERROR: Type")
			return errTypeCheck
		}

		_, xtype := resources.GetXObjectByName(*name)
		switch xtype {
		case model.XObjectTypeImage:
			return ctx.extractXObjectImage(name, gs, resources)
		case model.XObjectTypeForm:
			return ctx.extractFormImages(name, gs, resources)
		}
	}
	return nil
}

func (ctx *imageExtractContext) extractInlineImage(iimg *contentstream.ContentStreamInlineImage, gs contentstream.GraphicsState, resources *model.PdfPageResources) error {
	img, err := iimg.ToImage(resources)
	if err != nil {
		return err
	}

	cs, err := iimg.GetColorSpace(resources)
	if err != nil {
		return err
	}
	if cs == nil {

		cs = model.NewPdfColorspaceDeviceGray()
	}

	rgbImg, err := cs.ImageToRGB(*img)
	if err != nil {
		return err
	}

	imgMark := ImageMark{
		Image:  &rgbImg,
		Width:  gs.CTM.ScalingFactorX(),
		Height: gs.CTM.ScalingFactorY(),
		Angle:  gs.CTM.Angle(),
	}
	imgMark.X, imgMark.Y = gs.CTM.Translation()

	ctx.extractedImages = append(ctx.extractedImages, imgMark)
	ctx.inlineImages++
	return nil
}

func (ctx *imageExtractContext) extractXObjectImage(name *core.PdfObjectName, gs contentstream.GraphicsState, resources *model.PdfPageResources) error {
	stream, _ := resources.GetXObjectByName(*name)
	if stream == nil {
		return nil
	}

	cimg, cached := ctx.cacheXObjectImages[stream]
	if !cached {
		ximg, err := resources.GetXObjectImageByName(*name)
		if err != nil {
			return err
		}
		if ximg == nil {
			return nil
		}

		img, err := ximg.ToImage()
		if err != nil {
			return err
		}

		cimg = &cachedImage{
			image: img,
			cs:    ximg.ColorSpace,
		}
		ctx.cacheXObjectImages[stream] = cimg
	}
	img := cimg.image
	cs := cimg.cs

	rgbImg, err := cs.ImageToRGB(*img)
	if err != nil {
		return err
	}

	common.Log.Debug("@Do CTM: %s", gs.CTM.String())
	imgMark := ImageMark{
		Image:  &rgbImg,
		Width:  gs.CTM.ScalingFactorX(),
		Height: gs.CTM.ScalingFactorY(),
		Angle:  gs.CTM.Angle(),
	}
	imgMark.X, imgMark.Y = gs.CTM.Translation()

	ctx.extractedImages = append(ctx.extractedImages, imgMark)
	ctx.xObjectImages++
	return nil
}

func (ctx *imageExtractContext) extractFormImages(name *core.PdfObjectName, gs contentstream.GraphicsState, resources *model.PdfPageResources) error {
	xform, err := resources.GetXObjectFormByName(*name)
	if err != nil {
		return err
	}
	if xform == nil {
		return nil
	}

	formContent, err := xform.GetContentStream()
	if err != nil {
		return err
	}

	formResources := xform.Resources
	if formResources == nil {
		formResources = resources
	}

	err = ctx.extractContentStreamImages(string(formContent), formResources)
	if err != nil {
		return err
	}
	ctx.xObjectForms++
	return nil
}
