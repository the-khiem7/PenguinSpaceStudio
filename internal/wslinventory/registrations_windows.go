//go:build windows

package wslinventory

import (
	"context"
	"fmt"

	"golang.org/x/sys/windows/registry"
)

const wslRegistrationKey = `Software\Microsoft\Windows\CurrentVersion\Lxss`

type windowsRegistrationSource struct{}

func newSystemRegistrationSource() RegistrationSource {
	return windowsRegistrationSource{}
}

func (windowsRegistrationSource) List(ctx context.Context) ([]Registration, error) {
	root, err := registry.OpenKey(registry.CURRENT_USER, wslRegistrationKey, registry.READ)
	if err != nil {
		return nil, fmt.Errorf("open current-user WSL registration key: %w", err)
	}
	defer root.Close()

	subkeys, err := root.ReadSubKeyNames(-1)
	if err != nil {
		return nil, fmt.Errorf("list current-user WSL registrations: %w", err)
	}
	registrations := make([]Registration, 0, len(subkeys))
	for _, subkeyName := range subkeys {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		subkey, err := registry.OpenKey(root, subkeyName, registry.QUERY_VALUE)
		if err != nil {
			return nil, fmt.Errorf("open registration %q: %w", subkeyName, err)
		}
		name, _, nameErr := subkey.GetStringValue("DistributionName")
		basePath, _, pathErr := subkey.GetStringValue("BasePath")
		closeErr := subkey.Close()
		if nameErr != nil {
			return nil, fmt.Errorf("read DistributionName for %q: %w", subkeyName, nameErr)
		}
		if pathErr != nil {
			return nil, fmt.Errorf("read BasePath for %q: %w", name, pathErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close registration %q: %w", subkeyName, closeErr)
		}
		registrations = append(registrations, Registration{Name: name, BasePath: basePath})
	}
	return registrations, nil
}
