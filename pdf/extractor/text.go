package extractor

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/finalversus/doc/common"
	"github.com/finalversus/doc/pdf/contentstream"
	"github.com/finalversus/doc/pdf/core"
	"github.com/finalversus/doc/pdf/internal/transform"
	"github.com/finalversus/doc/pdf/model"
	"golang.org/x/text/unicode/norm"
)

func (e *Extractor) ExtractText() (string, error) {
	text, _, _, err := e.ExtractTextWithStats()
	return text, err
}

func (e *Extractor) ExtractTextWithStats() (extracted string, numChars int, numMisses int, err error) {
	pageText, numChars, numMisses, err := e.ExtractPageText()
	if err != nil {
		return "", numChars, numMisses, err
	}
	return pageText.ToText(), numChars, numMisses, nil
}

func (e *Extractor) ExtractPageText() (*PageText, int, int, error) {
	return e.extractPageText(e.contents, e.resources, 0)
}

func (e *Extractor) extractPageText(contents string, resources *model.PdfPageResources, level int) (*PageText, int, int, error) {

	common.Log.Trace("extractPageText: level=%d", level)
	pageText := &PageText{}
	state := newTextState()
	fontStack := fontStacker{}
	var to *textObject

	cstreamParser := contentstream.NewContentStreamParser(contents)
	operations, err := cstreamParser.Parse()
	if err != nil {
		common.Log.Debug("ERROR: extractPageText parse failed. err=%v", err)
		return pageText, state.numChars, state.numMisses, err
	}

	processor := contentstream.NewContentStreamProcessor(*operations)

	processor.AddHandler(contentstream.HandlerConditionEnumAllOperands, "",
		func(op *contentstream.ContentStreamOperation, gs contentstream.GraphicsState,
			resources *model.PdfPageResources) error {

			operand := op.Operand

			switch operand {
			case "q":
				if !fontStack.empty() {
					common.Log.Trace("Save font state: %s\n%s",
						fontStack.peek(), fontStack.String())
					fontStack.push(fontStack.peek())
				}
				if state.tfont != nil {
					common.Log.Trace("Save font state: %s\n->%s\n%s",
						fontStack.peek(), state.tfont, fontStack.String())
					fontStack.push(state.tfont)
				}
			case "Q":
				if !fontStack.empty() {
					common.Log.Trace("Restore font state: %s\n->%s\n%s",
						fontStack.peek(), fontStack.get(-2), fontStack.String())
					fontStack.pop()
				}
				if len(fontStack) >= 2 {
					common.Log.Trace("Restore font state: %s\n->%s\n%s",
						state.tfont, fontStack.peek(), fontStack.String())
					state.tfont = fontStack.pop()
				}
			case "BT":

				if to != nil {
					common.Log.Debug("BT called while in a text object")
				}
				to = newTextObject(e, resources, gs, &state, &fontStack)
			case "ET":
				pageText.marks = append(pageText.marks, to.marks...)
				to = nil
			case "T*":
				to.nextLine()
			case "Td":
				if ok, err := to.checkOp(op, 2, true); !ok {
					common.Log.Debug("ERROR: err=%v", err)
					return err
				}
				x, y, err := toFloatXY(op.Params)
				if err != nil {
					return err
				}
				to.moveText(x, y)
			case "TD":
				if ok, err := to.checkOp(op, 2, true); !ok {
					common.Log.Debug("ERROR: err=%v", err)
					return err
				}
				x, y, err := toFloatXY(op.Params)
				if err != nil {
					common.Log.Debug("ERROR: err=%v", err)
					return err
				}
				to.moveTextSetLeading(x, y)
			case "Tj":
				if ok, err := to.checkOp(op, 1, true); !ok {
					common.Log.Debug("ERROR: Tj op=%s err=%v", op, err)
					return err
				}
				charcodes, ok := core.GetStringBytes(op.Params[0])
				if !ok {
					common.Log.Debug("ERROR: Tj op=%s GetStringBytes failed", op)
					return core.ErrTypeError
				}
				return to.showText(charcodes)
			case "TJ":
				if ok, err := to.checkOp(op, 1, true); !ok {
					common.Log.Debug("ERROR: TJ err=%v", err)
					return err
				}
				args, ok := core.GetArray(op.Params[0])
				if !ok {
					common.Log.Debug("ERROR: TJ op=%s GetArrayVal failed", op)
					return err
				}
				return to.showTextAdjusted(args)
			case "'":
				if ok, err := to.checkOp(op, 1, true); !ok {
					common.Log.Debug("ERROR: ' err=%v", err)
					return err
				}
				charcodes, ok := core.GetStringBytes(op.Params[0])
				if !ok {
					common.Log.Debug("ERROR: ' op=%s GetStringBytes failed", op)
					return core.ErrTypeError
				}
				to.nextLine()
				return to.showText(charcodes)
			case `"`:
				if ok, err := to.checkOp(op, 1, true); !ok {
					common.Log.Debug("ERROR: \" err=%v", err)
					return err
				}
				x, y, err := toFloatXY(op.Params[:2])
				if err != nil {
					return err
				}
				charcodes, ok := core.GetStringBytes(op.Params[2])
				if !ok {
					common.Log.Debug("ERROR: \" op=%s GetStringBytes failed", op)
					return core.ErrTypeError
				}
				to.setCharSpacing(x)
				to.setWordSpacing(y)
				to.nextLine()
				return to.showText(charcodes)
			case "TL":
				y, err := floatParam(op)
				if err != nil {
					common.Log.Debug("ERROR: TL err=%v", err)
					return err
				}
				to.setTextLeading(y)
			case "Tc":
				y, err := floatParam(op)
				if err != nil {
					common.Log.Debug("ERROR: Tc err=%v", err)
					return err
				}
				to.setCharSpacing(y)
			case "Tf":
				if to == nil {

					to = newTextObject(e, resources, gs, &state, &fontStack)
				}
				if ok, err := to.checkOp(op, 2, true); !ok {
					common.Log.Debug("ERROR: Tf err=%v", err)
					return err
				}
				name, ok := core.GetNameVal(op.Params[0])
				if !ok {
					common.Log.Debug("ERROR: Tf op=%s GetNameVal failed", op)
					return core.ErrTypeError
				}
				size, err := core.GetNumberAsFloat(op.Params[1])
				if !ok {
					common.Log.Debug("ERROR: Tf op=%s GetFloatVal failed. err=%v", op, err)
					return err
				}
				err = to.setFont(name, size)
				if err != nil {
					return err
				}
			case "Tm":
				if ok, err := to.checkOp(op, 6, true); !ok {
					common.Log.Debug("ERROR: Tm err=%v", err)
					return err
				}
				floats, err := core.GetNumbersAsFloat(op.Params)
				if err != nil {
					common.Log.Debug("ERROR: err=%v", err)
					return err
				}
				to.setTextMatrix(floats)
			case "Tr":
				if ok, err := to.checkOp(op, 1, true); !ok {
					common.Log.Debug("ERROR: Tr err=%v", err)
					return err
				}
				mode, ok := core.GetIntVal(op.Params[0])
				if !ok {
					common.Log.Debug("ERROR: Tr op=%s GetIntVal failed", op)
					return core.ErrTypeError
				}
				to.setTextRenderMode(mode)
			case "Ts":
				if ok, err := to.checkOp(op, 1, true); !ok {
					common.Log.Debug("ERROR: Ts err=%v", err)
					return err
				}
				y, err := core.GetNumberAsFloat(op.Params[0])
				if err != nil {
					common.Log.Debug("ERROR: err=%v", err)
					return err
				}
				to.setTextRise(y)
			case "Tw":
				if ok, err := to.checkOp(op, 1, true); !ok {
					common.Log.Debug("ERROR: err=%v", err)
					return err
				}
				y, err := core.GetNumberAsFloat(op.Params[0])
				if err != nil {
					common.Log.Debug("ERROR: err=%v", err)
					return err
				}
				to.setWordSpacing(y)
			case "Tz":
				if ok, err := to.checkOp(op, 1, true); !ok {
					common.Log.Debug("ERROR: err=%v", err)
					return err
				}
				y, err := core.GetNumberAsFloat(op.Params[0])
				if err != nil {
					common.Log.Debug("ERROR: err=%v", err)
					return err
				}
				to.setHorizScaling(y)

			case "Do":

				name := *op.Params[0].(*core.PdfObjectName)
				_, xtype := resources.GetXObjectByName(name)
				if xtype != model.XObjectTypeForm {
					break
				}

				formResult, ok := e.formResults[string(name)]
				if !ok {
					xform, err := resources.GetXObjectFormByName(name)
					if err != nil {
						common.Log.Debug("ERROR: %v", err)
						return err
					}
					formContent, err := xform.GetContentStream()
					if err != nil {
						common.Log.Debug("ERROR: %v", err)
						return err
					}
					formResources := xform.Resources
					if formResources == nil {
						formResources = resources
					}
					tList, numChars, numMisses, err := e.extractPageText(string(formContent),
						formResources, level+1)
					if err != nil {
						common.Log.Debug("ERROR: %v", err)
						return err
					}
					formResult = textResult{*tList, numChars, numMisses}
					e.formResults[string(name)] = formResult
				}

				pageText.marks = append(pageText.marks, formResult.pageText.marks...)
				state.numChars += formResult.numChars
				state.numMisses += formResult.numMisses
			}
			return nil
		})

	err = processor.Process(resources)
	if err != nil {
		common.Log.Debug("ERROR: Processing: err=%v", err)
	}
	return pageText, state.numChars, state.numMisses, err
}

