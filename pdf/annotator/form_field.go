package annotator

import (
	"bytes"
	"errors"

	"github.com/finalversus/doc/pdf/contentstream"
	"github.com/finalversus/doc/pdf/core"
	"github.com/finalversus/doc/pdf/model"
)

type TextFieldOptions struct {
	MaxLen int
	Value  string
}

func NewTextField(page *model.PdfPage, name string, rect []float64, opt TextFieldOptions) (*model.PdfFieldText, error) {
	if page == nil {
		return nil, errors.New("page not specified")
	}
	if len(name) <= 0 {
		return nil, errors.New("required attribute not specified")
	}
	if len(rect) != 4 {
		return nil, errors.New("invalid range")
	}

	field := model.NewPdfField()
	textfield := &model.PdfFieldText{}
	field.SetContext(textfield)
	textfield.PdfField = field

	textfield.T = core.MakeString(name)

	if opt.MaxLen > 0 {
		textfield.MaxLen = core.MakeInteger(int64(opt.MaxLen))
	}
	if len(opt.Value) > 0 {
		textfield.V = core.MakeString(opt.Value)
	}

	widget := model.NewPdfAnnotationWidget()
	widget.Rect = core.MakeArrayFromFloats(rect)
	widget.P = page.ToPdfObject()
	widget.F = core.MakeInteger(4)
	widget.Parent = textfield.ToPdfObject()

	textfield.Annotations = append(textfield.Annotations, widget)

	return textfield, nil
}

type CheckboxFieldOptions struct {
	Checked bool
}

func NewCheckboxField(page *model.PdfPage, name string, rect []float64, opt CheckboxFieldOptions) (*model.PdfFieldButton, error) {
	if page == nil {
		return nil, errors.New("page not specified")
	}
	if len(name) <= 0 {
		return nil, errors.New("required attribute not specified")
	}
	if len(rect) != 4 {
		return nil, errors.New("invalid range")
	}

	zapfdb, err := model.NewStandard14Font(model.ZapfDingbatsName)
	if err != nil {
		return nil, err
	}

	field := model.NewPdfField()
	buttonfield := &model.PdfFieldButton{}
	field.SetContext(buttonfield)
	buttonfield.PdfField = field

	buttonfield.T = core.MakeString(name)
	buttonfield.SetType(model.ButtonTypeCheckbox)

	state := "Off"
	if opt.Checked {
		state = "Yes"
	}

	buttonfield.V = core.MakeName(state)

	widget := model.NewPdfAnnotationWidget()
	widget.Rect = core.MakeArrayFromFloats(rect)
	widget.P = page.ToPdfObject()
	widget.F = core.MakeInteger(4)
	widget.Parent = buttonfield.ToPdfObject()

	w := rect[2] - rect[0]
	h := rect[3] - rect[1]

	var cs bytes.Buffer
	cs.WriteString("q\n")
	cs.WriteString("0 0 1 rg\n")
	cs.WriteString("BT\n")
	cs.WriteString("/ZaDb 12 Tf\n")
	cs.WriteString("ET\n")
	cs.WriteString("Q\n")

	cc := contentstream.NewContentCreator()
	cc.Add_q()
	cc.Add_rg(0, 0, 1)
	cc.Add_BT()
	cc.Add_Tf(*core.MakeName("ZaDb"), 12)
	cc.Add_Td(0, 0)
	cc.Add_ET()
	cc.Add_Q()

	xformOff := model.NewXObjectForm()
	xformOff.SetContentStream(cc.Bytes(), core.NewRawEncoder())
	xformOff.BBox = core.MakeArrayFromFloats([]float64{0, 0, w, h})
	xformOff.Resources = model.NewPdfPageResources()
	xformOff.Resources.SetFontByName("ZaDb", zapfdb.ToPdfObject())

	cc = contentstream.NewContentCreator()
	cc.Add_q()
	cc.Add_re(0, 0, w, h)
	cc.Add_W().Add_n()
	cc.Add_rg(0, 0, 1)
	cc.Translate(0, 3.0)
	cc.Add_BT()
	cc.Add_Tf(*core.MakeName("ZaDb"), 12)
	cc.Add_Td(0, 0)
	cc.Add_Tj(*core.MakeString("\064"))
	cc.Add_ET()
	cc.Add_Q()

	xformOn := model.NewXObjectForm()
	xformOn.SetContentStream(cc.Bytes(), core.NewRawEncoder())
	xformOn.BBox = core.MakeArrayFromFloats([]float64{0, 0, w, h})
	xformOn.Resources = model.NewPdfPageResources()
	xformOn.Resources.SetFontByName("ZaDb", zapfdb.ToPdfObject())

	dchoiceapp := core.MakeDict()
	dchoiceapp.Set("Off", xformOff.ToPdfObject())
	dchoiceapp.Set("Yes", xformOn.ToPdfObject())

	appearance := core.MakeDict()
	appearance.Set("N", dchoiceapp)

	widget.AP = appearance
	widget.AS = core.MakeName(state)

	buttonfield.Annotations = append(buttonfield.Annotations, widget)

	return buttonfield, nil
}

