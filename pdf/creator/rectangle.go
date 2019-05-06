package creator

import (
	"github.com/codefinio/doc/pdf/contentstream/draw"
	"github.com/codefinio/doc/pdf/model"
)

type Rectangle struct {
	x           float64
	y           float64
	width       float64
	height      float64
	fillColor   *model.PdfColorDeviceRGB
	borderColor *model.PdfColorDeviceRGB
	borderWidth float64
}

func newRectangle(x, y, width, height float64) *Rectangle {
	rect := &Rectangle{}

	rect.x = x
	rect.y = y
	rect.width = width
	rect.height = height

	rect.borderColor = model.NewPdfColorDeviceRGB(0, 0, 0)
	rect.borderWidth = 1.0

	return rect
}

func (rect *Rectangle) GetCoords() (float64, float64) {
	return rect.x, rect.y
}

func (rect *Rectangle) SetBorderWidth(bw float64) {
	rect.borderWidth = bw
}

func (rect *Rectangle) SetBorderColor(col Color) {
	rect.borderColor = model.NewPdfColorDeviceRGB(col.ToRGB())
}

func (rect *Rectangle) SetFillColor(col Color) {
	rect.fillColor = model.NewPdfColorDeviceRGB(col.ToRGB())
}

func (rect *Rectangle) GeneratePageBlocks(ctx DrawContext) ([]*Block, DrawContext, error) {
	block := NewBlock(ctx.PageWidth, ctx.PageHeight)

	drawrect := draw.Rectangle{
		Opacity: 1.0,
		X:       rect.x,
		Y:       ctx.PageHeight - rect.y - rect.height,
		Height:  rect.height,
		Width:   rect.width,
	}
	if rect.fillColor != nil {
		drawrect.FillEnabled = true
		drawrect.FillColor = rect.fillColor
	}
	if rect.borderColor != nil && rect.borderWidth > 0 {
		drawrect.BorderEnabled = true
		drawrect.BorderColor = rect.borderColor
		drawrect.BorderWidth = rect.borderWidth
	}

	contents, _, err := drawrect.Draw("")
	if err != nil {
		return nil, ctx, err
	}

	err = block.addContentsByString(string(contents))
	if err != nil {
		return nil, ctx, err
	}

	return []*Block{block}, ctx, nil
}
