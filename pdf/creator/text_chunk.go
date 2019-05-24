package creator

import (
	"github.com/finalversus/doc/pdf/core"
	"github.com/finalversus/doc/pdf/model"
)

type TextChunk struct {
	Text string

	Style TextStyle

	annotation *model.PdfAnnotation

	annotationProcessed bool
}

func newTextChunk(text string, style TextStyle) *TextChunk {
	return &TextChunk{
		Text:  text,
		Style: style,
	}
}

func newExternalLinkAnnotation(url string) *model.PdfAnnotation {
	annotation := model.NewPdfAnnotationLink()

	bs := model.NewBorderStyle()
	bs.SetBorderWidth(0)
	annotation.BS = bs.ToPdfObject()

	action := core.MakeDict()
	action.Set(core.PdfObjectName("S"), core.MakeName("URI"))
	action.Set(core.PdfObjectName("URI"), core.MakeString(url))
	annotation.A = action

	return annotation.PdfAnnotation
}

func newInternalLinkAnnotation(page int64, x, y, zoom float64) *model.PdfAnnotation {
	annotation := model.NewPdfAnnotationLink()

	bs := model.NewBorderStyle()
	bs.SetBorderWidth(0)
	annotation.BS = bs.ToPdfObject()

	if page < 0 {
		page = 0
	}

	annotation.Dest = core.MakeArray(
		core.MakeInteger(page),
		core.MakeName("XYZ"),
		core.MakeFloat(x),
		core.MakeFloat(y),
		core.MakeFloat(zoom),
	)

	return annotation.PdfAnnotation
}

func copyLinkAnnotation(link *model.PdfAnnotationLink) *model.PdfAnnotationLink {
	if link == nil {
		return nil
	}

	annotation := model.NewPdfAnnotationLink()
	annotation.BS = link.BS
	annotation.A = link.A

	if annotDest, ok := link.Dest.(*core.PdfObjectArray); ok {
		annotation.Dest = core.MakeArray(annotDest.Elements()...)
	}

	return annotation
}
