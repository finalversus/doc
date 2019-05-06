package creator

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/codefinio/doc/common"
	"github.com/codefinio/doc/pdf/contentstream"
	"github.com/codefinio/doc/pdf/contentstream/draw"
	"github.com/codefinio/doc/pdf/core"
	"github.com/codefinio/doc/pdf/model"
)

type StyledParagraph struct {
	chunks []*TextChunk

	defaultStyle TextStyle

	defaultLinkStyle TextStyle

	alignment TextAlignment

	lineHeight float64

	enableWrap bool
	wrapWidth  float64

	defaultWrap bool

	angle float64

	margins margins

	positioning positioning

	xPos float64
	yPos float64

	scaleX float64
	scaleY float64

	lines [][]*TextChunk

	beforeRender func(p *StyledParagraph, ctx DrawContext)
}

func newStyledParagraph(style TextStyle) *StyledParagraph {

	return &StyledParagraph{
		chunks:           []*TextChunk{},
		defaultStyle:     style,
		defaultLinkStyle: newLinkStyle(style.Font),
		lineHeight:       1.0,
		alignment:        TextAlignmentLeft,
		enableWrap:       true,
		defaultWrap:      true,
		angle:            0,
		scaleX:           1,
		scaleY:           1,
		positioning:      positionRelative,
	}
}

func (p *StyledParagraph) appendChunk(chunk *TextChunk) *TextChunk {
	p.chunks = append(p.chunks, chunk)
	p.wrapText()
	return chunk
}

func (p *StyledParagraph) Append(text string) *TextChunk {
	chunk := newTextChunk(text, p.defaultStyle)
	return p.appendChunk(chunk)
}

func (p *StyledParagraph) Insert(index uint, text string) *TextChunk {
	l := uint(len(p.chunks))
	if index > l {
		index = l
	}

	chunk := newTextChunk(text, p.defaultStyle)
	p.chunks = append(p.chunks[:index], append([]*TextChunk{chunk}, p.chunks[index:]...)...)
	p.wrapText()

	return chunk
}

func (p *StyledParagraph) AddExternalLink(text, url string) *TextChunk {
	chunk := newTextChunk(text, p.defaultLinkStyle)
	chunk.annotation = newExternalLinkAnnotation(url)
	return p.appendChunk(chunk)
}

func (p *StyledParagraph) AddInternalLink(text string, page int64, x, y, zoom float64) *TextChunk {
	chunk := newTextChunk(text, p.defaultLinkStyle)
	chunk.annotation = newInternalLinkAnnotation(page-1, x, y, zoom)
	return p.appendChunk(chunk)
}

func (p *StyledParagraph) Reset() {
	p.chunks = []*TextChunk{}
}

func (p *StyledParagraph) SetText(text string) *TextChunk {
	p.Reset()
	return p.Append(text)
}

func (p *StyledParagraph) SetTextAlignment(align TextAlignment) {
	p.alignment = align
}

func (p *StyledParagraph) SetLineHeight(lineheight float64) {
	p.lineHeight = lineheight
}

func (p *StyledParagraph) SetEnableWrap(enableWrap bool) {
	p.enableWrap = enableWrap
	p.defaultWrap = false
}

func (p *StyledParagraph) SetPos(x, y float64) {
	p.positioning = positionAbsolute
	p.xPos = x
	p.yPos = y
}

func (p *StyledParagraph) SetAngle(angle float64) {
	p.angle = angle
}

func (p *StyledParagraph) SetMargins(left, right, top, bottom float64) {
	p.margins.left = left
	p.margins.right = right
	p.margins.top = top
	p.margins.bottom = bottom
}

func (p *StyledParagraph) GetMargins() (float64, float64, float64, float64) {
	return p.margins.left, p.margins.right, p.margins.top, p.margins.bottom
}

func (p *StyledParagraph) SetWidth(width float64) {
	p.wrapWidth = width
	p.wrapText()
}

func (p *StyledParagraph) Width() float64 {
	if p.enableWrap && int(p.wrapWidth) > 0 {
		return p.wrapWidth
	}

	return p.getTextWidth() / 1000.0
}

func (p *StyledParagraph) Height() float64 {
	if p.lines == nil || len(p.lines) == 0 {
		p.wrapText()
	}

	var height float64
	for _, line := range p.lines {
		var lineHeight float64
		for _, chunk := range line {
			h := p.lineHeight * chunk.Style.FontSize
			if h > lineHeight {
				lineHeight = h
			}
		}

		height += lineHeight
	}

	return height
}

