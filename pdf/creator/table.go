package creator

import (
	"errors"
	"math"
	"sort"

	"github.com/finalversus/doc/common"
	"github.com/finalversus/doc/pdf/contentstream/draw"
	"github.com/finalversus/doc/pdf/core"
	"github.com/finalversus/doc/pdf/model"
)

type Table struct {
	rows int
	cols int

	curCell int

	colWidths []float64

	rowHeights []float64

	defaultRowHeight float64

	cells []*TableCell

	positioning positioning

	xPos, yPos float64

	margins margins

	hasHeader bool

	headerStartRow int
	headerEndRow   int
}

func newTable(cols int) *Table {
	t := &Table{
		cols:             cols,
		defaultRowHeight: 10.0,
		colWidths:        []float64{},
		rowHeights:       []float64{},
		cells:            []*TableCell{},
	}

	t.resetColumnWidths()
	return t
}

func (table *Table) SetColumnWidths(widths ...float64) error {
	if len(widths) != table.cols {
		common.Log.Debug("Mismatching number of widths and columns")
		return errors.New("range check error")
	}

	table.colWidths = widths
	return nil
}

func (table *Table) resetColumnWidths() {
	table.colWidths = []float64{}
	colWidth := float64(1.0) / float64(table.cols)

	for i := 0; i < table.cols; i++ {
		table.colWidths = append(table.colWidths, colWidth)
	}
}

func (table *Table) Height() float64 {
	sum := float64(0.0)
	for _, h := range table.rowHeights {
		sum += h
	}

	return sum
}

func (table *Table) Width() float64 {
	return 0
}

func (table *Table) SetMargins(left, right, top, bottom float64) {
	table.margins.left = left
	table.margins.right = right
	table.margins.top = top
	table.margins.bottom = bottom
}

func (table *Table) GetMargins() (float64, float64, float64, float64) {
	return table.margins.left, table.margins.right, table.margins.top, table.margins.bottom
}

func (table *Table) GetRowHeight(row int) (float64, error) {
	if row < 1 || row > len(table.rowHeights) {
		return 0, errors.New("range check error")
	}

	return table.rowHeights[row-1], nil
}

func (table *Table) SetRowHeight(row int, h float64) error {
	if row < 1 || row > len(table.rowHeights) {
		return errors.New("range check error")
	}

	table.rowHeights[row-1] = h
	return nil
}

func (table *Table) Rows() int {
	return table.rows
}

func (table *Table) Cols() int {
	return table.cols
}

func (table *Table) CurRow() int {
	curRow := (table.curCell-1)/table.cols + 1
	return curRow
}

func (table *Table) CurCol() int {
	curCol := (table.curCell-1)%(table.cols) + 1
	return curCol
}

func (table *Table) SetPos(x, y float64) {
	table.positioning = positionAbsolute
	table.xPos = x
	table.yPos = y
}

func (table *Table) SetHeaderRows(startRow, endRow int) error {
	if startRow <= 0 {
		return errors.New("header start row must be greater than 0")
	}
	if endRow <= 0 {
		return errors.New("header end row must be greater than 0")
	}
	if startRow > endRow {
		return errors.New("header start row  must be less than or equal to the end row")
	}

	table.hasHeader = true
	table.headerStartRow = startRow
	table.headerEndRow = endRow
	return nil
}

func (table *Table) AddSubtable(row, col int, subtable *Table) {
	for _, cell := range subtable.cells {
		c := &TableCell{}
		*c = *cell
		c.table = table

		c.col += col - 1
		if colsLeft := table.cols - (c.col - 1); colsLeft < c.colspan {
			table.cols += c.colspan - colsLeft
			table.resetColumnWidths()
			common.Log.Debug("Table: subtable exceeds destination table. Expanding table to %d columns.", table.cols)
		}

		c.row += row - 1

		subRowHeight := subtable.rowHeights[cell.row-1]
		if c.row > table.rows {
			for c.row > table.rows {
				table.rows++
				table.rowHeights = append(table.rowHeights, table.defaultRowHeight)
			}

			table.rowHeights[c.row-1] = subRowHeight
		} else {
			table.rowHeights[c.row-1] = math.Max(table.rowHeights[c.row-1], subRowHeight)
		}

		table.cells = append(table.cells, c)
	}

	sort.Slice(table.cells, func(i, j int) bool {
		rowA := table.cells[i].row
		rowB := table.cells[j].row
		if rowA < rowB {
			return true
		}
		if rowA > rowB {
			return false
		}

		return table.cells[i].col < table.cells[j].col
	})
}

