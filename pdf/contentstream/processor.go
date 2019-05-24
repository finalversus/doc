package contentstream

import (
	"errors"

	"github.com/finalversus/doc/common"
	"github.com/finalversus/doc/pdf/core"
	"github.com/finalversus/doc/pdf/internal/transform"
	"github.com/finalversus/doc/pdf/model"
)

type GraphicsState struct {
	ColorspaceStroking    model.PdfColorspace
	ColorspaceNonStroking model.PdfColorspace
	ColorStroking         model.PdfColor
	ColorNonStroking      model.PdfColor
	CTM                   transform.Matrix
}

type GraphicStateStack []GraphicsState

func (gsStack *GraphicStateStack) Push(gs GraphicsState) {
	*gsStack = append(*gsStack, gs)
}

func (gsStack *GraphicStateStack) Pop() GraphicsState {
	gs := (*gsStack)[len(*gsStack)-1]
	*gsStack = (*gsStack)[:len(*gsStack)-1]
	return gs
}

func (gs *GraphicsState) Transform(x, y float64) (float64, float64) {
	return gs.CTM.Transform(x, y)
}

type ContentStreamProcessor struct {
	graphicsStack GraphicStateStack
	operations    []*ContentStreamOperation
	graphicsState GraphicsState

	handlers     []handlerEntry
	currentIndex int
}

type HandlerFunc func(op *ContentStreamOperation, gs GraphicsState, resources *model.PdfPageResources) error

type handlerEntry struct {
	Condition HandlerConditionEnum
	Operand   string
	Handler   HandlerFunc
}

type HandlerConditionEnum int

const (
	HandlerConditionEnumOperand HandlerConditionEnum = iota
	HandlerConditionEnumAllOperands
)

func (hce HandlerConditionEnum) All() bool {
	return hce == HandlerConditionEnumAllOperands
}

func (hce HandlerConditionEnum) Operand() bool {
	return hce == HandlerConditionEnumOperand
}

func NewContentStreamProcessor(ops []*ContentStreamOperation) *ContentStreamProcessor {
	csp := ContentStreamProcessor{}
	csp.graphicsStack = GraphicStateStack{}

	gs := GraphicsState{}

	csp.graphicsState = gs

	csp.handlers = []handlerEntry{}
	csp.currentIndex = 0
	csp.operations = ops

	return &csp
}

func (proc *ContentStreamProcessor) AddHandler(condition HandlerConditionEnum, operand string, handler HandlerFunc) {
	entry := handlerEntry{}
	entry.Condition = condition
	entry.Operand = operand
	entry.Handler = handler
	proc.handlers = append(proc.handlers, entry)
}

func (proc *ContentStreamProcessor) getColorspace(name string, resources *model.PdfPageResources) (model.PdfColorspace, error) {
	switch name {
	case "DeviceGray":
		return model.NewPdfColorspaceDeviceGray(), nil
	case "DeviceRGB":
		return model.NewPdfColorspaceDeviceRGB(), nil
	case "DeviceCMYK":
		return model.NewPdfColorspaceDeviceCMYK(), nil
	case "Pattern":
		return model.NewPdfColorspaceSpecialPattern(), nil
	}

	cs, has := resources.GetColorspaceByName(core.PdfObjectName(name))
	if has {
		return cs, nil
	}

	switch name {
	case "CalGray":
		return model.NewPdfColorspaceCalGray(), nil
	case "CalRGB":
		return model.NewPdfColorspaceCalRGB(), nil
	case "Lab":
		return model.NewPdfColorspaceLab(), nil
	}

	common.Log.Debug("Unknown colorspace requested: %s", name)
	return nil, errors.New("unsupported colorspace")
}