func (p *StyledParagraph) getTextWidth() float64 {
	var width float64
	lenChunks := len(p.chunks)

	for i, chunk := range p.chunks {
		style := &chunk.Style
		lenRunes := len(chunk.Text)

		for j, r := range chunk.Text {

			if r == '\u000A' {
				continue
			}

			metrics, found := style.Font.GetRuneMetrics(r)
			if !found {
				common.Log.Debug("Rune char metrics not found! %v\n", r)

				return -1
			}

			width += style.FontSize * metrics.Wx

			if i != lenChunks-1 || j != lenRunes-1 {
				width += style.CharSpacing * 1000.0
			}
		}
	}

	return width
}

func (p *StyledParagraph) getTextLineWidth(line []*TextChunk) float64 {
	var width float64
	lenChunks := len(line)

	for i, chunk := range line {
		style := &chunk.Style
		lenRunes := len(chunk.Text)

		for j, r := range chunk.Text {

			if r == '\u000A' {
				continue
			}

			metrics, found := style.Font.GetRuneMetrics(r)
			if !found {
				common.Log.Debug("Rune char metrics not found! %v\n", r)

				return -1
			}

			width += style.FontSize * metrics.Wx

			if i != lenChunks-1 || j != lenRunes-1 {
				width += style.CharSpacing * 1000.0
			}
		}
	}

	return width
}

func (p *StyledParagraph) getMaxLineWidth() float64 {
	if p.lines == nil || len(p.lines) == 0 {
		p.wrapText()
	}

	var width float64
	for _, line := range p.lines {
		w := p.getTextLineWidth(line)
		if w > width {
			width = w
		}
	}

	return width
}

func (p *StyledParagraph) getTextHeight() float64 {
	var height float64
	for _, chunk := range p.chunks {
		h := chunk.Style.FontSize * p.lineHeight
		if h > height {
			height = h
		}
	}

	return height
}

func (p *StyledParagraph) wrapText() error {
	if !p.enableWrap || int(p.wrapWidth) <= 0 {
		p.lines = [][]*TextChunk{p.chunks}
		return nil
	}

	p.lines = [][]*TextChunk{}
	var line []*TextChunk
	var lineWidth float64

	copyAnnotation := func(src *model.PdfAnnotation) *model.PdfAnnotation {
		if src == nil {
			return nil
		}

		var annotation *model.PdfAnnotation
		switch t := src.GetContext().(type) {
		case *model.PdfAnnotationLink:
			if annot := copyLinkAnnotation(t); annot != nil {
				annotation = annot.PdfAnnotation
			}
		}

		return annotation
	}

	for _, chunk := range p.chunks {
		style := chunk.Style
		annotation := chunk.annotation

		var (
			part   []rune
			widths []float64
		)

		for _, r := range chunk.Text {

			if r == '\u000A' {

				line = append(line, &TextChunk{
					Text:       strings.TrimRightFunc(string(part), unicode.IsSpace),
					Style:      style,
					annotation: copyAnnotation(annotation),
				})
				p.lines = append(p.lines, line)
				line = nil

				lineWidth = 0
				part = nil
				widths = nil
				continue
			}

			metrics, found := style.Font.GetRuneMetrics(r)
			if !found {
				common.Log.Debug("Rune char metrics not found! %v\n", r)
				return errors.New("glyph char metrics missing")
			}

			w := style.FontSize * metrics.Wx
			charWidth := w + style.CharSpacing*1000.0

			if lineWidth+w > p.wrapWidth*1000.0 {

				idx := -1
				for j := len(part) - 1; j >= 0; j-- {
					if part[j] == ' ' {
						idx = j
						break
					}
				}

				text := string(part)
				if idx >= 0 {
					text = string(part[0 : idx+1])

					part = part[idx+1:]
					part = append(part, r)
					widths = widths[idx+1:]
					widths = append(widths, charWidth)

					lineWidth = 0
					for _, width := range widths {
						lineWidth += width
					}
				} else {
					lineWidth = charWidth
					part = []rune{r}
					widths = []float64{charWidth}
				}

				line = append(line, &TextChunk{
					Text:       strings.TrimRightFunc(string(text), unicode.IsSpace),
					Style:      style,
					annotation: copyAnnotation(annotation),
				})
				p.lines = append(p.lines, line)
				line = []*TextChunk{}
			} else {
				lineWidth += charWidth
				part = append(part, r)
				widths = append(widths, charWidth)
			}
		}

		if len(part) > 0 {
			line = append(line, &TextChunk{
				Text:       string(part),
				Style:      style,
				annotation: copyAnnotation(annotation),
			})
		}
	}

	if len(line) > 0 {
		p.lines = append(p.lines, line)
	}

	return nil
}