func (table *Table) GeneratePageBlocks(ctx DrawContext) ([]*Block, DrawContext, error) {
	var blocks []*Block
	block := NewBlock(ctx.PageWidth, ctx.PageHeight)

	origCtx := ctx
	if table.positioning.isAbsolute() {
		ctx.X = table.xPos
		ctx.Y = table.yPos
	} else {

		ctx.X += table.margins.left
		ctx.Y += table.margins.top
		ctx.Width -= table.margins.left + table.margins.right
		ctx.Height -= table.margins.bottom + table.margins.top
	}
	tableWidth := ctx.Width

	ulX := ctx.X
	ulY := ctx.Y

	ctx.Height = ctx.PageHeight - ctx.Y - ctx.Margins.bottom
	origHeight := ctx.Height

	startrow := 0

	startHeaderCell := -1
	endHeaderCell := -1

	for cellIdx, cell := range table.cells {

		wf := float64(0.0)
		for i := 0; i < cell.colspan; i++ {
			wf += table.colWidths[cell.col+i-1]
		}

		xrel := float64(0.0)
		for i := 0; i < cell.col-1; i++ {
			xrel += table.colWidths[i] * tableWidth
		}

		yrel := float64(0.0)
		for i := startrow; i < cell.row-1; i++ {
			yrel += table.rowHeights[i]
		}

		w := wf * tableWidth

		h := float64(0.0)
		for i := 0; i < cell.rowspan; i++ {
			h += table.rowHeights[cell.row+i-1]
		}

		if table.hasHeader {
			if cell.row >= table.headerStartRow && cell.row <= table.headerEndRow {
				if startHeaderCell < 0 {
					startHeaderCell = cellIdx
				}
				endHeaderCell = cellIdx
			}
		}

		switch t := cell.content.(type) {
		case *Paragraph:
			p := t
			if p.enableWrap {
				p.SetWidth(w - cell.indent)
			}

			newh := p.Height() + p.margins.bottom + p.margins.bottom
			newh += 0.5 * p.fontSize * p.lineHeight
			if newh > h {
				diffh := newh - h

				table.rowHeights[cell.row+cell.rowspan-2] += diffh
			}
		case *StyledParagraph:
			sp := t
			if sp.enableWrap {
				sp.SetWidth(w - cell.indent)
			}

			newh := sp.Height() + sp.margins.top + sp.margins.bottom
			newh += 0.5 * sp.getTextHeight()
			if newh > h {
				diffh := newh - h

				table.rowHeights[cell.row+cell.rowspan-2] += diffh
			}
		case *Image:
			img := t
			newh := img.Height() + img.margins.top + img.margins.bottom
			if newh > h {
				diffh := newh - h

				table.rowHeights[cell.row+cell.rowspan-2] += diffh
			}
		case *Table:
			tbl := t
			newh := tbl.Height() + tbl.margins.top + tbl.margins.bottom
			if newh > h {
				diffh := newh - h

				table.rowHeights[cell.row+cell.rowspan-2] += diffh
			}
		case *List:
			lst := t
			newh := lst.tableHeight(w-cell.indent) + lst.margins.top + lst.margins.bottom
			if newh > h {
				diffh := newh - h

				table.rowHeights[cell.row+cell.rowspan-2] += diffh
			}
		case *Division:
			div := t

			ctx := DrawContext{
				X:     xrel,
				Y:     yrel,
				Width: w,
			}

			divBlocks, updCtx, err := div.GeneratePageBlocks(ctx)
			if err != nil {
				return nil, ctx, err
			}

			if len(divBlocks) > 1 {

				newh := ctx.Height - h
				if newh > h {
					diffh := newh - h

					table.rowHeights[cell.row+cell.rowspan-2] += diffh
				}
			}

			newh := div.Height() + div.margins.top + div.margins.bottom
			_ = updCtx

			if newh > h {
				diffh := newh - h

				table.rowHeights[cell.row+cell.rowspan-2] += diffh
			}
		}
	}

	var drawingHeaders bool
	var resumeIdx, resumeStartRow int

	for cellIdx := 0; cellIdx < len(table.cells); cellIdx++ {
		cell := table.cells[cellIdx]

		wf := float64(0.0)
		for i := 0; i < cell.colspan; i++ {
			wf += table.colWidths[cell.col+i-1]
		}

		xrel := float64(0.0)
		for i := 0; i < cell.col-1; i++ {
			xrel += table.colWidths[i] * tableWidth
		}

		yrel := float64(0.0)
		for i := startrow; i < cell.row-1; i++ {
			yrel += table.rowHeights[i]
		}

		w := wf * tableWidth

		h := float64(0.0)
		for i := 0; i < cell.rowspan; i++ {
			h += table.rowHeights[cell.row+i-1]
		}

		ctx.Height = origHeight - yrel
		if h > ctx.Height {

			blocks = append(blocks, block)
			block = NewBlock(ctx.PageWidth, ctx.PageHeight)
			ulX = ctx.Margins.left
			ulY = ctx.Margins.top

			ctx.Height = ctx.PageHeight - ctx.Margins.top - ctx.Margins.bottom
			origHeight = ctx.Height

			startrow = cell.row - 1
			yrel = 0

			if table.hasHeader && startHeaderCell >= 0 {
				resumeIdx = cellIdx
				cellIdx = startHeaderCell - 1

				resumeStartRow = startrow
				startrow = table.headerStartRow - 1

				drawingHeaders = true
				continue
			}
		}

		ctx.Width = w
		ctx.X = ulX + xrel
		ctx.Y = ulY + yrel

		border := newBorder(ctx.X, ctx.Y, w, h)

		if cell.backgroundColor != nil {
			r := cell.backgroundColor.R()
			g := cell.backgroundColor.G()
			b := cell.backgroundColor.B()

			border.SetFillColor(ColorRGBFromArithmetic(r, g, b))
		}

		border.LineStyle = cell.borderLineStyle

		border.styleLeft = cell.borderStyleLeft
		border.styleRight = cell.borderStyleRight
		border.styleTop = cell.borderStyleTop
		border.styleBottom = cell.borderStyleBottom

		if cell.borderColorLeft != nil {
			border.SetColorLeft(ColorRGBFromArithmetic(cell.borderColorLeft.R(), cell.borderColorLeft.G(), cell.borderColorLeft.B()))
		}
		if cell.borderColorBottom != nil {
			border.SetColorBottom(ColorRGBFromArithmetic(cell.borderColorBottom.R(), cell.borderColorBottom.G(), cell.borderColorBottom.B()))
		}
		if cell.borderColorRight != nil {
			border.SetColorRight(ColorRGBFromArithmetic(cell.borderColorRight.R(), cell.borderColorRight.G(), cell.borderColorRight.B()))
		}
		if cell.borderColorTop != nil {
			border.SetColorTop(ColorRGBFromArithmetic(cell.borderColorTop.R(), cell.borderColorTop.G(), cell.borderColorTop.B()))
		}

		border.SetWidthBottom(cell.borderWidthBottom)
		border.SetWidthLeft(cell.borderWidthLeft)
		border.SetWidthRight(cell.borderWidthRight)
		border.SetWidthTop(cell.borderWidthTop)

		err := block.Draw(border)
		if err != nil {
			common.Log.Debug("ERROR: %v", err)
		}

		if cell.content != nil {

			cw := cell.content.Width()
			switch t := cell.content.(type) {
			case *Paragraph:
				if t.enableWrap {
					cw = t.getMaxLineWidth() / 1000.0
				}
			case *StyledParagraph:
				if t.enableWrap {
					cw = t.getMaxLineWidth() / 1000.0
				}
			case *Table:
				cw = w
			case *List:
				cw = w
			}

			switch cell.horizontalAlignment {
			case CellHorizontalAlignmentLeft:

				ctx.X += cell.indent
				ctx.Width -= cell.indent
			case CellHorizontalAlignmentCenter:

				dw := w - cw
				if dw > 0 {
					ctx.X += dw / 2
					ctx.Width -= dw / 2
				}
			case CellHorizontalAlignmentRight:
				if w > cw {
					ctx.X = ctx.X + w - cw - cell.indent
					ctx.Width = cw
				}
			}

			ch := cell.content.Height()
			switch cell.verticalAlignment {
			case CellVerticalAlignmentTop:

			case CellVerticalAlignmentMiddle:
				dh := h - ch
				if dh > 0 {
					ctx.Y += dh / 2
					ctx.Height -= dh / 2
				}
			case CellVerticalAlignmentBottom:
				if h > ch {
					ctx.Y = ctx.Y + h - ch
					ctx.Height = ch
				}
			}

			err := block.DrawWithContext(cell.content, ctx)
			if err != nil {
				common.Log.Debug("ERROR: %v", err)
			}
		}

		ctx.Y += h

		if drawingHeaders && cellIdx+1 > endHeaderCell {

			ulY += yrel + h
			origHeight -= h + yrel

			startrow = resumeStartRow
			cellIdx = resumeIdx - 1

			drawingHeaders = false
		}
	}
	blocks = append(blocks, block)

	if table.positioning.isAbsolute() {
		return blocks, origCtx, nil
	}

	ctx.X = origCtx.X

	ctx.Width = origCtx.Width

	ctx.Y += table.margins.bottom

	return blocks, ctx, nil
}

