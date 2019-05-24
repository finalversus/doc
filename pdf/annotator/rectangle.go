package annotator

import (
	"github.com/finalversus/doc/common"

	"github.com/finalversus/doc/pdf/contentstream/draw"
	pdfcore "github.com/finalversus/doc/pdf/core"
	pdf "github.com/finalversus/doc/pdf/model"
)

type RectangleAnnotationDef struct {
	X             float64
	Y             float64
	Width         float64
	Height        float64
	FillEnabled   bool
	FillColor     *pdf.PdfColorDeviceRGB
	BorderEnabled bool
	BorderWidth   float64
	BorderColor   *pdf.PdfColorDeviceRGB
	Opacity       float64
}

func CreateRectangleAnnotation(rectDef RectangleAnnotationDef) (*pdf.PdfAnnotation, error) {
	rectAnnotation := pdf.NewPdfAnnotationSquare()

	if rectDef.BorderEnabled {
		r, g, b := rectDef.BorderColor.R(), rectDef.BorderColor.G(), rectDef.BorderColor.B()
		rectAnnotation.C = pdfcore.MakeArrayFromFloats([]float64{r, g, b})
		bs := pdf.NewBorderStyle()
		bs.SetBorderWidth(rectDef.BorderWidth)
		rectAnnotation.BS = bs.ToPdfObject()
	}

	if rectDef.FillEnabled {
		r, g, b := rectDef.FillColor.R(), rectDef.FillColor.G(), rectDef.FillColor.B()
		rectAnnotation.IC = pdfcore.MakeArrayFromFloats([]float64{r, g, b})
	} else {
		rectAnnotation.IC = pdfcore.MakeArrayFromIntegers([]int{})
	}

	if rectDef.Opacity < 1.0 {
		rectAnnotation.CA = pdfcore.MakeFloat(rectDef.Opacity)
	}

	apDict, bbox, err := makeRectangleAnnotationAppearanceStream(rectDef)
	if err != nil {
		return nil, err
	}

	rectAnnotation.AP = apDict
	rectAnnotation.Rect = pdfcore.MakeArrayFromFloats([]float64{bbox.Llx, bbox.Lly, bbox.Urx, bbox.Ury})

	return rectAnnotation.PdfAnnotation, nil

}

func makeRectangleAnnotationAppearanceStream(rectDef RectangleAnnotationDef) (*pdfcore.PdfObjectDictionary, *pdf.PdfRectangle, error) {
	form := pdf.NewXObjectForm()
	form.Resources = pdf.NewPdfPageResources()

	gsName := ""
	if rectDef.Opacity < 1.0 {

		gsState := pdfcore.MakeDict()
		gsState.Set("ca", pdfcore.MakeFloat(rectDef.Opacity))
		gsState.Set("CA", pdfcore.MakeFloat(rectDef.Opacity))
		err := form.Resources.AddExtGState("gs1", gsState)
		if err != nil {
			common.Log.Debug("Unable to add extgstate gs1")
			return nil, nil, err
		}

		gsName = "gs1"
	}

	content, localBbox, globalBbox, err := drawPdfRectangle(rectDef, gsName)
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

func drawPdfRectangle(rectDef RectangleAnnotationDef, gsName string) ([]byte, *pdf.PdfRectangle, *pdf.PdfRectangle, error) {

	rect := draw.Rectangle{
		X:             0,
		Y:             0,
		Width:         rectDef.Width,
		Height:        rectDef.Height,
		FillEnabled:   rectDef.FillEnabled,
		FillColor:     rectDef.FillColor,
		BorderEnabled: rectDef.BorderEnabled,
		BorderWidth:   2 * rectDef.BorderWidth,
		BorderColor:   rectDef.BorderColor,
		Opacity:       rectDef.Opacity,
	}

	content, localBbox, err := rect.Draw(gsName)
	if err != nil {
		return nil, nil, nil, err
	}

	globalBbox := &pdf.PdfRectangle{}
	globalBbox.Llx = rectDef.X + localBbox.Llx
	globalBbox.Lly = rectDef.Y + localBbox.Lly
	globalBbox.Urx = rectDef.X + localBbox.Urx
	globalBbox.Ury = rectDef.Y + localBbox.Ury

	return content, localBbox, globalBbox, nil
}
