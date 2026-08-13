//go:build !windows

package wslinventory

import "errors"

type systemAllocationSource struct{}

func (systemAllocationSource) MeasureAllocation(string) (FileAllocation, error) {
	return FileAllocation{}, errors.New("VHDX allocation metadata is available only on Windows")
}
