package annotator

import (
	"github.com/finalversus/doc/common"

	"github.com/finalversus/doc/pdf/contentstream/draw"
	pdfcore "github.com/finalversus/doc/pdf/core"
	pdf "github.com/finalversus/doc/pdf/model"
)

type CircleAnnotationDef struct {
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

func CreateCircleAnnotation(circDef CircleAnnotationDef) (*pdf.PdfAnnotation, error) {
	circAnnotation := pdf.NewPdfAnnotationCircle()

	if circDef.BorderEnabled {
		r, g, b := circDef.BorderColor.R(), circDef.BorderColor.G(), circDef.BorderColor.B()
		circAnnotation.C = pdfcore.MakeArrayFromFloats([]float64{r, g, b})
		bs := pdf.NewBorderStyle()
		bs.SetBorderWidth(circDef.BorderWidth)
		circAnnotation.BS = bs.ToPdfObject()
	}

	if circDef.FillEnabled {
		r, g, b := circDef.FillColor.R(), circDef.FillColor.G(), circDef.FillColor.B()
		circAnnotation.IC = pdfcore.MakeArrayFromFloats([]float64{r, g, b})
	} else {
		circAnnotation.IC = pdfcore.MakeArrayFromIntegers([]int{})
	}

	if circDef.Opacity < 1.0 {
		circAnnotation.CA = pdfcore.MakeFloat(circDef.Opacity)
	}

	apDict, bbox, err := makeCircleAnnotationAppearanceStream(circDef)
	if err != nil {
		return nil, err
	}

	circAnnotation.AP = apDict
	circAnnotation.Rect = pdfcore.MakeArrayFromFloats([]float64{bbox.Llx, bbox.Lly, bbox.Urx, bbox.Ury})

	return circAnnotation.PdfAnnotation, nil

}

func makeCircleAnnotationAppearanceStream(circDef CircleAnnotationDef) (*pdfcore.PdfObjectDictionary, *pdf.PdfRectangle, error) {
	form := pdf.NewXObjectForm()
	form.Resources = pdf.NewPdfPageResources()

	gsName := ""
	if circDef.Opacity < 1.0 {

		gsState := pdfcore.MakeDict()
		gsState.Set("ca", pdfcore.MakeFloat(circDef.Opacity))
		gsState.Set("CA", pdfcore.MakeFloat(circDef.Opacity))
		err := form.Resources.AddExtGState("gs1", gsState)
		if err != nil {
			common.Log.Debug("Unable to add extgstate gs1")
			return nil, nil, err
		}

		gsName = "gs1"
	}

	content, localBbox, globalBbox, err := drawPdfCircle(circDef, gsName)
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

func drawPdfCircle(circDef CircleAnnotationDef, gsName string) ([]byte, *pdf.PdfRectangle, *pdf.PdfRectangle, error) {

	circle := draw.Circle{
		X:             circDef.X,
		Y:             circDef.Y,
		Width:         circDef.Width,
		Height:        circDef.Height,
		FillEnabled:   circDef.FillEnabled,
		FillColor:     circDef.FillColor,
		BorderEnabled: circDef.BorderEnabled,
		BorderWidth:   circDef.BorderWidth,
		BorderColor:   circDef.BorderColor,
		Opacity:       circDef.Opacity,
	}

	content, localBbox, err := circle.Draw(gsName)
	if err != nil {
		return nil, nil, nil, err
	}

	globalBbox := &pdf.PdfRectangle{}
	globalBbox.Llx = circDef.X + localBbox.Llx
	globalBbox.Lly = circDef.Y + localBbox.Lly
	globalBbox.Urx = circDef.X + localBbox.Urx
	globalBbox.Ury = circDef.Y + localBbox.Ury

	return content, localBbox, globalBbox, nil
}
