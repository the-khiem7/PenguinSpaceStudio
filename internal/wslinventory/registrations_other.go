//go:build !windows

package wslinventory

import (
	"context"
	"errors"
)

type unsupportedRegistrationSource struct{}

func newSystemRegistrationSource() RegistrationSource {
	return unsupportedRegistrationSource{}
}

func (unsupportedRegistrationSource) List(context.Context) ([]Registration, error) {
	return nil, errors.New("WSL registration metadata is available only on Windows")
}