func (proc *ContentStreamProcessor) getInitialColor(cs model.PdfColorspace) (model.PdfColor, error) {
	switch cs := cs.(type) {
	case *model.PdfColorspaceDeviceGray:
		return model.NewPdfColorDeviceGray(0.0), nil
	case *model.PdfColorspaceDeviceRGB:
		return model.NewPdfColorDeviceRGB(0.0, 0.0, 0.0), nil
	case *model.PdfColorspaceDeviceCMYK:
		return model.NewPdfColorDeviceCMYK(0.0, 0.0, 0.0, 1.0), nil
	case *model.PdfColorspaceCalGray:
		return model.NewPdfColorCalGray(0.0), nil
	case *model.PdfColorspaceCalRGB:
		return model.NewPdfColorCalRGB(0.0, 0.0, 0.0), nil
	case *model.PdfColorspaceLab:
		l := 0.0
		a := 0.0
		b := 0.0
		if cs.Range[0] > 0 {
			l = cs.Range[0]
		}
		if cs.Range[2] > 0 {
			a = cs.Range[2]
		}
		return model.NewPdfColorLab(l, a, b), nil
	case *model.PdfColorspaceICCBased:
		if cs.Alternate == nil {

			common.Log.Trace("ICC Based not defined - attempting fall back (N = %d)", cs.N)
			if cs.N == 1 {
				common.Log.Trace("Falling back to DeviceGray")
				return proc.getInitialColor(model.NewPdfColorspaceDeviceGray())
			} else if cs.N == 3 {
				common.Log.Trace("Falling back to DeviceRGB")
				return proc.getInitialColor(model.NewPdfColorspaceDeviceRGB())
			} else if cs.N == 4 {
				common.Log.Trace("Falling back to DeviceCMYK")
				return proc.getInitialColor(model.NewPdfColorspaceDeviceCMYK())
			} else {
				return nil, errors.New("alternate space not defined for ICC")
			}
		}
		return proc.getInitialColor(cs.Alternate)
	case *model.PdfColorspaceSpecialIndexed:
		if cs.Base == nil {
			return nil, errors.New("indexed base not specified")
		}
		return proc.getInitialColor(cs.Base)
	case *model.PdfColorspaceSpecialSeparation:
		if cs.AlternateSpace == nil {
			return nil, errors.New("alternate space not specified")
		}
		return proc.getInitialColor(cs.AlternateSpace)
	case *model.PdfColorspaceDeviceN:
		if cs.AlternateSpace == nil {
			return nil, errors.New("alternate space not specified")
		}
		return proc.getInitialColor(cs.AlternateSpace)
	case *model.PdfColorspaceSpecialPattern:

		return nil, nil
	}

	common.Log.Debug("Unable to determine initial color for unknown colorspace: %T", cs)
	return nil, errors.New("unsupported colorspace")
}

func (proc *ContentStreamProcessor) Process(resources *model.PdfPageResources) error {

	proc.graphicsState.ColorspaceStroking = model.NewPdfColorspaceDeviceGray()
	proc.graphicsState.ColorspaceNonStroking = model.NewPdfColorspaceDeviceGray()
	proc.graphicsState.ColorStroking = model.NewPdfColorDeviceGray(0)
	proc.graphicsState.ColorNonStroking = model.NewPdfColorDeviceGray(0)
	proc.graphicsState.CTM = transform.IdentityMatrix()

	for _, op := range proc.operations {
		var err error

		switch op.Operand {
		case "q":
			proc.graphicsStack.Push(proc.graphicsState)
		case "Q":
			proc.graphicsState = proc.graphicsStack.Pop()

		case "CS":
			err = proc.handleCommand_CS(op, resources)
		case "cs":
			err = proc.handleCommand_cs(op, resources)
		case "SC":
			err = proc.handleCommand_SC(op, resources)
		case "SCN":
			err = proc.handleCommand_SCN(op, resources)
		case "sc":
			err = proc.handleCommand_sc(op, resources)
		case "scn":
			err = proc.handleCommand_scn(op, resources)
		case "G":
			err = proc.handleCommand_G(op, resources)
		case "g":
			err = proc.handleCommand_g(op, resources)
		case "RG":
			err = proc.handleCommand_RG(op, resources)
		case "rg":
			err = proc.handleCommand_rg(op, resources)
		case "K":
			err = proc.handleCommand_K(op, resources)
		case "k":
			err = proc.handleCommand_k(op, resources)
		case "cm":
			err = proc.handleCommand_cm(op, resources)
		}
		if err != nil {
			common.Log.Debug("Processor handling error (%s): %v", op.Operand, err)
			common.Log.Debug("Operand: %#v", op.Operand)
			return err
		}

		for _, entry := range proc.handlers {
			var err error
			if entry.Condition.All() {
				err = entry.Handler(op, proc.graphicsState, resources)
			} else if entry.Condition.Operand() && op.Operand == entry.Operand {
				err = entry.Handler(op, proc.graphicsState, resources)
			}
			if err != nil {
				common.Log.Debug("Processor handler error: %v", err)
				return err
			}
		}
	}

	return nil
}

