package creator

import (
	"os"

	"github.com/finalversus/doc/pdf/contentstream/draw"
	"github.com/finalversus/doc/pdf/model"
)

func loadPagesFromFile(f *os.File) ([]*model.PdfPage, error) {
	pdfReader, err := model.NewPdfReader(f)
	if err != nil {
		return nil, err
	}

	numPages, err := pdfReader.GetNumPages()
	if err != nil {
		return nil, err
	}

	var pages []*model.PdfPage
	for i := 0; i < numPages; i++ {
		page, err := pdfReader.GetPage(i + 1)
		if err != nil {
			return nil, err
		}

		pages = append(pages, page)
	}

	return pages, nil
}

func rotateRect(w, h, angle float64) (x, y, rotatedWidth, rotatedHeight float64) {
	if angle == 0 {
		return 0, 0, w, h
	}

	bbox := draw.Path{Points: []draw.Point{
		draw.NewPoint(0, 0).Rotate(angle),
		draw.NewPoint(w, 0).Rotate(angle),
		draw.NewPoint(0, h).Rotate(angle),
		draw.NewPoint(w, h).Rotate(angle),
	}}.GetBoundingBox()

	return bbox.X, bbox.Y, bbox.Width, bbox.Height
}