type textResult struct {
	pageText  PageText
	numChars  int
	numMisses int
}

func (to *textObject) moveText(tx, ty float64) {
	to.moveTo(tx, ty)
}

func (to *textObject) moveTextSetLeading(tx, ty float64) {
	to.state.tl = -ty
	to.moveTo(tx, ty)
}

func (to *textObject) nextLine() {
	to.moveTo(0, -to.state.tl)
}

func (to *textObject) setTextMatrix(f []float64) {
	if len(f) != 6 {
		common.Log.Debug("ERROR: len(f) != 6 (%d)", len(f))
		return
	}
	a, b, c, d, tx, ty := f[0], f[1], f[2], f[3], f[4], f[5]
	to.tm = transform.NewMatrix(a, b, c, d, tx, ty)
	to.tlm = to.tm
}

func (to *textObject) showText(charcodes []byte) error {
	return to.renderText(charcodes)
}

func (to *textObject) showTextAdjusted(args *core.PdfObjectArray) error {
	vertical := false
	for _, o := range args.Elements() {
		switch o.(type) {
		case *core.PdfObjectFloat, *core.PdfObjectInteger:
			x, err := core.GetNumberAsFloat(o)
			if err != nil {
				common.Log.Debug("ERROR: showTextAdjusted. Bad numerical arg. o=%s args=%+v", o, args)
				return err
			}
			dx, dy := -x*0.001*to.state.tfs, 0.0
			if vertical {
				dy, dx = dx, dy
			}
			td := translationMatrix(transform.Point{X: dx, Y: dy})
			to.tm.Concat(td)
			common.Log.Trace("showTextAdjusted: dx,dy=%3f,%.3f Tm=%s", dx, dy, to.tm)
		case *core.PdfObjectString:
			charcodes, ok := core.GetStringBytes(o)
			if !ok {
				common.Log.Trace("showTextAdjusted: Bad string arg. o=%s args=%+v", o, args)
				return core.ErrTypeError
			}
			to.renderText(charcodes)
		default:
			common.Log.Debug("ERROR: showTextAdjusted. Unexpected type (%T) args=%+v", o, args)
			return core.ErrTypeError
		}
	}
	return nil
}