type CellBorderStyle int

const (
	CellBorderStyleNone CellBorderStyle = iota

	CellBorderStyleSingle
	CellBorderStyleDouble
)

type CellBorderSide int

const (
	CellBorderSideLeft CellBorderSide = iota

	CellBorderSideRight

	CellBorderSideTop

	CellBorderSideBottom

	CellBorderSideAll
)

type CellHorizontalAlignment int

const (
	CellHorizontalAlignmentLeft CellHorizontalAlignment = iota

	CellHorizontalAlignmentCenter

	CellHorizontalAlignmentRight
)

type CellVerticalAlignment int

const (
	CellVerticalAlignmentTop CellVerticalAlignment = iota

	CellVerticalAlignmentMiddle

	CellVerticalAlignmentBottom
)

type TableCell struct {
	backgroundColor *model.PdfColorDeviceRGB

	borderLineStyle draw.LineStyle

	borderStyleLeft   CellBorderStyle
	borderColorLeft   *model.PdfColorDeviceRGB
	borderWidthLeft   float64
	borderStyleBottom CellBorderStyle
	borderColorBottom *model.PdfColorDeviceRGB
	borderWidthBottom float64
	borderStyleRight  CellBorderStyle
	borderColorRight  *model.PdfColorDeviceRGB
	borderWidthRight  float64
	borderStyleTop    CellBorderStyle
	borderColorTop    *model.PdfColorDeviceRGB
	borderWidthTop    float64

	row, col int

	rowspan int
	colspan int

	content VectorDrawable

	horizontalAlignment CellHorizontalAlignment
	verticalAlignment   CellVerticalAlignment

	indent float64

	table *Table
}

