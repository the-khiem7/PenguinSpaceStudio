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
    needsConfiguration: boolean;
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

export type ProviderAvailability = {
  providerId: string;
  status: "available" | "needs-configuration" | "unavailable" | "not-applicable" | "workspace-required";
  workspaceScoped: boolean;
  detection: {
    providerId: string;
    detected: boolean;
    supported: boolean;
    needsConfiguration: boolean;
    version?: string;
    executablePath?: string;
    message: string;
  };
  message: string;
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

export type DockerAwareness = {
  daemon: {
    cliAvailable: boolean;
    available: boolean;
    executablePath?: string;
    version?: string;
    operatingSystem?: string;
    architecture?: string;
    message: string;
  };
  inspectedAt: string;
  resources: Array<{
    kind: string;
    name: string;
    count: number;
    countAvailable: boolean;
    size: { bytes: number; kind: string };
    reclaimable: { bytes: number; kind: string };
    stateful: boolean;
    boundary: string;
  }>;
  warnings: string[] | null;
};

export const backend = {
  dashboard: () => AppService.Dashboard(),
  runFixtureScenario: (): Promise<Scenario> => AppService.RunFixtureScenario() as Promise<Scenario>,
  startElevationProbe: (mode: ProbeMode): Promise<ElevationStatus> => AppService.StartElevationProbe(mode) as Promise<ElevationStatus>,
  cancelElevationProbe: (): Promise<ElevationStatus> => AppService.CancelElevationProbe() as Promise<ElevationStatus>,
  elevationStatus: (): Promise<ElevationStatus> => AppService.ElevationStatus() as Promise<ElevationStatus>,
  recentHistory: () => AppService.RecentHistory(),
  inspectDockerAwareness: (): Promise<DockerAwareness> => AppService.InspectDockerAwareness() as Promise<DockerAwareness>,
  setWorkspaceRoot: (path: string): Promise<WorkspaceRoot> => AppService.SetWorkspaceRoot(path) as Promise<WorkspaceRoot>,
  discoverDeveloperProviders: (): Promise<ProviderAvailability[]> => AppService.DiscoverDeveloperProviders() as Promise<ProviderAvailability[]>,
  inspectDeveloperProvider: (providerId: string): Promise<ProviderInspection> => AppService.InspectDeveloperProvider(providerId) as Promise<ProviderInspection>,
  executeDeveloperProvider: (providerId: string): Promise<ProviderCleanupOutcome> => AppService.ExecuteDeveloperProvider(providerId, true) as Promise<ProviderCleanupOutcome>,
};