func (to *textObject) setTextLeading(y float64) {
	if to == nil || to.state == nil {
		return
	}
	to.state.tl = y
}

func (to *textObject) setCharSpacing(x float64) {
	if to == nil {
		return
	}
	to.state.tc = x
}

func (to *textObject) setFont(name string, size float64) error {
	if to == nil {
		return nil
	}
	font, err := to.getFont(name)
	if err == nil {
		to.state.tfont = font
		if len(*to.fontStack) == 0 {
			to.fontStack.push(font)
		} else {
			(*to.fontStack)[len(*to.fontStack)-1] = font
		}
	} else if err == model.ErrFontNotSupported {

		return err
	} else {
		return err
	}
	to.state.tfs = size
	return nil
}

func (to *textObject) setTextRenderMode(mode int) {
	if to == nil {
		return
	}
	to.state.tmode = RenderMode(mode)
}

func (to *textObject) setTextRise(y float64) {
	if to == nil {
		return
	}
	to.state.trise = y
}

func (to *textObject) setWordSpacing(y float64) {
	if to == nil {
		return
	}
	to.state.tw = y
}

func (to *textObject) setHorizScaling(y float64) {
	if to == nil {
		return
	}
	to.state.th = y
}

func floatParam(op *contentstream.ContentStreamOperation) (float64, error) {
	if len(op.Params) != 1 {
		err := errors.New("incorrect parameter count")
		common.Log.Debug("ERROR: %#q should have %d input params, got %d %+v",
			op.Operand, 1, len(op.Params), op.Params)
		return 0.0, err
	}
	return core.GetNumberAsFloat(op.Params[0])
}