func (table *Table) NewCell() *TableCell {
	return table.newCell(1)
}

func (table *Table) MultiColCell(colspan int) *TableCell {
	return table.newCell(colspan)
}

func (table *Table) newCell(colspan int) *TableCell {
	table.curCell++

	curRow := (table.curCell-1)/table.cols + 1
	for curRow > table.rows {
		table.rows++
		table.rowHeights = append(table.rowHeights, table.defaultRowHeight)
	}
	curCol := (table.curCell-1)%(table.cols) + 1

	cell := &TableCell{}
	cell.row = curRow
	cell.col = curCol
	cell.rowspan = 1

	cell.indent = 5

	cell.borderStyleLeft = CellBorderStyleNone
	cell.borderLineStyle = draw.LineStyleSolid

	cell.horizontalAlignment = CellHorizontalAlignmentLeft
	cell.verticalAlignment = CellVerticalAlignmentTop

	cell.borderWidthLeft = 0
	cell.borderWidthBottom = 0
	cell.borderWidthRight = 0
	cell.borderWidthTop = 0

	col := ColorBlack
	cell.borderColorLeft = model.NewPdfColorDeviceRGB(col.ToRGB())
	cell.borderColorBottom = model.NewPdfColorDeviceRGB(col.ToRGB())
	cell.borderColorRight = model.NewPdfColorDeviceRGB(col.ToRGB())
	cell.borderColorTop = model.NewPdfColorDeviceRGB(col.ToRGB())

	if colspan < 1 {
		common.Log.Debug("Table: cell colspan less than 1 (%d). Setting cell colspan to 1.", colspan)
		colspan = 1
	}

	remainingCols := table.cols - (cell.col - 1)
	if colspan > remainingCols {
		common.Log.Debug("Table: cell colspan (%d) exceeds remaining row cols (%d). Adjusting colspan.", colspan, remainingCols)
		colspan = remainingCols
	}
	cell.colspan = colspan
	table.curCell += colspan - 1

	table.cells = append(table.cells, cell)

	cell.table = table

	return cell
}

