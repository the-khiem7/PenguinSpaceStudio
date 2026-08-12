package core

import "context"

func InspectProvider(ctx context.Context, provider Provider) (ProviderInspection, error) {
	detection, err := provider.Detect(ctx)
	if err != nil {
		return ProviderInspection{}, err
	}
	inspection := ProviderInspection{Detection: detection, ExecutionEnabled: provider.ExecutionEnabled()}
	if !detection.Detected || !detection.Supported {
		return inspection, nil
	}

	scan, err := provider.Scan(ctx, detection)
	if err != nil {
		return ProviderInspection{}, err
	}
	plan, err := provider.Plan(scan)
	if err != nil {
		return ProviderInspection{}, err
	}
	inspection.Scan = scan
	inspection.Plan = plan
	return inspection, nil
}
