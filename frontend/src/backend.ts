import { AppService } from "../bindings/github.com/the-khiem7/PenguinSpaceStudio";
import { ProbeMode } from "../bindings/github.com/the-khiem7/PenguinSpaceStudio/internal/elevation";

export { ProbeMode as ElevationProbeMode };

export type Scenario = {
  scan: { providerId: string; items: Array<{ name: string; measured: { bytes: number } }> };
  plan: { actions: Array<{ risk: string; recoveryCost: string; consequence: string; estimated: { bytes: number } }> };
  execution: { executed: boolean; destructive: boolean; message: string };
  verification: { reclaimedActual: { bytes: number } };
};

export type ElevationStatus = {
  id: string;
  actionId: string;
  state: string;
  message: string;
  updatedAt: string;
};

export type ProviderInspection = {
  detection: {
    providerId: string;
    detected: boolean;
    supported: boolean;
    version?: string;
    executablePath?: string;
    message: string;
  };
  scan: {
    providerId: string;
    items: Array<{
      name: string;
      storageClass: string;
      risk: string;
      recoveryCost: string;
      location?: string;
      measured: { bytes: number; kind: string };
    }>;
  };
  plan: {
    actions: Array<{
	  location?: string;
      risk: string;
      recoveryCost: string;
      consequence: string;
      observed: { bytes: number; kind: string };
      estimated: { bytes: number; kind: string };
    }>;
  };
  executionEnabled: boolean;
};

export type ProviderCleanupOutcome = {
  execution: { planId: string; executed: boolean; destructive: boolean; message: string };
  verification: {
    planId: string;
    measuredAfter: { bytes: number; kind: string };
    reclaimedActual: { bytes: number; kind: string };
  };
};

export type WorkspaceRoot = { path: string };

export const backend = {
  dashboard: () => AppService.Dashboard(),
  runFixtureScenario: (): Promise<Scenario> => AppService.RunFixtureScenario() as Promise<Scenario>,
  startElevationProbe: (mode: ProbeMode): Promise<ElevationStatus> => AppService.StartElevationProbe(mode) as Promise<ElevationStatus>,
  cancelElevationProbe: (): Promise<ElevationStatus> => AppService.CancelElevationProbe() as Promise<ElevationStatus>,
  elevationStatus: (): Promise<ElevationStatus> => AppService.ElevationStatus() as Promise<ElevationStatus>,
  recentHistory: () => AppService.RecentHistory(),
  setWorkspaceRoot: (path: string): Promise<WorkspaceRoot> => AppService.SetWorkspaceRoot(path) as Promise<WorkspaceRoot>,
  inspectDeveloperProvider: (providerId: string): Promise<ProviderInspection> => AppService.InspectDeveloperProvider(providerId) as Promise<ProviderInspection>,
  executeDeveloperProvider: (providerId: string): Promise<ProviderCleanupOutcome> => AppService.ExecuteDeveloperProvider(providerId, true) as Promise<ProviderCleanupOutcome>,
};
