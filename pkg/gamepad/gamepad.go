package gamepad

import (
	"errors"
	"sync"

	"github.com/0xcafed00d/joystick"
	"github.com/hashicorp/go-hclog"
)

var (
	// ErrNoSuchField is returned in the event that no field is
	// available for the given fieldID.
	ErrNoSuchField = errors.New("no field with that ID exists")
)

// Values abstracts over the joystick to the given values that
// are returned by a gamepad.
type Values struct {
	AxisLX           int
	AxisLY           int
	AxisRX           int
	AxisRY           int
	AxisDX           int
	AxisDY           int
	ButtonBack       bool
	ButtonStart      bool
	ButtonLeftStick  bool
	ButtonRightStick bool
	ButtonX          bool
	ButtonY          bool
	ButtonA          bool
	ButtonB          bool
	ButtonLShoulder  bool
	ButtonRShoulder  bool
	ButtonLT         bool
	ButtonRT         bool
}

// JSController handles the action of actually fetching data from the
// joystick and making it available to the rest of the system.
type JSController struct {
	l hclog.Logger

	controllers map[string]joystick.Joystick

	cMutex sync.RWMutex
}

// NewJSController sets up the joystick controller.
func NewJSController(opts ...Option) JSController {
	jsc := JSController{
		l:           hclog.NewNullLogger(),
		controllers: make(map[string]joystick.Joystick),
	}

	for _, o := range opts {
		o(&jsc)
	}
	return jsc
}

// BindController attaches a controller to a particular name.
func (j *JSController) BindController(name string, id int) error {
	j.cMutex.Lock()
	defer j.cMutex.Unlock()
	js, jserr := joystick.Open(id)
	if jserr != nil {
		return jserr
	}
	j.controllers[name] = js

	if js.AxisCount() != 6 || js.ButtonCount() != 12 {
		j.l.Error("Wrong joystick counts!", "axis", js.AxisCount(), " buttons", js.ButtonCount())
		return errors.New("bad joystick config")
	}

	j.l.Info("Successfully bound controller", "fid", name, "jsid", id)
	return nil
}

// GetState polls the joystick and updates the values available to the
// controller.
func (j *JSController) GetState(fieldID string) (*Values, error) {
	j.cMutex.RLock()
	defer j.cMutex.RUnlock()

	js, ok := j.controllers[fieldID]
	if !ok {
		return nil, ErrNoSuchField
	}

	jinfo, err := js.Read()
	if err != nil {
		return nil, err
	}

	jvals := Values{
		AxisLX: mapRange(jinfo.AxisData[0], -32768, 32768, 0, 255),
		AxisLY: mapRange(jinfo.AxisData[1], -32768, 32768, 0, 255),

		AxisRX: mapRange(jinfo.AxisData[2], -32768, 32768, 0, 255),
		AxisRY: mapRange(jinfo.AxisData[3], -32768, 32768, 0, 255),

		AxisDX: mapRange(jinfo.AxisData[4], -32768, 32768, 0, 255),
		AxisDY: mapRange(jinfo.AxisData[5], -32768, 32768, 0, 255),

		ButtonBack:       (jinfo.Buttons & (1 << uint32(8))) != 0,
		ButtonStart:      (jinfo.Buttons & (1 << uint32(9))) != 0,
		ButtonLeftStick:  (jinfo.Buttons & (1 << uint32(10))) != 0,
		ButtonRightStick: (jinfo.Buttons & (1 << uint32(11))) != 0,
		ButtonX:          (jinfo.Buttons & (1 << uint32(0))) != 0,
		ButtonY:          (jinfo.Buttons & (1 << uint32(3))) != 0,
		ButtonA:          (jinfo.Buttons & (1 << uint32(1))) != 0,
		ButtonB:          (jinfo.Buttons & (1 << uint32(2))) != 0,
		ButtonLShoulder:  (jinfo.Buttons & (1 << uint32(4))) != 0,
		ButtonRShoulder:  (jinfo.Buttons & (1 << uint32(5))) != 0,
		ButtonLT:         (jinfo.Buttons & (1 << uint32(6))) != 0,
		ButtonRT:         (jinfo.Buttons & (1 << uint32(7))) != 0,
	}

	j.l.Trace("Refreshed state", "fid", fieldID, "state", jvals)
	return &jvals, nil
}

// func (j *JSController) doRefreshAll() {
// 	j.fMutex.RLock()
// 	defer j.fMutex.RUnlock()

// 	for f, id := range j.fields {
// 		go func() {
// 			if err := j.UpdateState(f); err != nil {
// 				j.l.Warn("Error polling joystick, attempting rebind", "error", err, "field", f)
// 				j.BindController(f, id)
// 			}
// 		}()
// 	}
// }

func mapRange(x, xMin, xMax, oMin, oMax int) int {
	return (x-xMin)*(oMax-oMin)/(xMax-xMin) + oMin
}