func (proc *ContentStreamProcessor) handleCommand_CS(op *ContentStreamOperation, resources *model.PdfPageResources) error {
	if len(op.Params) < 1 {
		common.Log.Debug("Invalid cs command, skipping over")
		return errors.New("too few parameters")
	}
	if len(op.Params) > 1 {
		common.Log.Debug("cs command with too many parameters - continuing")
		return errors.New("too many parameters")
	}
	name, ok := op.Params[0].(*core.PdfObjectName)
	if !ok {
		common.Log.Debug("ERROR: cs command with invalid parameter, skipping over")
		return errors.New("type check error")
	}

	cs, err := proc.getColorspace(string(*name), resources)
	if err != nil {
		return err
	}
	proc.graphicsState.ColorspaceStroking = cs

	color, err := proc.getInitialColor(cs)
	if err != nil {
		return err
	}
	proc.graphicsState.ColorStroking = color

	return nil
}

func (proc *ContentStreamProcessor) handleCommand_cs(op *ContentStreamOperation, resources *model.PdfPageResources) error {
	if len(op.Params) < 1 {
		common.Log.Debug("Invalid CS command, skipping over")
		return errors.New("too few parameters")
	}
	if len(op.Params) > 1 {
		common.Log.Debug("CS command with too many parameters - continuing")
		return errors.New("too many parameters")
	}
	name, ok := op.Params[0].(*core.PdfObjectName)
	if !ok {
		common.Log.Debug("ERROR: CS command with invalid parameter, skipping over")
		return errors.New("type check error")
	}

	cs, err := proc.getColorspace(string(*name), resources)
	if err != nil {
		return err
	}
	proc.graphicsState.ColorspaceNonStroking = cs

	color, err := proc.getInitialColor(cs)
	if err != nil {
		return err
	}
	proc.graphicsState.ColorNonStroking = color

	return nil
}

func (proc *ContentStreamProcessor) handleCommand_SC(op *ContentStreamOperation, resources *model.PdfPageResources) error {

	cs := proc.graphicsState.ColorspaceStroking
	if len(op.Params) != cs.GetNumComponents() {
		common.Log.Debug("Invalid number of parameters for SC")
		common.Log.Debug("Number %d not matching colorspace %T", len(op.Params), cs)
		return errors.New("invalid number of parameters")
	}

	color, err := cs.ColorFromPdfObjects(op.Params)
	if err != nil {
		return err
	}

	proc.graphicsState.ColorStroking = color
	return nil
}

func isPatternCS(cs model.PdfColorspace) bool {
	_, isPattern := cs.(*model.PdfColorspaceSpecialPattern)
	return isPattern
}

func (proc *ContentStreamProcessor) handleCommand_SCN(op *ContentStreamOperation, resources *model.PdfPageResources) error {
	cs := proc.graphicsState.ColorspaceStroking

	if !isPatternCS(cs) {
		if len(op.Params) != cs.GetNumComponents() {
			common.Log.Debug("Invalid number of parameters for SC")
			common.Log.Debug("Number %d not matching colorspace %T", len(op.Params), cs)
			return errors.New("invalid number of parameters")
		}
	}

	color, err := cs.ColorFromPdfObjects(op.Params)
	if err != nil {
		return err
	}

	proc.graphicsState.ColorStroking = color

	return nil
}

func (proc *ContentStreamProcessor) handleCommand_sc(op *ContentStreamOperation, resources *model.PdfPageResources) error {
	cs := proc.graphicsState.ColorspaceNonStroking

	if !isPatternCS(cs) {
		if len(op.Params) != cs.GetNumComponents() {
			common.Log.Debug("Invalid number of parameters for SC")
			common.Log.Debug("Number %d not matching colorspace %T", len(op.Params), cs)
			return errors.New("invalid number of parameters")
		}
	}

	color, err := cs.ColorFromPdfObjects(op.Params)
	if err != nil {
		return err
	}

	proc.graphicsState.ColorNonStroking = color

	return nil
}

func (proc *ContentStreamProcessor) handleCommand_scn(op *ContentStreamOperation, resources *model.PdfPageResources) error {
	cs := proc.graphicsState.ColorspaceNonStroking

	if !isPatternCS(cs) {
		if len(op.Params) != cs.GetNumComponents() {
			common.Log.Debug("Invalid number of parameters for SC")
			common.Log.Debug("Number %d not matching colorspace %T", len(op.Params), cs)
			return errors.New("invalid number of parameters")
		}
	}

	color, err := cs.ColorFromPdfObjects(op.Params)
	if err != nil {
		common.Log.Debug("ERROR: Fail to get color from params: %+v (CS is %+v)", op.Params, cs)
		return err
	}

	proc.graphicsState.ColorNonStroking = color

	return nil
}

