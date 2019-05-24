package contentstream

import (
	"math"

	"github.com/finalversus/doc/common"
	"github.com/finalversus/doc/pdf/core"
	"github.com/finalversus/doc/pdf/model"
)

type ContentCreator struct {
	operands ContentStreamOperations
}

func NewContentCreator() *ContentCreator {
	creator := &ContentCreator{}
	creator.operands = ContentStreamOperations{}
	return creator
}

func (cc *ContentCreator) Operations() *ContentStreamOperations {
	return &cc.operands
}

func (cc *ContentCreator) Bytes() []byte {
	return cc.operands.Bytes()
}

func (cc *ContentCreator) String() string {
	return string(cc.operands.Bytes())
}

func (cc *ContentCreator) Wrap() {
	cc.operands.WrapIfNeeded()
}

func (cc *ContentCreator) AddOperand(op ContentStreamOperation) *ContentCreator {
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_q() *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "q"
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_Q() *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "Q"
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_cm(a, b, c, d, e, f float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "cm"
	op.Params = makeParamsFromFloats([]float64{a, b, c, d, e, f})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Translate(tx, ty float64) *ContentCreator {
	return cc.Add_cm(1, 0, 0, 1, tx, ty)
}

func (cc *ContentCreator) Scale(sx, sy float64) *ContentCreator {
	return cc.Add_cm(sx, 0, 0, sy, 0, 0)
}

func (cc *ContentCreator) RotateDeg(angle float64) *ContentCreator {
	u1 := math.Cos(angle * math.Pi / 180.0)
	u2 := math.Sin(angle * math.Pi / 180.0)
	u3 := -math.Sin(angle * math.Pi / 180.0)
	u4 := math.Cos(angle * math.Pi / 180.0)
	return cc.Add_cm(u1, u2, u3, u4, 0, 0)
}

func (cc *ContentCreator) Add_w(lineWidth float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "w"
	op.Params = makeParamsFromFloats([]float64{lineWidth})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_J(lineCapStyle string) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "J"
	op.Params = makeParamsFromNames([]core.PdfObjectName{core.PdfObjectName(lineCapStyle)})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_j(lineJoinStyle string) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "j"
	op.Params = makeParamsFromNames([]core.PdfObjectName{core.PdfObjectName(lineJoinStyle)})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_M(miterlimit float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "M"
	op.Params = makeParamsFromFloats([]float64{miterlimit})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_d(dashArray []int64, dashPhase int64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "d"

	op.Params = []core.PdfObject{}
	op.Params = append(op.Params, core.MakeArrayFromIntegers64(dashArray))
	op.Params = append(op.Params, core.MakeInteger(dashPhase))
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_ri(intent core.PdfObjectName) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "ri"
	op.Params = makeParamsFromNames([]core.PdfObjectName{intent})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_i(flatness float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "i"
	op.Params = makeParamsFromFloats([]float64{flatness})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_gs(dictName core.PdfObjectName) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "gs"
	op.Params = makeParamsFromNames([]core.PdfObjectName{dictName})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_m(x, y float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "m"
	op.Params = makeParamsFromFloats([]float64{x, y})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_l(x, y float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "l"
	op.Params = makeParamsFromFloats([]float64{x, y})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_c(x1, y1, x2, y2, x3, y3 float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "c"
	op.Params = makeParamsFromFloats([]float64{x1, y1, x2, y2, x3, y3})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_v(x2, y2, x3, y3 float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "v"
	op.Params = makeParamsFromFloats([]float64{x2, y2, x3, y3})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_y(x1, y1, x3, y3 float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "y"
	op.Params = makeParamsFromFloats([]float64{x1, y1, x3, y3})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_h() *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "h"
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_re(x, y, width, height float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "re"
	op.Params = makeParamsFromFloats([]float64{x, y, width, height})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_Do(name core.PdfObjectName) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "Do"
	op.Params = makeParamsFromNames([]core.PdfObjectName{name})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_S() *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "S"
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_s() *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "s"
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_f() *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "f"
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_f_starred() *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "f*"
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_B() *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "B"
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_B_starred() *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "B*"
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_b() *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "b"
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_b_starred() *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "b*"
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_n() *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "n"
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_W() *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "W"
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_W_starred() *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "W*"
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_CS(name core.PdfObjectName) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "CS"
	op.Params = makeParamsFromNames([]core.PdfObjectName{name})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_cs(name core.PdfObjectName) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "cs"
	op.Params = makeParamsFromNames([]core.PdfObjectName{name})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_SC(c ...float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "SC"
	op.Params = makeParamsFromFloats(c)
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_SCN(c ...float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "SCN"
	op.Params = makeParamsFromFloats(c)
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_SCN_pattern(name core.PdfObjectName, c ...float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "SCN"
	op.Params = makeParamsFromFloats(c)
	op.Params = append(op.Params, core.MakeName(string(name)))
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_scn(c ...float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "scn"
	op.Params = makeParamsFromFloats(c)
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_scn_pattern(name core.PdfObjectName, c ...float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "scn"
	op.Params = makeParamsFromFloats(c)
	op.Params = append(op.Params, core.MakeName(string(name)))
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_G(gray float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "G"
	op.Params = makeParamsFromFloats([]float64{gray})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_g(gray float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "g"
	op.Params = makeParamsFromFloats([]float64{gray})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_RG(r, g, b float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "RG"
	op.Params = makeParamsFromFloats([]float64{r, g, b})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_rg(r, g, b float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "rg"
	op.Params = makeParamsFromFloats([]float64{r, g, b})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_K(c, m, y, k float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "K"
	op.Params = makeParamsFromFloats([]float64{c, m, y, k})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_k(c, m, y, k float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "k"
	op.Params = makeParamsFromFloats([]float64{c, m, y, k})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) SetStrokingColor(color model.PdfColor) *ContentCreator {
	switch t := color.(type) {
	case *model.PdfColorDeviceGray:
		cc.Add_G(t.Val())
	case *model.PdfColorDeviceRGB:
		cc.Add_RG(t.R(), t.G(), t.B())
	case *model.PdfColorDeviceCMYK:
		cc.Add_K(t.C(), t.M(), t.Y(), t.K())
	default:
		common.Log.Debug("SetStrokingColor: unsupported color: %T", t)
	}
	return cc
}

func (cc *ContentCreator) SetNonStrokingColor(color model.PdfColor) *ContentCreator {
	switch t := color.(type) {
	case *model.PdfColorDeviceGray:
		cc.Add_g(t.Val())
	case *model.PdfColorDeviceRGB:
		cc.Add_rg(t.R(), t.G(), t.B())
	case *model.PdfColorDeviceCMYK:
		cc.Add_k(t.C(), t.M(), t.Y(), t.K())
	default:
		common.Log.Debug("SetNonStrokingColor: unsupported color: %T", t)
	}
	return cc
}

func (cc *ContentCreator) Add_sh(name core.PdfObjectName) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "sh"
	op.Params = makeParamsFromNames([]core.PdfObjectName{name})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_BT() *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "BT"
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_ET() *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "ET"
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_Tc(charSpace float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "Tc"
	op.Params = makeParamsFromFloats([]float64{charSpace})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_Tw(wordSpace float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "Tw"
	op.Params = makeParamsFromFloats([]float64{wordSpace})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_Tz(scale float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "Tz"
	op.Params = makeParamsFromFloats([]float64{scale})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_TL(leading float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "TL"
	op.Params = makeParamsFromFloats([]float64{leading})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_Tf(fontName core.PdfObjectName, fontSize float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "Tf"
	op.Params = makeParamsFromNames([]core.PdfObjectName{fontName})
	op.Params = append(op.Params, makeParamsFromFloats([]float64{fontSize})...)
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_Tr(render int64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "Tr"
	op.Params = makeParamsFromInts([]int64{render})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_Ts(rise float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "Ts"
	op.Params = makeParamsFromFloats([]float64{rise})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_Td(tx, ty float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "Td"
	op.Params = makeParamsFromFloats([]float64{tx, ty})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_TD(tx, ty float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "TD"
	op.Params = makeParamsFromFloats([]float64{tx, ty})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_Tm(a, b, c, d, e, f float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "Tm"
	op.Params = makeParamsFromFloats([]float64{a, b, c, d, e, f})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_Tstar() *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "T*"
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_Tj(textstr core.PdfObjectString) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "Tj"
	op.Params = makeParamsFromStrings([]core.PdfObjectString{textstr})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_quote(textstr core.PdfObjectString) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "'"
	op.Params = makeParamsFromStrings([]core.PdfObjectString{textstr})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_quotes(textstr core.PdfObjectString, aw, ac float64) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "''"
	op.Params = makeParamsFromFloats([]float64{aw, ac})
	op.Params = append(op.Params, makeParamsFromStrings([]core.PdfObjectString{textstr})...)
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_TJ(vals ...core.PdfObject) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "TJ"
	op.Params = []core.PdfObject{core.MakeArray(vals...)}
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_BMC(tag core.PdfObjectName) *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "BMC"
	op.Params = makeParamsFromNames([]core.PdfObjectName{tag})
	cc.operands = append(cc.operands, &op)
	return cc
}

func (cc *ContentCreator) Add_EMC() *ContentCreator {
	op := ContentStreamOperation{}
	op.Operand = "EMC"
	cc.operands = append(cc.operands, &op)
	return cc
}