func (to *textObject) checkOp(op *contentstream.ContentStreamOperation, numParams int,
	hard bool) (ok bool, err error) {
	if to == nil {
		var params []core.PdfObject
		if numParams > 0 {
			params = op.Params
			if len(params) > numParams {
				params = params[:numParams]
			}
		}
		common.Log.Debug("%#q operand outside text. params=%+v", op.Operand, params)
	}
	if numParams >= 0 {
		if len(op.Params) != numParams {
			if hard {
				err = errors.New("incorrect parameter count")
			}
			common.Log.Debug("ERROR: %#q should have %d input params, got %d %+v",
				op.Operand, numParams, len(op.Params), op.Params)
			return false, err
		}
	}
	return true, nil
}

type fontStacker []*model.PdfFont

func (fontStack *fontStacker) String() string {
	parts := []string{"---- font stack"}
	for i, font := range *fontStack {
		s := "<nil>"
		if font != nil {
			s = font.String()
		}
		parts = append(parts, fmt.Sprintf("\t%2d: %s", i, s))
	}
	return strings.Join(parts, "\n")
}

func (fontStack *fontStacker) push(font *model.PdfFont) {
	*fontStack = append(*fontStack, font)
}

func (fontStack *fontStacker) pop() *model.PdfFont {
	if fontStack.empty() {
		return nil
	}
	font := (*fontStack)[len(*fontStack)-1]
	*fontStack = (*fontStack)[:len(*fontStack)-1]
	return font
}

func (fontStack *fontStacker) peek() *model.PdfFont {
	if fontStack.empty() {
		return nil
	}
	return (*fontStack)[len(*fontStack)-1]
}

func (fontStack *fontStacker) get(idx int) *model.PdfFont {
	if idx < 0 {
		idx += fontStack.size()
	}
	if idx < 0 || idx > fontStack.size()-1 {
		return nil
	}
	return (*fontStack)[idx]
}

func (fontStack *fontStacker) empty() bool {
	return len(*fontStack) == 0
}

func (fontStack *fontStacker) size() int {
	return len(*fontStack)
}

type textState struct {
	tc    float64
	tw    float64
	th    float64
	tl    float64
	tfs   float64
	tmode RenderMode
	trise float64
	tfont *model.PdfFont

	numChars  int
	numMisses int
}

type textObject struct {
	e         *Extractor
	resources *model.PdfPageResources
	gs        contentstream.GraphicsState
	fontStack *fontStacker
	state     *textState
	tm        transform.Matrix
	tlm       transform.Matrix
	marks     []textMark
}

func newTextState() textState {
	return textState{
		th:    100,
		tmode: RenderModeFill,
	}
}

func newTextObject(e *Extractor, resources *model.PdfPageResources, gs contentstream.GraphicsState,
	state *textState,
	fontStack *fontStacker) *textObject {
	return &textObject{
		e:         e,
		resources: resources,
		gs:        gs,
		fontStack: fontStack,
		state:     state,
		tm:        transform.IdentityMatrix(),
		tlm:       transform.IdentityMatrix(),
	}
}

