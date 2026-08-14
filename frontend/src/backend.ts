import { AppService } from "../bindings/github.com/the-khiem7/PenguinSpaceStudio";
import { ProbeMode } from "../bindings/github.com/the-khiem7/PenguinSpaceStudio/internal/elevation";

export { ProbeMode as ElevationProbeMode };

export type MeasurementKind = "measured-logical" | "estimated-logical" | "measured-physical" | "unavailable";
export type Measurement = { bytes: number; kind: MeasurementKind };

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

export type DockerRelationshipObservation = {
  kind: string;
  count: number;
  available: boolean;
};

export type DockerScopedResource = {
  id: string;
  kind: string;
  name: string;
  scope: "compose-project" | "unscoped";
  labels: { project?: string; service?: string; network?: string; volume?: string };
  relationships: DockerRelationshipObservation[];
  relatedResourceId?: string;
  stateful: boolean;
  risk: string;
};

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
  ownershipGroups: Array<{
    scope: "compose-project" | "unscoped";
    project?: string;
    resources: DockerScopedResource[];
  }>;
  ownershipComplete: boolean;
  builder: {
    scope: "selected-builder";
    name: string;
    count: number;
    countAvailable: boolean;
    sharedCount: number;
    records: Array<{ id: string; shared: boolean; mutable: boolean; reclaimable: boolean }>;
    boundary: string;
  };
  warnings: string[] | null;
};

export type DockerNetworkRemovalPlan = {
  id: string;
  networkId: string;
  networkName: string;
  project: string;
  networkLabel: string;
  risk: "Review";
  consequence: string;
  createdAt: string;
};

export type DockerNetworkRemovalOutcome = {
  planId: string;
  networkId: string;
  networkName: string;
  removalCommandAttempted: boolean;
  removalCommandCompleted: boolean;
  verifiedAbsent: boolean;
  awarenessRefreshed: boolean;
  historyRecorded: boolean;
  reclaimedActual: { bytes: number; kind: string };
  message: string;
  failure?: string;
  awareness: DockerAwareness;
};

export type ProjectEcosystem = "node" | "rust" | "python" | "gradle" | "maven";

export type ProjectSkipKind =
  | "reparse-point"
  | "excluded-metadata"
  | "unclaimed-generated-name"
  | "depth-limit"
  | "unreadable"
  | "excluded-by-rule"
  | "non-regular";

export type ProjectSkippedPath = { relativePath: string; kind: ProjectSkipKind; reason: string };

export type ProjectArtifactObservation = {
  name: string;
  path: string;
  relativePath: string;
  ecosystem: ProjectEcosystem;
  storageClass: string;
  risk: string;
  recoveryCost: string;
  measured: Measurement;
  boundary: string;
};

export type ProjectObservation = {
  name: string;
  path: string;
  relativePath: string;
  ecosystems: ProjectEcosystem[];
  markers: string[];
  artifacts: ProjectArtifactObservation[];
};

export type ProjectDiscovery = {
  root: string;
  rootApproved: boolean;
  inspectedAt: string;
  complete: boolean;
  truncated: boolean;
  projects: ProjectObservation[];
  skipped: ProjectSkippedPath[];
  warnings: string[] | null;
  message: string;
  boundary: string;
};

export type ProjectExclusionRule = { rule: string; relativePath: string; matched: boolean };

export type ProjectArtifactMeasurement = {
  name: string;
  path: string;
  relativePath: string;
  ecosystem: ProjectEcosystem;
  storageClass: string;
  risk: string;
  recoveryCost: string;
  measured: Measurement;
  reclaimable: Measurement;
  files: number;
  directories: number;
  complete: boolean;
  truncated: boolean;
  skipped: ProjectSkippedPath[];
  boundary: string;
};

export type ProjectMeasurement = {
  name: string;
  path: string;
  relativePath: string;
  root: string;
  measuredAt: string;
  artifacts: ProjectArtifactMeasurement[];
  total: Measurement;
  reclaimable: Measurement;
  complete: boolean;
  truncated: boolean;
  exclusions: ProjectExclusionRule[];
  warnings: string[] | null;
  message: string;
  boundary: string;
};

export type WSLAwareness = {
  cliAvailable: boolean;
  available: boolean;
  executablePath?: string;
  inspectedAt: string;
  distributions: Array<{
    name: string;
    state: "running" | "stopped" | "unknown";
    version: number;
    versionAvailable: boolean;
    vhdx: {
      path?: string;
      pathAvailable: boolean;
      physicalSize: Measurement;
      logicalUsage: Measurement;
      compactable: Measurement;
      message: string;
    };
  }>;
  warnings: string[] | null;
  message: string;
};

export const backend = {
  dashboard: () => AppService.Dashboard(),
  runFixtureScenario: (): Promise<Scenario> => AppService.RunFixtureScenario() as Promise<Scenario>,
  startElevationProbe: (mode: ProbeMode): Promise<ElevationStatus> => AppService.StartElevationProbe(mode) as Promise<ElevationStatus>,
  cancelElevationProbe: (): Promise<ElevationStatus> => AppService.CancelElevationProbe() as Promise<ElevationStatus>,
  elevationStatus: (): Promise<ElevationStatus> => AppService.ElevationStatus() as Promise<ElevationStatus>,
  recentHistory: () => AppService.RecentHistory(),
  inspectDockerAwareness: (): Promise<DockerAwareness> => AppService.InspectDockerAwareness() as Promise<DockerAwareness>,
  inspectWSLAwareness: (): Promise<WSLAwareness> => AppService.InspectWSLAwareness() as Promise<WSLAwareness>,
  inspectDockerNetworkRemoval: (networkId: string): Promise<DockerNetworkRemovalPlan> => AppService.InspectDockerNetworkRemoval(networkId) as Promise<DockerNetworkRemovalPlan>,
  executeDockerNetworkRemoval: (planId: string): Promise<DockerNetworkRemovalOutcome> => AppService.ExecuteDockerNetworkRemoval(planId, true) as Promise<DockerNetworkRemovalOutcome>,
  setWorkspaceRoot: (path: string): Promise<WorkspaceRoot> => AppService.SetWorkspaceRoot(path) as Promise<WorkspaceRoot>,
  discoverDeveloperProviders: (): Promise<ProviderAvailability[]> => AppService.DiscoverDeveloperProviders() as Promise<ProviderAvailability[]>,
  discoverProjectStorage: (): Promise<ProjectDiscovery> => AppService.DiscoverProjectStorage() as Promise<ProjectDiscovery>,
  measureProjectStorage: (projectPath: string, exclusions: string[]): Promise<ProjectMeasurement> =>
    AppService.MeasureProjectStorage(projectPath, exclusions) as Promise<ProjectMeasurement>,
  inspectDeveloperProvider: (providerId: string): Promise<ProviderInspection> => AppService.InspectDeveloperProvider(providerId) as Promise<ProviderInspection>,
  executeDeveloperProvider: (providerId: string): Promise<ProviderCleanupOutcome> => AppService.ExecuteDeveloperProvider(providerId, true) as Promise<ProviderCleanupOutcome>,
};
