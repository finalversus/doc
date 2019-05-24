package creator

import (
	"strings"

	"github.com/finalversus/doc/pdf/model"
)

type TOCLine struct {
	sp *StyledParagraph

	Number TextChunk

	Title TextChunk

	Separator TextChunk

	Page TextChunk

	offset float64

	level uint

	levelOffset float64

	positioning positioning

	linkX    float64
	linkY    float64
	linkPage int64
}

func newTOCLine(number, title, page string, level uint, style TextStyle) *TOCLine {
	return newStyledTOCLine(
		TextChunk{
			Text:  number,
			Style: style,
		},
		TextChunk{
			Text:  title,
			Style: style,
		},
		TextChunk{
			Text:  page,
			Style: style,
		},
		level,
		style,
	)
}

func newStyledTOCLine(number, title, page TextChunk, level uint, style TextStyle) *TOCLine {
	sp := newStyledParagraph(style)
	sp.SetEnableWrap(true)
	sp.SetTextAlignment(TextAlignmentLeft)
	sp.SetMargins(0, 0, 2, 2)

	tl := &TOCLine{
		sp:     sp,
		Number: number,
		Title:  title,
		Page:   page,
		Separator: TextChunk{
			Text:  ".",
			Style: style,
		},
		offset:      0,
		level:       level,
		levelOffset: 10,
		positioning: positionRelative,
	}

	sp.margins.left = tl.offset + float64(tl.level-1)*tl.levelOffset
	sp.beforeRender = tl.prepareParagraph
	return tl
}

func (tl *TOCLine) SetStyle(style TextStyle) {
	tl.Number.Style = style
	tl.Title.Style = style
	tl.Separator.Style = style
	tl.Page.Style = style
}

func (tl *TOCLine) Level() uint {
	return tl.level
}

func (tl *TOCLine) SetLevel(level uint) {
	tl.level = level
	tl.sp.margins.left = tl.offset + float64(tl.level-1)*tl.levelOffset
}

func (tl *TOCLine) LevelOffset() float64 {
	return tl.levelOffset
}

func (tl *TOCLine) SetLevelOffset(levelOffset float64) {
	tl.levelOffset = levelOffset
	tl.sp.margins.left = tl.offset + float64(tl.level-1)*tl.levelOffset
}

func (tl *TOCLine) GetMargins() (float64, float64, float64, float64) {
	m := &tl.sp.margins
	return tl.offset, m.right, m.top, m.bottom
}

func (tl *TOCLine) SetMargins(left, right, top, bottom float64) {
	tl.offset = left

	m := &tl.sp.margins
	m.left = tl.offset + float64(tl.level-1)*tl.levelOffset
	m.right = right
	m.top = top
	m.bottom = bottom
}

func (tl *TOCLine) SetLink(page int64, x, y float64) {
	tl.linkX = x
	tl.linkY = y
	tl.linkPage = page

	tl.SetStyle(tl.sp.defaultLinkStyle)
}

func (tl *TOCLine) getLineLink() *model.PdfAnnotation {
	if tl.linkPage <= 0 {
		return nil
	}

	return newInternalLinkAnnotation(tl.linkPage-1, tl.linkX, tl.linkY, 0)
}

func (tl *TOCLine) prepareParagraph(sp *StyledParagraph, ctx DrawContext) {

	title := tl.Title.Text
	if tl.Number.Text != "" {
		title = " " + title
	}
	title += " "

	page := tl.Page.Text
	if page != "" {
		page = " " + page
	}

	sp.chunks = []*TextChunk{
		{
			Text:       tl.Number.Text,
			Style:      tl.Number.Style,
			annotation: tl.getLineLink(),
		},
		{
			Text:       title,
			Style:      tl.Title.Style,
			annotation: tl.getLineLink(),
		},
		{
			Text:       page,
			Style:      tl.Page.Style,
			annotation: tl.getLineLink(),
		},
	}

	sp.wrapText()

	l := len(sp.lines)
	if l == 0 {
		return
	}

	availWidth := ctx.Width*1000 - sp.getTextLineWidth(sp.lines[l-1])
	sepWidth := sp.getTextLineWidth([]*TextChunk{&tl.Separator})
	sepCount := int(availWidth / sepWidth)
	sepText := strings.Repeat(tl.Separator.Text, sepCount)
	sepStyle := tl.Separator.Style

	chunk := sp.Insert(2, sepText)
	chunk.Style = sepStyle
	chunk.annotation = tl.getLineLink()

	availWidth = availWidth - float64(sepCount)*sepWidth
	if availWidth > 500 {
		spaceMetrics, found := sepStyle.Font.GetRuneMetrics(' ')
		if found && availWidth > spaceMetrics.Wx {
			spaces := int(availWidth / spaceMetrics.Wx)
			if spaces > 0 {
				style := sepStyle
				style.FontSize = 1

				chunk = sp.Insert(2, strings.Repeat(" ", spaces))
				chunk.Style = style
				chunk.annotation = tl.getLineLink()
			}
		}
	}
}

func (tl *TOCLine) GeneratePageBlocks(ctx DrawContext) ([]*Block, DrawContext, error) {
	origCtx := ctx

	blocks, ctx, err := tl.sp.GeneratePageBlocks(ctx)
	if err != nil {
		return blocks, ctx, err
	}

	if tl.positioning.isRelative() {

		ctx.X = origCtx.X
	}

	if tl.positioning.isAbsolute() {

		return blocks, origCtx, nil
	}

	return blocks, ctx, nil
}
