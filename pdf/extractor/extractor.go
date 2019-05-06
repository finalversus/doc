package extractor

import (
	"github.com/codefinio/doc/pdf/model"
)

type Extractor struct {
	contents  string
	resources *model.PdfPageResources

	fontCache map[string]fontEntry

	formResults map[string]textResult

	accessCount int64

	textCount int64
}

func New(page *model.PdfPage) (*Extractor, error) {
	contents, err := page.GetAllContentStreams()
	if err != nil {
		return nil, err
	}

	e := &Extractor{
		contents:    contents,
		resources:   page.Resources,
		fontCache:   map[string]fontEntry{},
		formResults: map[string]textResult{},
	}
	return e, nil
}
