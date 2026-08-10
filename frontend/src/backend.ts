import { AppService } from "../bindings/github.com/the-khiem7/PenguinSpaceStudio";

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

export const backend = {
  dashboard: () => AppService.Dashboard(),
  runFixtureScenario: (): Promise<Scenario> => AppService.RunFixtureScenario() as Promise<Scenario>,
  startElevationProbe: (): Promise<ElevationStatus> => AppService.StartElevationProbe() as Promise<ElevationStatus>,
  cancelElevationProbe: (): Promise<ElevationStatus> => AppService.CancelElevationProbe() as Promise<ElevationStatus>,
  elevationStatus: (): Promise<ElevationStatus> => AppService.ElevationStatus() as Promise<ElevationStatus>,
  recentHistory: () => AppService.RecentHistory(),
};
