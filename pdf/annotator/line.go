package annotator

import (
	"github.com/codefinio/doc/common"
	"github.com/codefinio/doc/pdf/contentstream/draw"
	pdfcore "github.com/codefinio/doc/pdf/core"
	pdf "github.com/codefinio/doc/pdf/model"
)

type LineAnnotationDef struct {
	X1               float64
	Y1               float64
	X2               float64
	Y2               float64
	LineColor        *pdf.PdfColorDeviceRGB
	Opacity          float64
	LineWidth        float64
	LineEndingStyle1 draw.LineEndingStyle
	LineEndingStyle2 draw.LineEndingStyle
}

func CreateLineAnnotation(lineDef LineAnnotationDef) (*pdf.PdfAnnotation, error) {

	lineAnnotation := pdf.NewPdfAnnotationLine()

	lineAnnotation.L = pdfcore.MakeArrayFromFloats([]float64{lineDef.X1, lineDef.Y1, lineDef.X2, lineDef.Y2})

	le1 := pdfcore.MakeName("None")
	if lineDef.LineEndingStyle1 == draw.LineEndingStyleArrow {
		le1 = pdfcore.MakeName("ClosedArrow")
	}
	le2 := pdfcore.MakeName("None")
	if lineDef.LineEndingStyle2 == draw.LineEndingStyleArrow {
		le2 = pdfcore.MakeName("ClosedArrow")
	}
	lineAnnotation.LE = pdfcore.MakeArray(le1, le2)

	if lineDef.Opacity < 1.0 {
		lineAnnotation.CA = pdfcore.MakeFloat(lineDef.Opacity)
	}

	r, g, b := lineDef.LineColor.R(), lineDef.LineColor.G(), lineDef.LineColor.B()
	lineAnnotation.IC = pdfcore.MakeArrayFromFloats([]float64{r, g, b})
	lineAnnotation.C = pdfcore.MakeArrayFromFloats([]float64{r, g, b})
	bs := pdf.NewBorderStyle()
	bs.SetBorderWidth(lineDef.LineWidth)
	lineAnnotation.BS = bs.ToPdfObject()

	apDict, bbox, err := makeLineAnnotationAppearanceStream(lineDef)
	if err != nil {
		return nil, err
	}
	lineAnnotation.AP = apDict

	lineAnnotation.Rect = pdfcore.MakeArrayFromFloats([]float64{bbox.Llx, bbox.Lly, bbox.Urx, bbox.Ury})

	return lineAnnotation.PdfAnnotation, nil
}

func makeLineAnnotationAppearanceStream(lineDef LineAnnotationDef) (*pdfcore.PdfObjectDictionary, *pdf.PdfRectangle, error) {
	form := pdf.NewXObjectForm()
	form.Resources = pdf.NewPdfPageResources()

	gsName := ""
	if lineDef.Opacity < 1.0 {

		gsState := pdfcore.MakeDict()
		gsState.Set("ca", pdfcore.MakeFloat(lineDef.Opacity))
		err := form.Resources.AddExtGState("gs1", gsState)
		if err != nil {
			common.Log.Debug("Unable to add extgstate gs1")
			return nil, nil, err
		}

		gsName = "gs1"
	}

	content, localBbox, globalBbox, err := drawPdfLine(lineDef, gsName)
	if err != nil {
		return nil, nil, err
	}

	err = form.SetContentStream(content, nil)
	if err != nil {
		return nil, nil, err
	}

	form.BBox = localBbox.ToPdfObject()

	apDict := pdfcore.MakeDict()
	apDict.Set("N", form.ToPdfObject())

	return apDict, globalBbox, nil
}

func drawPdfLine(lineDef LineAnnotationDef, gsName string) ([]byte, *pdf.PdfRectangle, *pdf.PdfRectangle, error) {

	line := draw.Line{
		X1:               0,
		Y1:               0,
		X2:               lineDef.X2 - lineDef.X1,
		Y2:               lineDef.Y2 - lineDef.Y1,
		LineColor:        lineDef.LineColor,
		Opacity:          lineDef.Opacity,
		LineWidth:        lineDef.LineWidth,
		LineEndingStyle1: lineDef.LineEndingStyle1,
		LineEndingStyle2: lineDef.LineEndingStyle2,
	}

	content, localBbox, err := line.Draw(gsName)
	if err != nil {
		return nil, nil, nil, err
	}

	globalBbox := &pdf.PdfRectangle{}
	globalBbox.Llx = lineDef.X1 + localBbox.Llx
	globalBbox.Lly = lineDef.Y1 + localBbox.Lly
	globalBbox.Urx = lineDef.X1 + localBbox.Urx
	globalBbox.Ury = lineDef.Y1 + localBbox.Ury

	return content, localBbox, globalBbox, nil
}
