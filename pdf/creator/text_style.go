package creator

import (
	"github.com/finalversus/doc/pdf/model"
)

type TextStyle struct {
	Color Color

	Font *model.PdfFont

	FontSize float64

	CharSpacing float64

	RenderingMode TextRenderingMode
}

func newTextStyle(font *model.PdfFont) TextStyle {
	return TextStyle{
		Color:    ColorRGBFrom8bit(0, 0, 0),
		Font:     font,
		FontSize: 10,
	}
}

func newLinkStyle(font *model.PdfFont) TextStyle {
	return TextStyle{
		Color:    ColorRGBFrom8bit(0, 0, 238),
		Font:     font,
		FontSize: 10,
	}
}