func (to *textObject) renderText(data []byte) error {

	font := to.getCurrentFont()

	charcodes := font.BytesToCharcodes(data)

	runes, numChars, numMisses := font.CharcodesToUnicodeWithStats(charcodes)
	if numMisses > 0 {
		common.Log.Debug("renderText: numChars=%d numMisses=%d", numChars, numMisses)
	}

	to.state.numChars += numChars
	to.state.numMisses += numMisses

	state := to.state
	tfs := state.tfs
	th := state.th / 100.0
	spaceMetrics, ok := font.GetRuneMetrics(' ')
	if !ok {
		spaceMetrics, ok = font.GetCharMetrics(32)
	}
	if !ok {
		spaceMetrics, _ = model.DefaultFont().GetRuneMetrics(' ')
	}
	spaceWidth := spaceMetrics.Wx * glyphTextRatio
	common.Log.Trace("spaceWidth=%.2f text=%q font=%s fontSize=%.1f", spaceWidth, runes, font, tfs)

	stateMatrix := transform.NewMatrix(
		tfs*th, 0,
		0, tfs,
		0, state.trise)

	common.Log.Trace("renderText: %d codes=%+v runes=%q", len(charcodes), charcodes, runes)

	for i, r := range runes {

		if r == '\x00' {
			continue
		}

		code := charcodes[i]

		trm := to.gs.CTM.Mult(to.tm).Mult(stateMatrix)

		w := 0.0
		if r == ' ' {
			w = state.tw
		}

		m, ok := font.GetCharMetrics(code)
		if !ok {
			common.Log.Debug("ERROR: No metric for code=%d r=0x%04x=%+q %s", code, r, r, font)
			return errors.New("no char metrics")
		}

		c := transform.Point{X: m.Wx * glyphTextRatio, Y: m.Wy * glyphTextRatio}

		t0 := transform.Point{X: (c.X*tfs + w) * th}
		t := transform.Point{X: (c.X*tfs + state.tc + w) * th}

		td0 := translationMatrix(t0)
		td := translationMatrix(t)

		common.Log.Trace("\"%c\" stateMatrix=%s CTM=%s Tm=%s", r, stateMatrix, to.gs.CTM, to.tm)
		common.Log.Trace("tfs=%.3f th=%.3f Tc=%.3f w=%.3f (Tw=%.3f)", tfs, th, state.tc, w, state.tw)
		common.Log.Trace("m=%s c=%+v t0=%+v td0=%s trm0=%s", m, c, t0, td0, td0.Mult(to.tm).Mult(to.gs.CTM))

		mark := to.newTextMark(
			string(r),
			trm,
			translation(to.gs.CTM.Mult(to.tm).Mult(td0)),
			spaceWidth*trm.ScalingFactorX())
		common.Log.Trace("i=%d code=%d mark=%s trm=%s", i, code, mark, trm)
		to.marks = append(to.marks, mark)

		to.tm.Concat(td)
		common.Log.Trace("to.tm=%s", to.tm)
	}

	return nil
}

const glyphTextRatio = 1.0 / 1000.0

func translation(m transform.Matrix) transform.Point {
	tx, ty := m.Translation()
	return transform.Point{X: tx, Y: ty}
}

func translationMatrix(p transform.Point) transform.Matrix {
	return transform.TranslationMatrix(p.X, p.Y)
}

func (to *textObject) moveTo(tx, ty float64) {
	to.tlm.Concat(transform.NewMatrix(1, 0, 0, 1, tx, ty))
	to.tm = to.tlm
}

type textMark struct {
	text          string
	orient        int
	orientedStart transform.Point
	orientedEnd   transform.Point
	height        float64
	spaceWidth    float64
	count         int64
}

func (to *textObject) newTextMark(text string, trm transform.Matrix, end transform.Point, spaceWidth float64) textMark {
	to.e.textCount++
	theta := trm.Angle()
	orient := nearestMultiple(theta, 10)
	var height float64
	if orient%180 != 90 {
		height = trm.ScalingFactorY()
	} else {
		height = trm.ScalingFactorX()
	}

	return textMark{
		text:          text,
		orient:        orient,
		orientedStart: translation(trm).Rotate(theta),
		orientedEnd:   end.Rotate(theta),
		height:        height,
		spaceWidth:    spaceWidth,
		count:         to.e.textCount,
	}
}

