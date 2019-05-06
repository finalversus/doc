package creator

type TOC struct {
	heading *StyledParagraph

	lines []*TOCLine

	lineNumberStyle TextStyle

	lineTitleStyle TextStyle

	lineSeparatorStyle TextStyle

	linePageStyle TextStyle

	lineSeparator string

	lineLevelOffset float64

	lineMargins margins

	positioning positioning

	defaultStyle TextStyle

	showLinks bool
}

func newTOC(title string, style, styleHeading TextStyle) *TOC {
	headingStyle := styleHeading
	headingStyle.FontSize = 14

	heading := newStyledParagraph(headingStyle)
	heading.SetEnableWrap(true)
	heading.SetTextAlignment(TextAlignmentLeft)
	heading.SetMargins(0, 0, 0, 5)

	chunk := heading.Append(title)
	chunk.Style = headingStyle

	return &TOC{
		heading:            heading,
		lines:              []*TOCLine{},
		lineNumberStyle:    style,
		lineTitleStyle:     style,
		lineSeparatorStyle: style,
		linePageStyle:      style,
		lineSeparator:      ".",
		lineLevelOffset:    10,
		lineMargins:        margins{0, 0, 2, 2},
		positioning:        positionRelative,
		defaultStyle:       style,
		showLinks:          true,
	}
}

func (t *TOC) Heading() *StyledParagraph {
	return t.heading
}

func (t *TOC) Lines() []*TOCLine {
	return t.lines
}

func (t *TOC) SetHeading(text string, style TextStyle) {
	h := t.Heading()

	h.Reset()
	chunk := h.Append(text)
	chunk.Style = style
}

func (t *TOC) Add(number, title, page string, level uint) *TOCLine {
	tl := t.AddLine(newStyledTOCLine(
		TextChunk{
			Text:  number,
			Style: t.lineNumberStyle,
		},
		TextChunk{
			Text:  title,
			Style: t.lineTitleStyle,
		},
		TextChunk{
			Text:  page,
			Style: t.linePageStyle,
		},
		level,
		t.defaultStyle,
	))

	if tl == nil {
		return nil
	}

	m := &t.lineMargins
	tl.SetMargins(m.left, m.right, m.top, m.bottom)

	tl.SetLevelOffset(t.lineLevelOffset)

	tl.Separator.Text = t.lineSeparator
	tl.Separator.Style = t.lineSeparatorStyle

	return tl
}

func (t *TOC) AddLine(line *TOCLine) *TOCLine {
	if line == nil {
		return nil
	}

	t.lines = append(t.lines, line)
	return line
}

func (t *TOC) SetLineSeparator(separator string) {
	t.lineSeparator = separator
}

func (t *TOC) SetLineMargins(left, right, top, bottom float64) {
	m := &t.lineMargins

	m.left = left
	m.right = right
	m.top = top
	m.bottom = bottom
}

func (t *TOC) SetLineStyle(style TextStyle) {
	t.SetLineNumberStyle(style)
	t.SetLineTitleStyle(style)
	t.SetLineSeparatorStyle(style)
	t.SetLinePageStyle(style)
}

func (t *TOC) SetLineNumberStyle(style TextStyle) {
	t.lineNumberStyle = style
}

func (t *TOC) SetLineTitleStyle(style TextStyle) {
	t.lineTitleStyle = style
}

func (t *TOC) SetLineSeparatorStyle(style TextStyle) {
	t.lineSeparatorStyle = style
}

func (t *TOC) SetLinePageStyle(style TextStyle) {
	t.linePageStyle = style
}

func (t *TOC) SetLineLevelOffset(levelOffset float64) {
	t.lineLevelOffset = levelOffset
}

func (t *TOC) SetShowLinks(showLinks bool) {
	t.showLinks = showLinks
}

func (t *TOC) GeneratePageBlocks(ctx DrawContext) ([]*Block, DrawContext, error) {
	origCtx := ctx

	blocks, ctx, err := t.heading.GeneratePageBlocks(ctx)
	if err != nil {
		return blocks, ctx, err
	}

	for _, line := range t.lines {
		linkPage := line.linkPage
		if !t.showLinks {
			line.linkPage = 0
		}

		newBlocks, c, err := line.GeneratePageBlocks(ctx)
		line.linkPage = linkPage

		if err != nil {
			return blocks, ctx, err
		}
		if len(newBlocks) < 1 {
			continue
		}

		blocks[len(blocks)-1].mergeBlocks(newBlocks[0])
		blocks = append(blocks, newBlocks[1:]...)

		ctx = c
	}

	if t.positioning.isRelative() {

		ctx.X = origCtx.X
	}

	if t.positioning.isAbsolute() {

		return blocks, origCtx, nil

	}

	return blocks, ctx, nil
}