func (table *Table) SkipCells(num int) {
	if num < 0 {
		common.Log.Debug("Table: cannot skip back to previous cells")
		return
	}
	table.curCell += num
}

func (table *Table) SkipRows(num int) {
	ncells := num*table.cols - 1
	if ncells < 0 {
		common.Log.Debug("Table: cannot skip back to previous cells")
		return
	}
	table.curCell += ncells
}

func (table *Table) SkipOver(rows, cols int) {
	ncells := rows*table.cols + cols - 1
	if ncells < 0 {
		common.Log.Debug("Table: cannot skip back to previous cells")
		return
	}
	table.curCell += ncells
}

func (cell *TableCell) SetIndent(indent float64) {
	cell.indent = indent
}

func (cell *TableCell) SetHorizontalAlignment(halign CellHorizontalAlignment) {
	cell.horizontalAlignment = halign
}

func (cell *TableCell) SetVerticalAlignment(valign CellVerticalAlignment) {
	cell.verticalAlignment = valign
}

func (cell *TableCell) SetBorder(side CellBorderSide, style CellBorderStyle, width float64) {
	if style == CellBorderStyleSingle && side == CellBorderSideAll {
		cell.borderStyleLeft = CellBorderStyleSingle
		cell.borderWidthLeft = width
		cell.borderStyleBottom = CellBorderStyleSingle
		cell.borderWidthBottom = width
		cell.borderStyleRight = CellBorderStyleSingle
		cell.borderWidthRight = width
		cell.borderStyleTop = CellBorderStyleSingle
		cell.borderWidthTop = width
	} else if style == CellBorderStyleDouble && side == CellBorderSideAll {
		cell.borderStyleLeft = CellBorderStyleDouble
		cell.borderWidthLeft = width
		cell.borderStyleBottom = CellBorderStyleDouble
		cell.borderWidthBottom = width
		cell.borderStyleRight = CellBorderStyleDouble
		cell.borderWidthRight = width
		cell.borderStyleTop = CellBorderStyleDouble
		cell.borderWidthTop = width
	} else if (style == CellBorderStyleSingle || style == CellBorderStyleDouble) && side == CellBorderSideLeft {
		cell.borderStyleLeft = style
		cell.borderWidthLeft = width
	} else if (style == CellBorderStyleSingle || style == CellBorderStyleDouble) && side == CellBorderSideBottom {
		cell.borderStyleBottom = style
		cell.borderWidthBottom = width
	} else if (style == CellBorderStyleSingle || style == CellBorderStyleDouble) && side == CellBorderSideRight {
		cell.borderStyleRight = style
		cell.borderWidthRight = width
	} else if (style == CellBorderStyleSingle || style == CellBorderStyleDouble) && side == CellBorderSideTop {
		cell.borderStyleTop = style
		cell.borderWidthTop = width
	}
}

func (cell *TableCell) SetBorderColor(col Color) {
	cell.borderColorLeft = model.NewPdfColorDeviceRGB(col.ToRGB())
	cell.borderColorBottom = model.NewPdfColorDeviceRGB(col.ToRGB())
	cell.borderColorRight = model.NewPdfColorDeviceRGB(col.ToRGB())
	cell.borderColorTop = model.NewPdfColorDeviceRGB(col.ToRGB())
}

func (cell *TableCell) SetBorderLineStyle(style draw.LineStyle) {
	cell.borderLineStyle = style
}

func (cell *TableCell) SetBackgroundColor(col Color) {
	cell.backgroundColor = model.NewPdfColorDeviceRGB(col.ToRGB())
}

func (cell *TableCell) Width(ctx DrawContext) float64 {
	fraction := float64(0.0)
	for j := 0; j < cell.colspan; j++ {
		fraction += cell.table.colWidths[cell.col+j-1]
	}
	w := ctx.Width * fraction
	return w
}

func (cell *TableCell) SetContent(vd VectorDrawable) error {
	switch t := vd.(type) {
	case *Paragraph:
		if t.defaultWrap {

			t.enableWrap = true
		}

		cell.content = vd
	case *StyledParagraph:
		if t.defaultWrap {

			t.enableWrap = true
		}

		cell.content = vd
	case *Image:
		cell.content = vd
	case *Table:
		cell.content = vd
	case *List:
		cell.content = vd
	case *Division:
		cell.content = vd
	default:
		common.Log.Debug("ERROR: unsupported cell content type %T", vd)
		return core.ErrTypeError
	}

	return nil
}