func nearestMultiple(x float64, m int) int {
	if m == 0 {
		m = 1
	}
	fac := float64(m)
	return int(math.Round(x/fac) * fac)
}

func (t textMark) String() string {
	return fmt.Sprintf("textMark{@%03d [%.3f,%.3f] %.1f %d° %q}",
		t.count, t.orientedStart.X, t.orientedStart.Y, t.Width(), t.orient, truncate(t.text, 100))
}

func (t textMark) Width() float64 {
	return math.Abs(t.orientedStart.X - t.orientedEnd.X)
}

type PageText struct {
	marks []textMark
}

func (pt PageText) String() string {
	parts := []string{fmt.Sprintf("PageText: %d elements", pt.length())}
	for _, t := range pt.marks {
		parts = append(parts, t.String())
	}
	return strings.Join(parts, "\n")
}

func (pt PageText) length() int {
	return len(pt.marks)
}

func (pt PageText) height() float64 {
	fontHeight := 0.0
	for _, t := range pt.marks {
		if t.height > fontHeight {
			fontHeight = t.height
		}
	}
	return fontHeight
}

func (pt PageText) ToText() string {
	fontHeight := pt.height()

	tol := minFloat(fontHeight*0.2, 5.0)
	common.Log.Trace("ToText: %d elements fontHeight=%.1f tol=%.1f", len(pt.marks), fontHeight, tol)

	pt.sortPosition(tol)

	lines := pt.toLines(tol)
	texts := make([]string, 0, len(lines))
	for _, l := range lines {
		texts = append(texts, l.text)
	}
	return strings.Join(texts, "\n")
}

func (pt *PageText) sortPosition(tol float64) {
	sort.SliceStable(pt.marks, func(i, j int) bool {
		ti, tj := pt.marks[i], pt.marks[j]
		if ti.orient != tj.orient {
			return ti.orient < tj.orient
		}
		if math.Abs(ti.orientedStart.Y-tj.orientedStart.Y) > tol {
			return ti.orientedStart.Y > tj.orientedStart.Y
		}
		return ti.orientedStart.X < tj.orientedStart.X
	})
}

type textLine struct {
	y      float64
	dxList []float64
	text   string
	words  []string
}

func (pt PageText) toLines(tol float64) []textLine {

	tlOrient := make(map[int][]textMark, len(pt.marks))
	for _, t := range pt.marks {
		tlOrient[t.orient] = append(tlOrient[t.orient], t)
	}
	var lines []textLine
	for _, o := range orientKeys(tlOrient) {
		lines = append(lines, PageText{tlOrient[o]}.toLinesOrient(tol)...)
	}
	return lines
}