func (proc *ContentStreamProcessor) handleCommand_G(op *ContentStreamOperation, resources *model.PdfPageResources) error {
	cs := model.NewPdfColorspaceDeviceGray()
	if len(op.Params) != cs.GetNumComponents() {
		common.Log.Debug("Invalid number of parameters for SC")
		common.Log.Debug("Number %d not matching colorspace %T", len(op.Params), cs)
		return errors.New("invalid number of parameters")
	}

	color, err := cs.ColorFromPdfObjects(op.Params)
	if err != nil {
		return err
	}

	proc.graphicsState.ColorspaceStroking = cs
	proc.graphicsState.ColorStroking = color

	return nil
}

func (proc *ContentStreamProcessor) handleCommand_g(op *ContentStreamOperation, resources *model.PdfPageResources) error {
	cs := model.NewPdfColorspaceDeviceGray()
	if len(op.Params) != cs.GetNumComponents() {
		common.Log.Debug("Invalid number of parameters for g")
		common.Log.Debug("Number %d not matching colorspace %T", len(op.Params), cs)
		return errors.New("invalid number of parameters")
	}

	color, err := cs.ColorFromPdfObjects(op.Params)
	if err != nil {
		common.Log.Debug("ERROR: handleCommand_g Invalid params. cs=%T op=%s err=%v", cs, op, err)
		return err
	}

	proc.graphicsState.ColorspaceNonStroking = cs
	proc.graphicsState.ColorNonStroking = color

	return nil
}

func (proc *ContentStreamProcessor) handleCommand_RG(op *ContentStreamOperation, resources *model.PdfPageResources) error {
	cs := model.NewPdfColorspaceDeviceRGB()
	if len(op.Params) != cs.GetNumComponents() {
		common.Log.Debug("Invalid number of parameters for RG")
		common.Log.Debug("Number %d not matching colorspace %T", len(op.Params), cs)
		return errors.New("invalid number of parameters")
	}

	color, err := cs.ColorFromPdfObjects(op.Params)
	if err != nil {
		return err
	}

	proc.graphicsState.ColorspaceStroking = cs
	proc.graphicsState.ColorStroking = color

	return nil
}

func (proc *ContentStreamProcessor) handleCommand_rg(op *ContentStreamOperation, resources *model.PdfPageResources) error {
	cs := model.NewPdfColorspaceDeviceRGB()
	if len(op.Params) != cs.GetNumComponents() {
		common.Log.Debug("Invalid number of parameters for SC")
		common.Log.Debug("Number %d not matching colorspace %T", len(op.Params), cs)
		return errors.New("invalid number of parameters")
	}

	color, err := cs.ColorFromPdfObjects(op.Params)
	if err != nil {
		return err
	}

	proc.graphicsState.ColorspaceNonStroking = cs
	proc.graphicsState.ColorNonStroking = color

	return nil
}

func (proc *ContentStreamProcessor) handleCommand_K(op *ContentStreamOperation, resources *model.PdfPageResources) error {
	cs := model.NewPdfColorspaceDeviceCMYK()
	if len(op.Params) != cs.GetNumComponents() {
		common.Log.Debug("Invalid number of parameters for SC")
		common.Log.Debug("Number %d not matching colorspace %T", len(op.Params), cs)
		return errors.New("invalid number of parameters")
	}

	color, err := cs.ColorFromPdfObjects(op.Params)
	if err != nil {
		return err
	}

	proc.graphicsState.ColorspaceStroking = cs
	proc.graphicsState.ColorStroking = color

	return nil
}

func (proc *ContentStreamProcessor) handleCommand_k(op *ContentStreamOperation, resources *model.PdfPageResources) error {
	cs := model.NewPdfColorspaceDeviceCMYK()
	if len(op.Params) != cs.GetNumComponents() {
		common.Log.Debug("Invalid number of parameters for SC")
		common.Log.Debug("Number %d not matching colorspace %T", len(op.Params), cs)
		return errors.New("invalid number of parameters")
	}

	color, err := cs.ColorFromPdfObjects(op.Params)
	if err != nil {
		return err
	}

	proc.graphicsState.ColorspaceNonStroking = cs
	proc.graphicsState.ColorNonStroking = color

	return nil
}

func (proc *ContentStreamProcessor) handleCommand_cm(op *ContentStreamOperation,
	resources *model.PdfPageResources) error {
	if len(op.Params) != 6 {
		common.Log.Debug("ERROR: Invalid number of parameters for cm: %d", len(op.Params))
		return errors.New("invalid number of parameters")
	}

	f, err := core.GetNumbersAsFloat(op.Params)
	if err != nil {
		return err
	}
	m := transform.NewMatrix(f[0], f[1], f[2], f[3], f[4], f[5])
	proc.graphicsState.CTM.Concat(m)

	return nil
}
