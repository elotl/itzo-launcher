package addons

import (
	"fmt"
	"os/exec"
)

type unitAction string

const (
	unitStart   unitAction = "start"
	unitStop    unitAction = "stop"
	unitRestart unitAction = "restart"
)

func manageUnit(action unitAction, unit string) error {
	cmd := exec.Command("systemctl", string(action), unit)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %v; output:\n%s", action, unit, err, output)
	}
	return nil
}