func (pt PageText) toLinesOrient(tol float64) []textLine {
	if len(pt.marks) == 0 {
		return []textLine{}
	}
	var lines []textLine
	var words []string
	var x []float64
	y := pt.marks[0].orientedStart.Y

	scanning := false

	averageCharWidth := exponAve{}
	wordSpacing := exponAve{}
	lastEndX := 0.0

	for _, t := range pt.marks {
		if t.orientedStart.Y+tol < y {
			if len(words) > 0 {
				line := newLine(y, x, words)
				if averageCharWidth.running {

					line = removeDuplicates(line, averageCharWidth.ave)
				}
				lines = append(lines, line)
			}
			words = []string{}
			x = []float64{}
			y = t.orientedStart.Y
			scanning = false
		}

		deltaSpace := 0.0
		if t.spaceWidth == 0 {
			deltaSpace = math.MaxFloat64
		} else {
			wordSpacing.update(t.spaceWidth)
			deltaSpace = wordSpacing.ave * 0.5
		}
		averageCharWidth.update(t.Width())
		deltaCharWidth := averageCharWidth.ave * 0.3

		isSpace := false
		nextWordX := lastEndX + minFloat(deltaSpace, deltaCharWidth)
		if scanning && t.text != " " {
			isSpace = nextWordX < t.orientedStart.X
		}
		common.Log.Trace("t=%s", t)
		common.Log.Trace("width=%.2f delta=%.2f deltaSpace=%.2g deltaCharWidth=%.2g",
			t.Width(), minFloat(deltaSpace, deltaCharWidth), deltaSpace, deltaCharWidth)
		common.Log.Trace("%+q [%.1f, %.1f] lastEndX=%.2f nextWordX=%.2f (%.2f) isSpace=%t",
			t.text, t.orientedStart.X, t.orientedStart.Y, lastEndX, nextWordX,
			nextWordX-t.orientedStart.X, isSpace)

		if isSpace {
			words = append(words, " ")
			x = append(x, (lastEndX+t.orientedStart.X)*0.5)
		}

		lastEndX = t.orientedEnd.X
		words = append(words, t.text)
		x = append(x, t.orientedStart.X)
		scanning = true
		common.Log.Trace("lastEndX=%.2f", lastEndX)
	}
	if len(words) > 0 {
		line := newLine(y, x, words)
		if averageCharWidth.running {
			line = removeDuplicates(line, averageCharWidth.ave)
		}
		lines = append(lines, line)
	}
	return lines
}

func orientKeys(tlOrient map[int][]textMark) []int {
	keys := []int{}
	for k := range tlOrient {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

type exponAve struct {
	ave     float64
	running bool
}

func (exp *exponAve) update(x float64) float64 {
	if !exp.running {
		exp.ave = x
		exp.running = true
	} else {

		exp.ave = (exp.ave + x) * 0.5
	}
	return exp.ave
}

func newLine(y float64, x []float64, words []string) textLine {
	dxList := make([]float64, 0, len(x))
	for i := 1; i < len(x); i++ {
		dxList = append(dxList, x[i]-x[i-1])
	}
	return textLine{y: y, dxList: dxList, text: strings.Join(words, ""), words: words}
}

func removeDuplicates(line textLine, charWidth float64) textLine {
	if len(line.dxList) == 0 {
		return line
	}

	tol := charWidth * 0.3
	words := []string{line.words[0]}
	var dxList []float64

	w0 := line.words[0]
	for i, dx := range line.dxList {
		w := line.words[i+1]
		if w != w0 || dx > tol {
			words = append(words, w)
			dxList = append(dxList, dx)
		}
		w0 = w
	}
	return textLine{y: line.y, dxList: dxList, text: strings.Join(words, ""), words: words}
}

func combineDiacritics(line textLine, charWidth float64) textLine {
	if len(line.dxList) == 0 {
		return line
	}

	tol := charWidth * 0.2
	common.Log.Trace("combineDiacritics: charWidth=%.2f tol=%.2f", charWidth, tol)

	var words []string
	var dxList []float64
	w := line.words[0]
	w, c := countDiacritic(w)
	delta := 0.0
	dx0 := 0.0
	parts := []string{w}
	numChars := c

	for i := 0; i < len(line.dxList); i++ {
		w = line.words[i+1]
		w, c := countDiacritic(w)
		dx := line.dxList[i]
		if numChars+c <= 1 && delta+dx <= tol {
			if len(parts) == 0 {
				dx0 = dx
			} else {
				delta += dx
			}
			parts = append(parts, w)
			numChars += c
		} else {
			if len(parts) > 0 {
				if len(words) > 0 {
					dxList = append(dxList, dx0)
				}
				words = append(words, combine(parts))
			}
			parts = []string{w}
			numChars = c
			dx0 = dx
			delta = 0.0
		}
	}
	if len(parts) > 0 {
		if len(words) > 0 {
			dxList = append(dxList, dx0)
		}
		words = append(words, combine(parts))
	}

	if len(words) != len(dxList)+1 {
		common.Log.Error("Inconsistent: \nwords=%d %q\ndxList=%d %.2f",
			len(words), words, len(dxList), dxList)
		return line
	}
	return textLine{y: line.y, dxList: dxList, text: strings.Join(words, ""), words: words}
}

func combine(parts []string) string {
	if len(parts) == 1 {

		return parts[0]
	}

	diacritic := map[string]bool{}
	for _, w := range parts {
		r := []rune(w)[0]
		diacritic[w] = unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Sk, r)
	}
	sort.SliceStable(parts, func(i, j int) bool { return !diacritic[parts[i]] && diacritic[parts[j]] })

	for i, w := range parts {
		parts[i] = strings.TrimSpace(norm.NFKC.String(w))
	}
	return strings.Join(parts, "")
}

func countDiacritic(w string) (string, int) {
	runes := []rune(w)
	if len(runes) != 1 {
		return w, 1
	}
	r := runes[0]
	c := 1
	if (unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Sk, r)) &&
		r != '\'' && r != '"' && r != '`' {
		c = 0
	}
	if w2, ok := diacritics[r]; ok {
		c = 0
		w = w2
	}
	return w, c
}