type ComboboxFieldOptions struct {
	Choices []string
}

func NewComboboxField(page *model.PdfPage, name string, rect []float64, opt ComboboxFieldOptions) (*model.PdfFieldChoice, error) {
	if page == nil {
		return nil, errors.New("page not specified")
	}
	if len(name) <= 0 {
		return nil, errors.New("required attribute not specified")
	}
	if len(rect) != 4 {
		return nil, errors.New("invalid range")
	}

	field := model.NewPdfField()
	chfield := &model.PdfFieldChoice{}
	field.SetContext(chfield)
	chfield.PdfField = field

	chfield.T = core.MakeString(name)
	chfield.Opt = core.MakeArray()
	for _, choicestr := range opt.Choices {
		chfield.Opt.Append(core.MakeString(choicestr))
	}
	chfield.SetFlag(model.FieldFlagCombo)

	widget := model.NewPdfAnnotationWidget()
	widget.Rect = core.MakeArrayFromFloats(rect)
	widget.P = page.ToPdfObject()
	widget.F = core.MakeInteger(4)
	widget.Parent = chfield.ToPdfObject()

	chfield.Annotations = append(chfield.Annotations, widget)

	return chfield, nil
}

type SignatureLine struct {
	Desc string
	Text string
}

func NewSignatureLine(desc, text string) *SignatureLine {
	return &SignatureLine{
		Desc: desc,
		Text: text,
	}
}

type SignatureFieldOpts struct {
	Rect []float64

	AutoSize bool

	Font *model.PdfFont

	FontSize float64

	LineHeight float64

	TextColor model.PdfColor

	FillColor model.PdfColor

	BorderSize float64

	BorderColor model.PdfColor
}

func NewSignatureFieldOpts() *SignatureFieldOpts {
	return &SignatureFieldOpts{
		Font:        model.DefaultFont(),
		FontSize:    10,
		LineHeight:  1,
		AutoSize:    true,
		TextColor:   model.NewPdfColorDeviceGray(0),
		BorderColor: model.NewPdfColorDeviceGray(0),
		FillColor:   model.NewPdfColorDeviceGray(1),
	}
}

func NewSignatureField(signature *model.PdfSignature, lines []*SignatureLine, opts *SignatureFieldOpts) (*model.PdfFieldSignature, error) {
	if signature == nil {
		return nil, errors.New("signature cannot be nil")
	}

	apDict, err := genFieldSignatureAppearance(lines, opts)
	if err != nil {
		return nil, err
	}

	field := model.NewPdfFieldSignature(signature)
	field.Rect = core.MakeArrayFromFloats(opts.Rect)
	field.AP = apDict
	return field, nil
}