func (p *StyledParagraph) GeneratePageBlocks(ctx DrawContext) ([]*Block, DrawContext, error) {
	origContext := ctx
	var blocks []*Block

	blk := NewBlock(ctx.PageWidth, ctx.PageHeight)
	if p.positioning.isRelative() {

		ctx.X += p.margins.left
		ctx.Y += p.margins.top
		ctx.Width -= p.margins.left + p.margins.right
		ctx.Height -= p.margins.top + p.margins.bottom

		p.SetWidth(ctx.Width)

		if p.Height() > ctx.Height {

			blocks = append(blocks, blk)
			blk = NewBlock(ctx.PageWidth, ctx.PageHeight)

			ctx.Page++
			newContext := ctx
			newContext.Y = ctx.Margins.top
			newContext.X = ctx.Margins.left + p.margins.left
			newContext.Height = ctx.PageHeight - ctx.Margins.top - ctx.Margins.bottom - p.margins.bottom
			newContext.Width = ctx.PageWidth - ctx.Margins.left - ctx.Margins.right - p.margins.left - p.margins.right
			ctx = newContext
		}
	} else {

		if int(p.wrapWidth) <= 0 {

			p.SetWidth(p.getTextWidth())
		}
		ctx.X = p.xPos
		ctx.Y = p.yPos
	}

	if p.beforeRender != nil {
		p.beforeRender(p, ctx)
	}

	ctx, err := drawStyledParagraphOnBlock(blk, p, ctx)
	if err != nil {
		common.Log.Debug("ERROR: %v", err)
		return nil, ctx, err
	}

	blocks = append(blocks, blk)
	if p.positioning.isRelative() {
		ctx.X -= p.margins.left
		ctx.Width = origContext.Width
		return blocks, ctx, nil
	}

	return blocks, origContext, nil
}