var diacritics = map[rune]string{
	0x0060: "\u0300",
	0x02CB: "\u0300",
	0x0027: "\u0301",
	0x02B9: "\u0301",
	0x02CA: "\u0301",
	0x005e: "\u0302",
	0x02C6: "\u0302",
	0x007E: "\u0303",
	0x02C9: "\u0304",
	0x00B0: "\u030A",
	0x02BA: "\u030B",
	0x02C7: "\u030C",
	0x02C8: "\u030D",
	0x0022: "\u030E",
	0x02BB: "\u0312",
	0x02BC: "\u0313",
	0x0486: "\u0313",
	0x055A: "\u0313",
	0x02BD: "\u0314",
	0x0485: "\u0314",
	0x0559: "\u0314",
	0x02D4: "\u031D",
	0x02D5: "\u031E",
	0x02D6: "\u031F",
	0x02D7: "\u0320",
	0x02B2: "\u0321",
	0x02CC: "\u0329",
	0x02B7: "\u032B",
	0x02CD: "\u0331",
	0x005F: "\u0332",
	0x204E: "\u0359",
}

func (to *textObject) getCurrentFont() *model.PdfFont {
	if to.fontStack.empty() {
		common.Log.Debug("ERROR: No font defined. Using default.")
		return model.DefaultFont()
	}
	return to.fontStack.peek()
}

func (to *textObject) getFont(name string) (*model.PdfFont, error) {
	if to.e.fontCache != nil {
		to.e.accessCount++
		entry, ok := to.e.fontCache[name]
		if ok {
			entry.access = to.e.accessCount
			return entry.font, nil
		}
	}

	font, err := to.getFontDirect(name)
	if err != nil {
		return nil, err
	}

	if to.e.fontCache != nil {
		entry := fontEntry{font, to.e.accessCount}

		if len(to.e.fontCache) >= maxFontCache {
			var names []string
			for name := range to.e.fontCache {
				names = append(names, name)
			}
			sort.Slice(names, func(i, j int) bool {
				return to.e.fontCache[names[i]].access < to.e.fontCache[names[j]].access
			})
			delete(to.e.fontCache, names[0])
		}
		to.e.fontCache[name] = entry
	}

	return font, nil
}

type fontEntry struct {
	font   *model.PdfFont
	access int64
}

const maxFontCache = 10

func (to *textObject) getFontDirect(name string) (*model.PdfFont, error) {
	fontObj, err := to.getFontDict(name)
	if err != nil {
		return nil, err
	}
	font, err := model.NewPdfFontFromPdfObject(fontObj)
	if err != nil {
		common.Log.Debug("getFontDirect: NewPdfFontFromPdfObject failed. name=%#q err=%v", name, err)
	}
	return font, err
}

func (to *textObject) getFontDict(name string) (fontObj core.PdfObject, err error) {
	resources := to.resources
	if resources == nil {
		common.Log.Debug("getFontDict. No resources. name=%#q", name)
		return nil, nil
	}
	fontObj, found := resources.GetFontByName(core.PdfObjectName(name))
	if !found {
		common.Log.Debug("ERROR: getFontDict: Font not found: name=%#q", name)
		return nil, errors.New("font not in resources")
	}
	return fontObj, nil
}
