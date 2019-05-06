package contentstream

import (
	"bytes"
	"fmt"

	"github.com/codefinio/doc/pdf/core"
)

type ContentStreamOperation struct {
	Params  []core.PdfObject
	Operand string
}

type ContentStreamOperations []*ContentStreamOperation

func (ops *ContentStreamOperations) isWrapped() bool {
	if len(*ops) < 2 {
		return false
	}

	depth := 0
	for _, op := range *ops {
		if op.Operand == "q" {
			depth++
		} else if op.Operand == "Q" {
			depth--
		} else {
			if depth < 1 {
				return false
			}
		}
	}

	return depth == 0
}

func (ops *ContentStreamOperations) WrapIfNeeded() *ContentStreamOperations {
	if len(*ops) == 0 {

		return ops
	}
	if ops.isWrapped() {
		return ops
	}

	*ops = append([]*ContentStreamOperation{{Operand: "q"}}, *ops...)

	depth := 0
	for _, op := range *ops {
		if op.Operand == "q" {
			depth++
		} else if op.Operand == "Q" {
			depth--
		}
	}

	for depth > 0 {
		*ops = append(*ops, &ContentStreamOperation{Operand: "Q"})
		depth--
	}

	return ops
}

func (ops *ContentStreamOperations) Bytes() []byte {
	var buf bytes.Buffer

	for _, op := range *ops {
		if op == nil {
			continue
		}

		if op.Operand == "BI" {

			buf.WriteString(op.Operand + "\n")
			buf.WriteString(op.Params[0].WriteString())

		} else {

			for _, param := range op.Params {
				buf.WriteString(param.WriteString())
				buf.WriteString(" ")

			}

			buf.WriteString(op.Operand + "\n")
		}
	}

	return buf.Bytes()
}

func (ops *ContentStreamOperations) String() string {
	return string(ops.Bytes())
}

func (csp *ContentStreamParser) ExtractText() (string, error) {
	operations, err := csp.Parse()
	if err != nil {
		return "", err
	}
	inText := false
	xPos, yPos := float64(-1), float64(-1)
	txt := ""
	for _, op := range *operations {
		if op.Operand == "BT" {
			inText = true
		} else if op.Operand == "ET" {
			inText = false
		}
		if op.Operand == "Td" || op.Operand == "TD" || op.Operand == "T*" {

			txt += "\n"
		}
		if op.Operand == "Tm" {
			if len(op.Params) != 6 {
				continue
			}
			xfloat, ok := op.Params[4].(*core.PdfObjectFloat)
			if !ok {
				xint, ok := op.Params[4].(*core.PdfObjectInteger)
				if !ok {
					continue
				}
				xfloat = core.MakeFloat(float64(*xint))
			}
			yfloat, ok := op.Params[5].(*core.PdfObjectFloat)
			if !ok {
				yint, ok := op.Params[5].(*core.PdfObjectInteger)
				if !ok {
					continue
				}
				yfloat = core.MakeFloat(float64(*yint))
			}
			if yPos == -1 {
				yPos = float64(*yfloat)
			} else if yPos > float64(*yfloat) {
				txt += "\n"
				xPos = float64(*xfloat)
				yPos = float64(*yfloat)
				continue
			}
			if xPos == -1 {
				xPos = float64(*xfloat)
			} else if xPos < float64(*xfloat) {
				txt += "\t"
				xPos = float64(*xfloat)
			}
		}
		if inText && op.Operand == "TJ" {
			if len(op.Params) < 1 {
				continue
			}
			paramList, ok := op.Params[0].(*core.PdfObjectArray)
			if !ok {
				return "", fmt.Errorf("invalid parameter type, no array (%T)", op.Params[0])
			}
			for _, obj := range paramList.Elements() {
				switch v := obj.(type) {
				case *core.PdfObjectString:
					txt += v.Str()
				case *core.PdfObjectFloat:
					if *v < -100 {
						txt += " "
					}
				case *core.PdfObjectInteger:
					if *v < -100 {
						txt += " "
					}
				}
			}
		} else if inText && op.Operand == "Tj" {
			if len(op.Params) < 1 {
				continue
			}
			param, ok := op.Params[0].(*core.PdfObjectString)
			if !ok {
				return "", fmt.Errorf("invalid parameter type, not string (%T)", op.Params[0])
			}
			txt += param.Str()
		}
	}

	return txt, nil
}