func drawStyledParagraphOnBlock(blk *Block, p *StyledParagraph, ctx DrawContext) (DrawContext, error) {

	num := 1
	fontName := core.PdfObjectName(fmt.Sprintf("Font%d", num))
	for blk.resources.HasFontByName(fontName) {
		num++
		fontName = core.PdfObjectName(fmt.Sprintf("Font%d", num))
	}

	err := blk.resources.SetFontByName(fontName, p.defaultStyle.Font.ToPdfObject())
	if err != nil {
		return ctx, err
	}
	num++

	defaultFontName := fontName
	defaultFontSize := p.defaultStyle.FontSize

	p.wrapText()

	var fonts [][]core.PdfObjectName

	for _, line := range p.lines {
		var fontLine []core.PdfObjectName

		for _, chunk := range line {
			fontName = core.PdfObjectName(fmt.Sprintf("Font%d", num))

			err := blk.resources.SetFontByName(fontName, chunk.Style.Font.ToPdfObject())
			if err != nil {
				return ctx, err
			}

			fontLine = append(fontLine, fontName)
			num++
		}

		fonts = append(fonts, fontLine)
	}

	cc := contentstream.NewContentCreator()
	cc.Add_q()

	yPos := ctx.PageHeight - ctx.Y - defaultFontSize*p.lineHeight
	cc.Translate(ctx.X, yPos)

	if p.angle != 0 {
		cc.RotateDeg(p.angle)
	}

	cc.Add_BT()

	currY := yPos
	for idx, line := range p.lines {
		currX := ctx.X

		if idx != 0 {

			cc.Add_Tstar()
		}

		isLastLine := idx == len(p.lines)-1

		var (
			width      float64
			height     float64
			spaceWidth float64
			spaces     uint
		)

		var chunkWidths []float64
		for _, chunk := range line {
			style := &chunk.Style

			if style.FontSize > height {
				height = style.FontSize
			}

			spaceMetrics, found := style.Font.GetRuneMetrics(' ')
			if !found {
				return ctx, errors.New("the font does not have a space glyph")
			}

			var chunkSpaces uint
			var chunkWidth float64
			lenChunk := len(chunk.Text)
			for i, r := range chunk.Text {
				if r == ' ' {
					chunkSpaces++
					continue
				}
				if r == '\u000A' {
					continue
				}

				metrics, found := style.Font.GetRuneMetrics(r)
				if !found {
					common.Log.Debug("Unsupported rune %v in font\n", r)
					return ctx, errors.New("unsupported text glyph")
				}

				chunkWidth += style.FontSize * metrics.Wx

				if i != lenChunk-1 {
					chunkWidth += style.CharSpacing * 1000.0
				}
			}

			chunkWidths = append(chunkWidths, chunkWidth)
			width += chunkWidth

			spaceWidth += float64(chunkSpaces) * spaceMetrics.Wx * style.FontSize
			spaces += chunkSpaces
		}
		height *= p.lineHeight

		var objs []core.PdfObject

		wrapWidth := p.wrapWidth * 1000.0
		if p.alignment == TextAlignmentJustify {

			if spaces > 0 && !isLastLine {
				spaceWidth = (wrapWidth - width) / float64(spaces) / defaultFontSize
			}
		} else if p.alignment == TextAlignmentCenter {

			offset := (wrapWidth - width - spaceWidth) / 2
			shift := offset / defaultFontSize
			objs = append(objs, core.MakeFloat(-shift))

			currX += offset / 1000.0
		} else if p.alignment == TextAlignmentRight {

			offset := (wrapWidth - width - spaceWidth)
			shift := offset / defaultFontSize
			objs = append(objs, core.MakeFloat(-shift))

			currX += offset / 1000.0
		}

		if len(objs) > 0 {
			cc.Add_Tf(defaultFontName, defaultFontSize).
				Add_TL(defaultFontSize * p.lineHeight).
				Add_TJ(objs...)
		}

		for k, chunk := range line {
			style := &chunk.Style

			r, g, b := style.Color.ToRGB()
			fontName := defaultFontName
			fontSize := defaultFontSize

			cc.Add_Tr(int64(style.RenderingMode))

			cc.Add_Tc(style.CharSpacing)

			if p.alignment != TextAlignmentJustify || isLastLine {
				spaceMetrics, found := style.Font.GetRuneMetrics(' ')
				if !found {
					return ctx, errors.New("the font does not have a space glyph")
				}

				fontName = fonts[idx][k]
				fontSize = style.FontSize
				spaceWidth = spaceMetrics.Wx
			}
			enc := style.Font.Encoder()

			var encStr []byte
			for _, rn := range chunk.Text {
				if rn == ' ' {
					if len(encStr) > 0 {
						cc.Add_rg(r, g, b).
							Add_Tf(fonts[idx][k], style.FontSize).
							Add_TL(style.FontSize * p.lineHeight).
							Add_TJ([]core.PdfObject{core.MakeStringFromBytes(encStr)}...)

						encStr = nil
					}

					cc.Add_Tf(fontName, fontSize).
						Add_TL(fontSize * p.lineHeight).
						Add_TJ([]core.PdfObject{core.MakeFloat(-spaceWidth)}...)

					chunkWidths[k] += spaceWidth * fontSize
				} else {
					encStr = append(encStr, enc.Encode(string(rn))...)
				}
			}

			if len(encStr) > 0 {
				cc.Add_rg(r, g, b).
					Add_Tf(fonts[idx][k], style.FontSize).
					Add_TL(style.FontSize * p.lineHeight).
					Add_TJ([]core.PdfObject{core.MakeStringFromBytes(encStr)}...)
			}

			chunkWidth := chunkWidths[k] / 1000.0

			if chunk.annotation != nil {
				var annotRect *core.PdfObjectArray

				if !chunk.annotationProcessed {
					switch t := chunk.annotation.GetContext().(type) {
					case *model.PdfAnnotationLink:

						annotRect = core.MakeArray()
						t.Rect = annotRect

						annotDest, ok := t.Dest.(*core.PdfObjectArray)
						if ok && annotDest.Len() == 5 {
							t, ok := annotDest.Get(1).(*core.PdfObjectName)
							if ok && t.String() == "XYZ" {
								y, err := core.GetNumberAsFloat(annotDest.Get(3))
								if err == nil {
									annotDest.Set(3, core.MakeFloat(ctx.PageHeight-y))
								}
							}
						}
					}

					chunk.annotationProcessed = true
				}

				if annotRect != nil {

					annotPos := draw.NewPoint(currX-ctx.X, currY-yPos).Rotate(p.angle)
					annotPos.X += ctx.X
					annotPos.Y += yPos

					offX, offY, annotW, annotH := rotateRect(chunkWidth, height, p.angle)
					annotPos.X += offX
					annotPos.Y += offY

					annotRect.Clear()
					annotRect.Append(core.MakeFloat(annotPos.X))
					annotRect.Append(core.MakeFloat(annotPos.Y))
					annotRect.Append(core.MakeFloat(annotPos.X + annotW))
					annotRect.Append(core.MakeFloat(annotPos.Y + annotH))
				}

				blk.AddAnnotation(chunk.annotation)
			}

			currX += chunkWidth

			cc.Add_Tr(int64(TextRenderingModeFill))

			cc.Add_Tc(0)
		}

		currY -= height
	}
	cc.Add_ET()
	cc.Add_Q()

	ops := cc.Operations()
	ops.WrapIfNeeded()

	blk.addContents(ops)

	if p.positioning.isRelative() {
		pHeight := p.Height() + p.margins.bottom
		ctx.Y += pHeight
		ctx.Height -= pHeight

		if ctx.Inline {
			ctx.X += p.Width() + p.margins.right
		}
	}

	return ctx, nil
}
