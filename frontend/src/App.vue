<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { backend, ElevationProbeMode, type DockerAwareness, type DockerNetworkRemovalOutcome, type DockerNetworkRemovalPlan, type DockerScopedResource, type ElevationStatus, type Measurement, type MeasurementKind, type ProjectDiscovery, type ProjectMeasurement, type ProjectSkipKind, type ProjectSkippedPath, type ProviderAvailability, type TimeObservation, type Scenario, type WSLAwareness } from "./backend";
import ProviderCard from "./components/ProviderCard.vue";

type ProviderDefinition = {
  id: string;
  label: string;
  title: string;
  inspectLabel: string;
  description: string;
  workspaceScoped?: boolean;
};

const providerDefinitions: ProviderDefinition[] = [
  { id: "bun.global-cache", label: "Bun", title: "Bun global module cache", inspectLabel: "Inspect Bun cache", description: "Version-aware inspection, reviewed cleanup, and post-operation logical measurement. Physical reclaim may be lower because Bun can use hardlinks." },
  { id: "npm.global-cache", label: "npm", title: "npm managed content cache", inspectLabel: "Inspect npm cache", description: "Measures only npm-managed _cacache content. Logs and npx cache are outside this action; cleanup remains Review because npm requires --force." },
  { id: "pnpm.global-store", label: "pnpm", title: "pnpm configured store", inspectLabel: "Inspect pnpm store", description: "An explicit storeDir can be measured and pruned; default per-disk stores require project-root context. Pruneable bytes remain unavailable before execution." },
  { id: "uv.global-cache", label: "uv", title: "uv global cache", inspectLabel: "Inspect uv cache", description: "Prunes unused entries and centralized project environments. Total cache bytes are observable, but reclaimable bytes remain unavailable before execution." },
  { id: "yarn.classic-global-cache", label: "Yarn Classic", title: "Yarn Classic global cache", inspectLabel: "Inspect Yarn cache", description: "Supports only Yarn Classic 1.x global cache. Modern Yarn's project-local and shared-cache modes remain outside this action." },
  { id: "nuget.http-cache", label: "NuGet", title: "NuGet HTTP cache", inspectLabel: "Inspect NuGet HTTP cache", description: "Clears only HTTP-request metadata. Global packages, temporary data, and plugin caches are deliberately outside this safe action." },
  { id: "cypress.binary-cache", label: "Cypress", title: "Cypress binary cache", inspectLabel: "Inspect Cypress cache", description: "Prunes older downloaded Cypress binaries while retaining the binary currently in use. The observed total is not a reclaim estimate." },
  { id: "cargo.workspace-target", label: "Cargo", title: "Cargo workspace target", inspectLabel: "Inspect Cargo target", description: "Requires Cargo.toml in the approved workspace and a host Cargo installation. Docker-only Cargo toolchains are outside this provider.", workspaceScoped: true },
  { id: "gradle.workspace-build", label: "Gradle", title: "Gradle root build output", inspectLabel: "Inspect Gradle build", description: "Requires a regular Gradle wrapper and root build/settings file. Gradle User Home is excluded.", workspaceScoped: true },
  { id: "maven.workspace-target", label: "Maven", title: "Maven workspace target", inspectLabel: "Inspect Maven target", description: "Requires pom.xml in the approved workspace and a host Maven installation. The shared local repository is excluded.", workspaceScoped: true },
  { id: "playwright.hermetic-browsers", label: "Playwright", title: "Playwright hermetic browsers", inspectLabel: "Inspect local Playwright browsers", description: "Supports only browsers installed hermetically inside this workspace. Shared browser cache and --all removal are excluded.", workspaceScoped: true },
];

const stage = ref("Connecting to the local service…");
const safetyMessage = ref("All cleanup requires a reviewed plan.");
const scenario = ref<Scenario | null>(null);
const error = ref("");
const running = ref(false);
const elevation = ref<ElevationStatus | null>(null);
const elevationStarting = ref(false);
const workspaceRootInput = ref("");
const workspaceRoot = ref("");
const workspaceSaving = ref(false);
const providerAvailability = ref<ProviderAvailability[]>([]);
const discoveryLoading = ref(false);
const discoveryError = ref("");
const dockerAwareness = ref<DockerAwareness | null>(null);
const dockerLoading = ref(false);
const dockerError = ref("");
const dockerNetworkPlan = ref<DockerNetworkRemovalPlan | null>(null);
const dockerNetworkOutcome = ref<DockerNetworkRemovalOutcome | null>(null);
const dockerNetworkReviewingID = ref("");
const dockerNetworkRemoving = ref(false);
const wslAwareness = ref<WSLAwareness | null>(null);
const wslLoading = ref(false);
const wslError = ref("");
const projectDiscovery = ref<ProjectDiscovery | null>(null);
const projectLoading = ref(false);
const projectError = ref("");
const projectExclusionInput = ref("");
const projectMeasurement = ref<ProjectMeasurement | null>(null);
const projectMeasuringPath = ref("");
const projectMeasureError = ref("");
const projectCancelling = ref(false);
let elevationPoller: ReturnType<typeof setInterval> | undefined;

const reclaimed = computed(() => scenario.value?.verification.reclaimedActual.bytes ?? 0);
const formattedReclaimed = computed(() => `${(reclaimed.value / 1024 / 1024).toFixed(2)} MiB`);
const elevationBusy = computed(() =>
  elevationStarting.value ||
  (elevation.value !== null && !["succeeded", "failed", "cancelled", "timed-out"].includes(elevation.value.state)),
);
const availabilityByID = computed(() => new Map(providerAvailability.value.map((entry) => [entry.providerId, entry])));
const availableProviders = computed(() => providerDefinitions.filter((provider) => availabilityByID.value.get(provider.id)?.status === "available"));
const providersNeedingConfiguration = computed(() => providerDefinitions.filter((provider) => availabilityByID.value.get(provider.id)?.status === "needs-configuration"));
const unavailableProviders = computed(() => providerAvailability.value.filter((entry) => entry.status === "unavailable"));
const detectedProviderLabels = computed(() => providerAvailability.value
  .filter((entry) => entry.status === "available" || entry.status === "needs-configuration")
  .map((entry) => definitionFor(entry.providerId)?.label ?? entry.providerId));
const detectedProviders = computed(() => 1 + detectedProviderLabels.value.length);
const detectedProviderSummary = computed(() => {
  if (discoveryLoading.value) return "Checking installed developer tools…";
  return detectedProviderLabels.value.length > 0 ? `Fixture + ${detectedProviderLabels.value.join(" + ")}` : "Fixture; no supported host providers detected";
});
const workspaceDiscoverySummary = computed(() => {
  if (!workspaceRoot.value) return "Choose an approved workspace root to discover project providers."
  if (discoveryLoading.value) return "Checking project markers and host tools…";
  const workspaceEntries = providerAvailability.value.filter((entry) => entry.workspaceScoped);
  if (workspaceEntries.some((entry) => entry.status === "available" || entry.status === "needs-configuration" || entry.status === "unavailable")) return "Matching project providers are shown with the other developer tools.";
  return "No supported host-based project provider matches this workspace root.";
});

onMounted(async () => {
  try {
    const dashboard = await backend.dashboard();
    stage.value = dashboard.stage;
    safetyMessage.value = dashboard.safetyMessage;
  } catch (cause) {
    error.value = `Backend connection failed: ${String(cause)}`;
  }
  await refreshDeveloperProviders();
  await refreshDockerAwareness();
  await refreshWSLAwareness();
  elevationPoller = setInterval(async () => {
    const status = await backend.elevationStatus();
    if (status.id) elevation.value = status;
  }, 500);
});

onBeforeUnmount(() => {
  if (elevationPoller) clearInterval(elevationPoller);
});

async function refreshDeveloperProviders() {
  discoveryLoading.value = true;
  discoveryError.value = "";
  try {
    providerAvailability.value = await backend.discoverDeveloperProviders();
  } catch (cause) {
    discoveryError.value = `Developer tool discovery failed: ${String(cause)}`;
  } finally {
    discoveryLoading.value = false;
  }
}

async function refreshDockerAwareness() {
  dockerLoading.value = true;
  dockerError.value = "";
  dockerAwareness.value = null;
  dockerNetworkPlan.value = null;
  dockerNetworkOutcome.value = null;
  try {
    dockerAwareness.value = await backend.inspectDockerAwareness();
  } catch (cause) {
    dockerError.value = `Docker awareness failed: ${String(cause)}`;
  } finally {
    dockerLoading.value = false;
  }
}

async function refreshProjectDiscovery() {
  if (!workspaceRoot.value) {
    projectDiscovery.value = null;
    projectError.value = "";
    return;
  }
  projectLoading.value = true;
  projectError.value = "";
  projectDiscovery.value = null;
  projectMeasurement.value = null;
  projectMeasureError.value = "";
  try {
    projectDiscovery.value = await backend.discoverProjectStorage();
  } catch (cause) {
    projectError.value = `Project discovery failed: ${String(cause)}`;
  } finally {
    projectLoading.value = false;
  }
}

function projectSkipLabel(kind: ProjectSkipKind) {
  const labels: Record<ProjectSkipKind, string> = {
    "reparse-point": "Reparse point, not followed",
    "excluded-metadata": "Version-control metadata",
    "unclaimed-generated-name": "Generated name without a claiming marker",
    "depth-limit": "Depth bound reached",
    unreadable: "Unreadable",
    "excluded-by-rule": "Excluded by rule",
    "non-regular": "Not a regular file",
  };
  return labels[kind] ?? kind;
}

const projectExclusions = computed(() =>
  projectExclusionInput.value
    .split("\n")
    .map((entry) => entry.trim())
    .filter((entry) => entry.length > 0),
);
const projectMeasurementAuthoritative = computed(() =>
  projectMeasurement.value !== null && projectMeasurement.value.complete && !projectMeasurement.value.truncated,
);

async function measureProject(projectPath: string) {
  projectMeasuringPath.value = projectPath;
  projectMeasureError.value = "";
  projectMeasurement.value = null;
  projectCancelling.value = false;
  try {
    projectMeasurement.value = await backend.measureProjectStorage(projectPath, projectExclusions.value);
  } catch (cause) {
    projectMeasureError.value = `Project measurement failed: ${String(cause)}`;
  } finally {
    projectMeasuringPath.value = "";
    projectCancelling.value = false;
  }
}

async function cancelProjectMeasurement() {
  if (!projectMeasuringPath.value || projectCancelling.value) return;
  projectCancelling.value = true;
  try {
    await backend.cancelProjectMeasurement();
  } catch (cause) {
    // The measurement call above still resolves with a partial result or an error;
    // a failed cancel request itself is not fatal to the pending measurement.
    projectMeasureError.value = `Cancel request failed: ${String(cause)}`;
  }
}

function measuredValue(measurement: Measurement) {
  return measurement.kind === "measured-logical" ? formatBytes(measurement.bytes) : "Unavailable";
}

// Decided 2026-08-14: this is a modification-time-only signal. It is deliberately
// never labelled "last used" or "last accessed", and it must never drive sorting,
// ranking, "abandoned" classification, or preselection. This disclosure string must
// accompany every rendering of the value, not only its first appearance.
const lastModifiedDisclosure = "Last modified reflects when this directory's contents last changed, not when it was last read or used.";

function lastModifiedValue(observation: TimeObservation) {
  if (!observation.available || !observation.value) return "Unavailable";
  const parsed = new Date(observation.value);
  if (Number.isNaN(parsed.getTime())) return "Unavailable";
  return parsed.toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
}

// A count is described by why it is not authoritative, so a deliberate exclusion is
// never presented as the same thing as a failed read or an exhausted budget.
function countLabel(scope: { complete: boolean; truncated: boolean; cancelled?: boolean; skipped?: ProjectSkippedPath[] }) {
  if (scope.cancelled) return "Cancelled, partial";
  if (scope.truncated) return "Partial count";
  if (scope.complete) return "Full count";
  const skipped = scope.skipped ?? [];
  if (skipped.some((skip) => skip.kind !== "excluded-by-rule")) return "Incomplete count";
  if (skipped.some((skip) => skip.kind === "excluded-by-rule")) return "Excluded scope";
  return "Incomplete count";
}

function projectCountLabel(measurement: ProjectMeasurement) {
  return countLabel({
    complete: measurement.complete,
    truncated: measurement.truncated,
    cancelled: measurement.cancelled,
    skipped: measurement.artifacts.flatMap((artifact) => artifact.skipped),
  });
}

const projectArtifactCount = computed(() =>
  (projectDiscovery.value?.projects ?? []).reduce((total, project) => total + project.artifacts.length, 0),
);
const projectSnapshotAuthoritative = computed(() =>
  projectDiscovery.value !== null &&
  projectDiscovery.value.rootApproved &&
  projectDiscovery.value.complete &&
  !projectDiscovery.value.truncated,
);
const projectSnapshotLabel = computed(() => {
  if (projectSnapshotAuthoritative.value) return "Snapshot complete";
  return projectDiscovery.value?.truncated ? "Snapshot truncated" : "Snapshot incomplete";
});

async function refreshWSLAwareness() {
  wslLoading.value = true;
  wslError.value = "";
  wslAwareness.value = null;
  try {
    wslAwareness.value = await backend.inspectWSLAwareness();
  } catch (cause) {
    wslError.value = `WSL awareness failed: ${String(cause)}`;
  } finally {
    wslLoading.value = false;
  }
}

function canReviewDockerNetwork(resource: DockerScopedResource) {
  const attachments = resource.relationships.filter((relationship) => relationship.kind === "container-attachments");
  return dockerAwareness.value?.ownershipComplete === true &&
    resource.kind === "network" &&
    resource.scope === "compose-project" &&
    Boolean(resource.labels.project && resource.labels.network) &&
    attachments.length === 1 && attachments[0].available && attachments[0].count === 0;
}

async function reviewDockerNetwork(networkId: string) {
  dockerNetworkReviewingID.value = networkId;
  dockerNetworkPlan.value = null;
  dockerNetworkOutcome.value = null;
  dockerError.value = "";
  try {
    dockerNetworkPlan.value = await backend.inspectDockerNetworkRemoval(networkId);
  } catch (cause) {
    dockerError.value = `Docker network review failed: ${String(cause)}`;
  } finally {
    dockerNetworkReviewingID.value = "";
  }
}

async function executeDockerNetworkRemoval() {
  if (!dockerNetworkPlan.value) return;
  dockerNetworkRemoving.value = true;
  dockerError.value = "";
  try {
    const outcome = await backend.executeDockerNetworkRemoval(dockerNetworkPlan.value.id);
    dockerNetworkOutcome.value = outcome;
    dockerAwareness.value = outcome.awareness;
    dockerNetworkPlan.value = null;
  } catch (cause) {
    dockerError.value = `Docker network removal failed: ${String(cause)}`;
    dockerNetworkPlan.value = null;
  } finally {
    dockerNetworkRemoving.value = false;
  }
}

function formatBytes(bytes: number, kind?: string) {
  if (kind !== undefined && kind !== "measured-logical" && kind !== "estimated-logical" && kind !== "measured-physical") return "Unavailable";
  if (!Number.isFinite(bytes) || bytes < 0) return "Unavailable";
  if (bytes === 0) return "0 B";
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  const exponent = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  return `${(bytes / 1024 ** exponent).toLocaleString(undefined, { maximumSignificantDigits: 3 })} ${units[exponent]}`;
}

function formatObservedMeasurement(measurement: Measurement, expectedKind: MeasurementKind, evidenceAvailable = true) {
  if (!evidenceAvailable || expectedKind === "unavailable" || measurement.kind !== expectedKind) return "Unavailable";
  return formatBytes(measurement.bytes);
}

function compactDockerID(id: string) {
  const value = id.startsWith("sha256:") ? id.slice(7) : id;
  return value.length > 16 ? `${value.slice(0, 16)}…` : value;
}

function relationshipLabel(kind: string) {
  const labels: Record<string, string> = {
    "container-references": "container references",
    "container-attachments": "container attachments",
    "container-mounts": "container mounts",
    networks: "networks",
    mounts: "mounts",
  };
  return labels[kind] ?? kind;
}

function composeDetail(resource: DockerAwareness["ownershipGroups"][number]["resources"][number]) {
  if (resource.labels.service) return `service: ${resource.labels.service}`;
  if (resource.labels.network) return `network: ${resource.labels.network}`;
  if (resource.labels.volume) return `volume: ${resource.labels.volume}`;
  return resource.scope === "unscoped" ? "No canonical Compose project label" : "Project label only";
}

async function runFixture() {
  running.value = true;
  error.value = "";
  try {
    scenario.value = await backend.runFixtureScenario();
  } catch (cause) {
    error.value = `Fixture scenario failed: ${String(cause)}`;
  } finally {
    running.value = false;
  }
}

async function startElevationProbe(mode: ElevationProbeMode) {
  if (elevationBusy.value) return;
  elevationStarting.value = true;
  error.value = "";
  try {
    elevation.value = await backend.startElevationProbe(mode);
  } catch (cause) {
    error.value = `Elevation probe failed to start: ${String(cause)}`;
  } finally {
    elevationStarting.value = false;
  }
}

async function cancelElevationProbe() {
  error.value = "";
  try {
    elevation.value = await backend.cancelElevationProbe();
  } catch (cause) {
    error.value = `Elevation probe cancellation failed: ${String(cause)}`;
  }
}

async function saveWorkspaceRoot() {
  workspaceSaving.value = true;
  error.value = "";
  try {
    const root = await backend.setWorkspaceRoot(workspaceRootInput.value);
    workspaceRoot.value = root.path;
    await refreshDeveloperProviders();
    await refreshProjectDiscovery();
  } catch (cause) {
    error.value = `Workspace root was not accepted: ${String(cause)}`;
  } finally {
    workspaceSaving.value = false;
  }
}

function definitionFor(providerID: string) {
  return providerDefinitions.find((provider) => provider.id === providerID);
}

function availabilityMessage(providerID: string) {
  const entry = availabilityByID.value.get(providerID);
  return entry?.message || entry?.detection.message || "Additional configuration is required before this provider can inspect storage.";
}
</script>

<template>
  <main class="app-shell">
    <aside class="sidebar">
      <div class="brand"><span class="brand-mark">P</span> PenguinSpace</div>
      <nav aria-label="Primary navigation">
        <a class="active" href="#home">Home</a>
        <a href="#developer-tools">Developer Tools</a>
        <a href="#containers">Containers &amp; WSL</a>
        <a href="#projects">Projects</a>
        <a href="#history">History</a>
        <a href="#settings">Settings</a>
      </nav>
      <p class="stage">{{ stage }}</p>
    </aside>

    <section class="content" id="home">
      <header class="topbar">
        <div>
          <p class="eyebrow">Storage control center</p>
          <h1>Review before reclaiming space.</h1>
        </div>
        <span class="status"><i></i> Backend-owned execution</span>
      </header>

      <section class="metrics" aria-label="Storage overview">
        <article><span>Detected providers</span><strong>{{ detectedProviders }}</strong><small>{{ detectedProviderSummary }}</small></article>
        <article><span>Reviewable estimate</span><strong>{{ scenario ? formattedReclaimed : "—" }}</strong><small>Exact bytes retained by backend</small></article>
        <article><span>Actual reclaimed</span><strong>{{ scenario ? formattedReclaimed : "—" }}</strong><small>Shown only after verification</small></article>
      </section>

      <section class="workspace">
        <div class="panel">
          <div class="panel-heading">
            <div>
              <p class="eyebrow">M1 vertical slice</p>
              <h2>Fixture cache lifecycle</h2>
            </div>
            <button :disabled="running" @click="runFixture">{{ running ? "Running…" : "Run safe fixture" }}</button>
          </div>
          <p class="muted">{{ safetyMessage }}</p>
          <ol class="lifecycle">
            <li :class="{ complete: scenario }"><b>1</b><span>Scan</span><small>{{ scenario?.scan.items[0]?.name ?? "Waiting" }}</small></li>
            <li :class="{ complete: scenario }"><b>2</b><span>Plan</span><small>{{ scenario?.plan.actions[0]?.risk ?? "Review required" }}</small></li>
            <li :class="{ complete: scenario }"><b>3</b><span>Confirm</span><small>Backend gate</small></li>
            <li :class="{ complete: scenario }"><b>4</b><span>Execute</span><small>{{ scenario?.execution.message ?? "No command" }}</small></li>
            <li :class="{ complete: scenario }"><b>5</b><span>Verify</span><small>{{ scenario ? `${formattedReclaimed} measured` : "Pending" }}</small></li>
          </ol>
          <p v-if="error" class="error" role="alert">{{ error }}</p>
        </div>

        <aside class="panel safety-panel">
          <p class="eyebrow">Safety contract</p>
          <h2>Only reviewed developer cache cleanup is wired.</h2>
          <ul>
            <li>Go owns provider semantics and execution.</li>
            <li>Confirmation is mandatory before execution.</li>
            <li>Every provider cache path is rechecked after review.</li>
            <li>The M1 fixture mutates memory only.</li>
            <li>SQLite records verified outcomes locally.</li>
          </ul>
          <div class="elevation-probe">
            <p class="eyebrow">Windows UAC probe</p>
            <p class="muted">This starts an elevated helper that only validates a fixed, no-op action. It cannot run a shell command or cleanup provider.</p>
            <p class="muted">For cancellation, approve UAC and then select Cancel probe. The timeout test waits for its fixed deadline.</p>
            <p class="probe-state">{{ elevation?.state ?? "not started" }}</p>
            <p v-if="elevation" class="muted">{{ elevation.message }}</p>
            <div class="probe-actions">
              <button :disabled="elevationBusy" @click="startElevationProbe(ElevationProbeMode.ProbeModeConsent)">Test Windows consent</button>
              <button class="secondary" :disabled="elevationBusy" @click="startElevationProbe(ElevationProbeMode.ProbeModeCancellation)">Test cancellation</button>
              <button class="secondary" :disabled="elevationBusy" @click="startElevationProbe(ElevationProbeMode.ProbeModeTimeout)">Test timeout</button>
              <button class="secondary" :disabled="!elevationBusy || elevationStarting" @click="cancelElevationProbe">Cancel probe</button>
            </div>
          </div>
        </aside>
      </section>

      <section id="containers" class="provider-category docker-awareness" aria-labelledby="docker-awareness-title">
        <div class="provider-category-heading">
          <div>
            <p class="eyebrow">M3.5 · Exact network lifecycle</p>
            <h2 id="docker-awareness-title">Docker resource ownership and network review</h2>
            <p class="muted">Daemon totals stay separate from exact Compose-label grouping. Only one canonically labeled, unattached custom network can proceed to an exact-ID Review plan; prune, force, and every other resource class remain unavailable.</p>
          </div>
          <button class="secondary" :disabled="dockerLoading || dockerNetworkRemoving || Boolean(dockerNetworkReviewingID)" @click="refreshDockerAwareness">{{ dockerLoading ? "Inspecting…" : "Refresh Docker" }}</button>
        </div>
        <p v-if="dockerError" class="error" role="alert">{{ dockerError }}</p>
        <p v-else-if="dockerLoading && !dockerAwareness" class="muted provider-loading">Checking Docker daemon, ownership labels, and relationships…</p>
        <div v-else-if="dockerAwareness" class="docker-status">
          <article class="panel docker-daemon" :class="{ available: dockerAwareness.daemon.available }">
            <div>
              <span class="status"><i></i>{{ dockerAwareness.daemon.available ? "Daemon available" : "Daemon unavailable" }}</span>
              <strong v-if="dockerAwareness.daemon.available">Docker {{ dockerAwareness.daemon.version }} · {{ dockerAwareness.daemon.operatingSystem }}/{{ dockerAwareness.daemon.architecture }}</strong>
              <strong v-else>Docker resources were not inspected</strong>
            </div>
            <small>{{ dockerAwareness.daemon.message }}</small>
          </article>

          <article v-if="dockerNetworkOutcome" class="network-removal-outcome" :class="{ failed: Boolean(dockerNetworkOutcome.failure) }" role="status">
            <strong>{{ dockerNetworkOutcome.verifiedAbsent ? "Removal verified" : "Removal needs follow-up" }}</strong>
            <p>{{ dockerNetworkOutcome.message }}</p>
            <small v-if="dockerNetworkOutcome.failure">{{ dockerNetworkOutcome.failure }}</small>
            <small>Command attempted: {{ dockerNetworkOutcome.removalCommandAttempted ? "yes" : "no" }} · Command completed: {{ dockerNetworkOutcome.removalCommandCompleted ? "yes" : "no" }} · Refreshed awareness: {{ dockerNetworkOutcome.awarenessRefreshed ? "yes" : "no" }} · History recorded: {{ dockerNetworkOutcome.historyRecorded ? "yes" : "no" }}</small>
          </article>

          <template v-if="dockerAwareness.daemon.available">
            <section class="docker-subsection" aria-labelledby="docker-totals-title">
              <div class="docker-subsection-heading">
                <div>
                  <p class="eyebrow">Daemon-wide</p>
                  <h3 id="docker-totals-title">Resource totals</h3>
                </div>
                <span class="readonly-tag">Observation only</span>
              </div>
              <div class="docker-resource-grid">
                <article v-for="resource in dockerAwareness.resources" :key="resource.kind" class="panel docker-resource" :class="{ stateful: resource.stateful }">
                  <div class="docker-resource-heading">
                    <strong>{{ resource.name }}</strong>
                    <span v-if="resource.stateful" class="danger-tag">Stateful · Danger</span>
                    <span v-else class="readonly-tag">Read-only</span>
                  </div>
                  <div class="docker-resource-metrics">
                    <span><b>{{ resource.countAvailable ? resource.count : "—" }}</b><small>items</small></span>
                    <span><b>{{ formatBytes(resource.size.bytes, resource.size.kind) }}</b><small>daemon size</small></span>
                    <span><b>{{ formatBytes(resource.reclaimable.bytes, resource.reclaimable.kind) }}</b><small>reported reclaimable</small></span>
                  </div>
                  <p>{{ resource.boundary }}</p>
                </article>
              </div>
            </section>

            <section class="docker-subsection" aria-labelledby="docker-ownership-title">
              <div class="docker-subsection-heading">
                <div>
                  <p class="eyebrow">Canonical Compose labels</p>
                  <h3 id="docker-ownership-title">Project groups and unscoped resources</h3>
                </div>
                <span class="readonly-tag">ID-backed</span>
              </div>
              <p v-if="!dockerAwareness.ownershipComplete" class="ownership-incomplete">Ownership snapshot is incomplete. Missing inspect results are not treated as unscoped resources or zero relationships.</p>
              <div class="ownership-groups">
                <article v-for="group in dockerAwareness.ownershipGroups" :key="group.scope === 'unscoped' ? 'unscoped' : group.project" class="panel ownership-group" :class="{ unscoped: group.scope === 'unscoped' }">
                  <header>
                    <div>
                      <span class="scope-kicker">{{ group.scope === "unscoped" ? "Explicit boundary" : "Compose project" }}</span>
                      <h4>{{ group.scope === "unscoped" ? "unscoped" : group.project }}</h4>
                    </div>
                    <b>{{ group.resources.length }} resources</b>
                  </header>
                  <p v-if="group.resources.length === 0" class="empty-state">{{ dockerAwareness.ownershipComplete ? "No resources lack a valid canonical Compose project label." : "No unscoped resources were available in this partial snapshot." }}</p>
                  <div v-else class="ownership-resource-list">
                    <div v-for="resource in group.resources" :key="`${resource.kind}:${resource.id}`" class="ownership-resource" :class="{ stateful: resource.stateful }">
                      <div class="ownership-resource-title">
                        <div>
                          <span>{{ resource.kind }}</span>
                          <strong>{{ resource.name }}</strong>
                        </div>
                        <span v-if="resource.stateful" class="danger-tag">Stateful · Danger</span>
                      </div>
                      <code :title="resource.id">{{ compactDockerID(resource.id) }}</code>
                      <small>{{ composeDetail(resource) }}</small>
                      <div class="relationship-list">
                        <span v-for="relation in resource.relationships" :key="relation.kind">
                          <b>{{ relation.available ? relation.count : "—" }}</b>
                          {{ relationshipLabel(relation.kind) }}
                        </span>
                        <span v-if="resource.relatedResourceId"><b>image</b> {{ compactDockerID(resource.relatedResourceId) }}</span>
                      </div>
                      <button
                        v-if="canReviewDockerNetwork(resource)"
                        class="secondary network-review-button"
                        :disabled="dockerNetworkRemoving || Boolean(dockerNetworkReviewingID)"
                        @click="reviewDockerNetwork(resource.id)"
                      >{{ dockerNetworkReviewingID === resource.id ? "Re-inspecting…" : "Review exact removal" }}</button>
                    </div>
                  </div>
                </article>
              </div>
            </section>

            <section v-if="dockerNetworkPlan" class="panel network-removal-review" aria-labelledby="network-removal-review-title">
              <div class="docker-subsection-heading">
                <div>
                  <p class="eyebrow">Review · exact ID only</p>
                  <h3 id="network-removal-review-title">Remove {{ dockerNetworkPlan.networkName }}?</h3>
                </div>
                <span class="review-tag">{{ dockerNetworkPlan.risk }}</span>
              </div>
              <dl>
                <div><dt>Compose project</dt><dd>{{ dockerNetworkPlan.project }}</dd></div>
                <div><dt>Network label</dt><dd>{{ dockerNetworkPlan.networkLabel }}</dd></div>
                <div><dt>Retained network ID</dt><dd><code :title="dockerNetworkPlan.networkId">{{ compactDockerID(dockerNetworkPlan.networkId) }}</code></dd></div>
                <div><dt>Reclaimed bytes</dt><dd>Unavailable</dd></div>
              </dl>
              <p>{{ dockerNetworkPlan.consequence }}</p>
              <p class="muted">Confirmation triggers one immediate label and attachment re-inspection, then only <code>docker network rm &lt;retained-ID&gt;</code>. No force flag is available.</p>
              <div class="network-removal-actions">
                <button class="secondary" :disabled="dockerNetworkRemoving" @click="dockerNetworkPlan = null">Cancel</button>
                <button :disabled="dockerNetworkRemoving" @click="executeDockerNetworkRemoval">{{ dockerNetworkRemoving ? "Removing and verifying…" : "Confirm exact network removal" }}</button>
              </div>
            </section>

            <section class="panel builder-scope" aria-labelledby="builder-scope-title">
              <div class="docker-subsection-heading">
                <div>
                  <p class="eyebrow">{{ dockerAwareness.builder.scope }}</p>
                  <h3 id="builder-scope-title">{{ dockerAwareness.builder.name }}</h3>
                </div>
                <span class="readonly-tag">No project attribution</span>
              </div>
              <div class="builder-metrics">
                <span><b>{{ dockerAwareness.builder.countAvailable ? dockerAwareness.builder.count : "—" }}</b><small>cache records</small></span>
                <span><b>{{ dockerAwareness.builder.countAvailable ? dockerAwareness.builder.sharedCount : "—" }}</b><small>shared records</small></span>
                <span><b>{{ dockerAwareness.builder.countAvailable ? dockerAwareness.builder.records.filter((record) => record.mutable).length : "—" }}</b><small>mutable records</small></span>
                <span><b>{{ dockerAwareness.builder.countAvailable ? dockerAwareness.builder.records.filter((record) => record.reclaimable).length : "—" }}</b><small>reported reclaimable</small></span>
              </div>
              <p>{{ dockerAwareness.builder.boundary }}</p>
            </section>
          </template>

          <ul v-if="dockerAwareness.warnings?.length" class="docker-warnings">
            <li v-for="warning in dockerAwareness.warnings" :key="warning">{{ warning }}</li>
          </ul>
        </div>
      </section>

      <section id="wsl-discovery" class="provider-category wsl-awareness" aria-labelledby="wsl-awareness-title">
        <div class="provider-category-heading">
          <div>
            <p class="eyebrow">M3.6 · Read-only discovery</p>
            <h2 id="wsl-awareness-title">WSL distributions and backing VHDX files</h2>
            <p class="muted">Lists registered distributions and measures only the physical host-file size of an evidence-backed ext4.vhdx path. Logical usage and compactable bytes remain unavailable.</p>
          </div>
          <button class="secondary" :disabled="wslLoading" @click="refreshWSLAwareness">{{ wslLoading ? "Inspecting…" : "Refresh WSL" }}</button>
        </div>
        <p v-if="wslError" class="error" role="alert">{{ wslError }}</p>
        <p v-else-if="wslLoading && !wslAwareness" class="muted provider-loading">Checking WSL registrations, state, version, and backing-disk metadata…</p>
        <div v-else-if="wslAwareness" class="wsl-status">
          <article class="panel wsl-summary" :class="{ available: wslAwareness.available }">
            <div>
              <span class="status"><i></i>{{ wslAwareness.available ? "WSL available" : "WSL unavailable" }}</span>
              <strong>{{ wslAwareness.available ? `${wslAwareness.distributions.length} registered distributions` : "No WSL metadata inspected" }}</strong>
            </div>
            <small>{{ wslAwareness.message }}</small>
          </article>

          <p v-if="wslAwareness.available && wslAwareness.distributions.length === 0" class="empty-state">No registered WSL distributions were reported.</p>
          <div v-else-if="wslAwareness.distributions.length" class="wsl-distribution-grid">
            <article v-for="distribution in wslAwareness.distributions" :key="distribution.name" class="panel wsl-distribution">
              <header>
                <div>
                  <span>{{ distribution.state }}</span>
                  <h3>{{ distribution.name }}</h3>
                </div>
                <span class="readonly-tag">Observation only</span>
              </header>
              <div class="wsl-metrics">
                <span><small>WSL version</small><b>{{ distribution.versionAvailable ? distribution.version : "—" }}</b></span>
                <span><small>Physical VHDX size</small><b>{{ formatObservedMeasurement(distribution.vhdx.physicalSize, "measured-physical", distribution.vhdx.pathAvailable) }}</b></span>
                <span><small>Logical usage</small><b>{{ formatObservedMeasurement(distribution.vhdx.logicalUsage, "unavailable") }}</b></span>
                <span><small>Compactable</small><b>{{ formatObservedMeasurement(distribution.vhdx.compactable, "unavailable") }}</b></span>
              </div>
              <code v-if="distribution.vhdx.path" :title="distribution.vhdx.path">{{ distribution.vhdx.path }}</code>
              <p>{{ distribution.vhdx.message }}</p>
            </article>
          </div>

          <article class="panel wsl-boundary">
            <strong>No mutation path</strong>
            <p>PenguinSpace does not start or stop a distribution, run Linux commands, mount a disk, change sparse mode, optimize, compact, or claim physical reclaim in M3.6.</p>
          </article>
          <ul v-if="wslAwareness.warnings?.length" class="docker-warnings">
            <li v-for="warning in wslAwareness.warnings" :key="warning">{{ warning }}</li>
          </ul>
        </div>
      </section>

      <section id="workspace-scope" class="panel workspace-scope" aria-labelledby="workspace-scope-title">
        <div class="panel-heading">
          <div>
            <p class="eyebrow">Project-scoped cleanup prerequisite</p>
            <h2 id="workspace-scope-title">Approved workspace root</h2>
          </div>
        </div>
        <p class="muted">Project providers are discovered only when this backend-validated root has a matching project marker. Host-only providers do not inspect Docker-held toolchains or caches.</p>
        <div class="probe-actions">
          <input v-model="workspaceRootInput" aria-label="Approved workspace root path" placeholder="Absolute project path" type="text" />
          <button :disabled="workspaceSaving || !workspaceRootInput.trim()" @click="saveWorkspaceRoot">{{ workspaceSaving ? "Validating…" : "Use workspace" }}</button>
        </div>
        <p class="muted">{{ workspaceRoot ? `Approved: ${workspaceRoot}` : "No workspace root approved." }}</p>
        <p class="muted">{{ workspaceDiscoverySummary }}</p>
      </section>

      <section id="projects" class="provider-category project-discovery" aria-labelledby="project-discovery-title">
        <div class="provider-category-heading">
          <div>
            <p class="eyebrow">M4.1 discovery · M4.2 measurement</p>
            <h2 id="project-discovery-title">Projects below the approved root</h2>
            <p class="muted">Projects are listed only from exact marker files, and a generated directory is listed only when a marker in the same project claims it. Measuring reports exact logical bytes, never physical reclaim; reclaim estimates, plans, and deletion stay unavailable.</p>
          </div>
          <button class="secondary" :disabled="projectLoading || !workspaceRoot || Boolean(projectMeasuringPath)" @click="refreshProjectDiscovery">{{ projectLoading ? "Discovering…" : "Refresh projects" }}</button>
        </div>
        <p v-if="!workspaceRoot" class="empty-state">Approve a workspace root above to discover projects. PenguinSpace never scans an implicit root or the user profile.</p>
        <p v-else-if="projectError" class="error" role="alert">{{ projectError }}</p>
        <p v-else-if="projectLoading" class="muted provider-loading">Reading project markers and claimed generated directories…</p>
        <div v-else-if="projectDiscovery" class="project-status">
          <article class="panel project-summary" :class="{ available: projectSnapshotAuthoritative }">
            <div>
              <span class="status"><i></i>{{ projectSnapshotLabel }}</span>
              <strong>{{ projectDiscovery.projects.length }} project(s) · {{ projectArtifactCount }} claimed generated director(ies)</strong>
            </div>
            <small>{{ projectDiscovery.message }}</small>
          </article>

          <p v-if="projectDiscovery.projects.length === 0" class="empty-state">{{ projectSnapshotAuthoritative ? "No marker-backed project was found below the approved root." : "No marker-backed project was reported, but this snapshot is incomplete, so the approved root cannot be presented as empty." }}</p>
          <div v-else class="project-grid">
            <article v-for="project in projectDiscovery.projects" :key="project.path" class="panel project-card">
              <header>
                <div>
                  <span>{{ project.relativePath === "." ? "approved root" : project.relativePath }}</span>
                  <h3>{{ project.name }}</h3>
                </div>
                <span class="readonly-tag">Observation only</span>
              </header>
              <div class="project-tags">
                <span v-for="ecosystem in project.ecosystems" :key="ecosystem" class="ecosystem-tag">{{ ecosystem }}</span>
                <code v-for="marker in project.markers" :key="marker">{{ marker }}</code>
              </div>
              <p class="muted last-modified-note" :title="lastModifiedDisclosure">Last modified: {{ lastModifiedValue(project.lastModified) }}</p>
              <p v-if="project.artifacts.length === 0" class="muted">No allow-listed generated directory is claimed by this project's markers.</p>
              <div v-else class="project-measure-actions">
                <button
                  class="secondary project-measure-button"
                  :disabled="Boolean(projectMeasuringPath) || projectLoading"
                  @click="measureProject(project.path)"
                >{{ projectMeasuringPath === project.path ? "Measuring…" : "Measure logical bytes" }}</button>
                <button
                  v-if="projectMeasuringPath === project.path"
                  class="secondary project-cancel-button"
                  :disabled="projectCancelling"
                  @click="cancelProjectMeasurement"
                >{{ projectCancelling ? "Cancelling…" : "Cancel measurement" }}</button>
              </div>
              <ul v-if="project.artifacts.length" class="artifact-list">
                <li v-for="artifact in project.artifacts" :key="artifact.path">
                  <div class="artifact-title">
                    <strong>{{ artifact.name }}</strong>
                    <span class="review-tag">{{ artifact.risk }} · {{ artifact.recoveryCost }}</span>
                  </div>
                  <div class="artifact-metrics">
                    <span><small>Storage class</small><b>{{ artifact.storageClass }}</b></span>
                    <span><small>Claimed by</small><b>{{ artifact.ecosystem }}</b></span>
                    <span><small>Size</small><b>{{ formatObservedMeasurement(artifact.measured, "unavailable") }}</b></span>
                    <span :title="lastModifiedDisclosure"><small>Last modified</small><b>{{ lastModifiedValue(artifact.lastModified) }}</b></span>
                  </div>
                  <small>{{ artifact.boundary }}</small>
                </li>
              </ul>
            </article>
          </div>

          <section class="panel measurement-scope" aria-labelledby="measurement-scope-title">
            <div class="docker-subsection-heading">
              <div>
                <p class="eyebrow">M4.2 · Measurement scope</p>
                <h3 id="measurement-scope-title">Exclusions for the next measurement</h3>
              </div>
              <span class="readonly-tag">Not persisted</span>
            </div>
            <p class="muted">One path per line, resolved against the <strong>approved root</strong>, not against the selected project. The backend validates every rule, rejects patterns, anything outside the root, and any path through a reparse point, then reports each rule with the result. An exclusion only removes bytes from the count; it can never re-include a path that a safety rule rejected.</p>
            <textarea
              v-model="projectExclusionInput"
              aria-label="Exclusion paths, one per line"
              placeholder="node_modules/.cache&#10;dist/reports"
              rows="3"
            ></textarea>
            <p class="muted">{{ projectExclusions.length === 0 ? "No exclusion is applied; measurement will count every readable regular file." : `${projectExclusions.length} exclusion(s) will be sent with the next measurement.` }}</p>
          </section>

          <p v-if="projectMeasureError" class="error" role="alert">{{ projectMeasureError }}</p>

          <p v-if="projectMeasuringPath" class="muted provider-loading" role="status" aria-live="polite">{{ projectCancelling ? "Cancelling; the partial count gathered so far will still be shown." : "Counting exact logical bytes; this can take a while on a large dependency tree. Use Cancel measurement above to stop early." }}</p>

          <section v-if="projectMeasurement" class="panel project-measurement" aria-labelledby="project-measurement-title" role="status" aria-live="polite">
            <div class="docker-subsection-heading">
              <div>
                <p class="eyebrow">{{ projectMeasurement.relativePath === "." ? "approved root" : projectMeasurement.relativePath }}</p>
                <h3 id="project-measurement-title">Measured logical bytes for {{ projectMeasurement.name }}</h3>
              </div>
              <span :class="projectMeasurementAuthoritative ? 'readonly-tag' : 'review-tag'" :title="projectMeasurement.cancelled ? 'Stopped by an explicit Cancel measurement request' : ''">{{ projectCountLabel(projectMeasurement) }}</span>
            </div>
            <div class="artifact-metrics">
              <span><small>Project total (logical)</small><b>{{ measuredValue(projectMeasurement.total) }}</b></span>
              <span><small>Reclaimable</small><b>{{ measuredValue(projectMeasurement.reclaimable) }}</b></span>
              <span><small>Artifacts measured</small><b>{{ projectMeasurement.artifacts.length }}</b></span>
            </div>
            <p class="muted">{{ projectMeasurement.message }}</p>
            <ul class="artifact-list">
              <li v-for="artifact in projectMeasurement.artifacts" :key="artifact.path">
                <div class="artifact-title">
                  <strong>{{ artifact.name }}</strong>
                  <span :class="artifact.complete && !artifact.truncated ? 'readonly-tag' : 'review-tag'">{{ countLabel(artifact) }}</span>
                </div>
                <div class="artifact-metrics">
                  <span><small>Logical bytes</small><b>{{ measuredValue(artifact.measured) }}</b></span>
                  <span><small>Files · directories</small><b>{{ artifact.files }} · {{ artifact.directories }}</b></span>
                  <span><small>Reclaimable</small><b>{{ measuredValue(artifact.reclaimable) }}</b></span>
                  <span :title="lastModifiedDisclosure"><small>Last modified</small><b>{{ lastModifiedValue(artifact.lastModified) }}</b></span>
                </div>
                <ul v-if="artifact.skipped.length" class="measurement-skip-list">
                  <li v-for="skip in artifact.skipped" :key="`${skip.kind}:${skip.relativePath}`">
                    <strong>{{ skip.relativePath }}</strong>
                    <span>{{ projectSkipLabel(skip.kind) }} — {{ skip.reason }}</span>
                  </li>
                </ul>
              </li>
            </ul>
            <ul v-if="projectMeasurement.exclusions.length" class="exclusion-list">
              <li v-for="rule in projectMeasurement.exclusions" :key="rule.rule">
                <code>{{ rule.relativePath }}</code>
                <span>{{ rule.matched ? "Applied; its bytes are not in the total" : "Matched nothing in this project" }}</span>
              </li>
            </ul>
            <p class="measurement-boundary">{{ projectMeasurement.boundary }}</p>
            <ul v-if="projectMeasurement.warnings?.length" class="docker-warnings">
              <li v-for="warning in projectMeasurement.warnings" :key="warning">{{ warning }}</li>
            </ul>
          </section>

          <details v-if="projectDiscovery.skipped.length" class="project-skipped">
            <summary>Recorded and not traversed ({{ projectDiscovery.skipped.length }})</summary>
            <p class="muted">These paths are listed for honesty. A skipped path is never treated as empty, absent, or reclaimable.</p>
            <ul>
              <li v-for="skip in projectDiscovery.skipped" :key="`${skip.kind}:${skip.relativePath}`">
                <strong>{{ skip.relativePath }}</strong>
                <span>{{ projectSkipLabel(skip.kind) }} — {{ skip.reason }}</span>
              </li>
            </ul>
          </details>

          <article class="panel project-boundary">
            <strong>No project cleanup path</strong>
            <p>{{ projectDiscovery.boundary }}</p>
          </article>
          <ul v-if="projectDiscovery.warnings?.length" class="docker-warnings">
            <li v-for="warning in projectDiscovery.warnings" :key="warning">{{ warning }}</li>
          </ul>
        </div>
      </section>

      <section id="developer-tools" class="provider-category" aria-labelledby="developer-tools-title">
        <div class="provider-category-heading">
          <div>
            <p class="eyebrow">Developer tools</p>
            <h2 id="developer-tools-title">Available caches</h2>
            <p class="muted">Discovery checks provider availability only. Storage is measured only after Inspect.</p>
          </div>
          <button class="secondary" :disabled="discoveryLoading" @click="refreshDeveloperProviders">{{ discoveryLoading ? "Checking…" : "Refresh detected tools" }}</button>
        </div>
        <p v-if="discoveryError" class="error" role="alert">{{ discoveryError }}</p>
        <p v-else-if="discoveryLoading" class="muted provider-loading">Checking installed developer tools…</p>
        <p v-else-if="availableProviders.length === 0" class="empty-state">No supported provider is ready to inspect on this host.</p>
        <div v-else class="provider-stack">
          <ProviderCard
            v-for="provider in availableProviders"
            :key="provider.id"
            :provider-id="provider.id"
            :provider-label="provider.label"
            :title="provider.title"
            :inspect-label="provider.inspectLabel"
            :description="provider.description"
            @inspected="refreshDeveloperProviders"
          />
        </div>
      </section>

      <section v-if="providersNeedingConfiguration.length" class="provider-category" aria-labelledby="needs-configuration-title">
        <div class="provider-category-heading">
          <div>
            <p class="eyebrow">Detected, needs setup</p>
            <h2 id="needs-configuration-title">Configuration required</h2>
          </div>
        </div>
        <div class="provider-notice-grid">
          <article v-for="provider in providersNeedingConfiguration" :key="provider.id" class="provider-notice">
            <strong>{{ provider.title }}</strong>
            <p>{{ availabilityMessage(provider.id) }}</p>
          </article>
        </div>
      </section>

      <section v-if="unavailableProviders.length" class="provider-category unavailable-providers" aria-label="Unavailable developer tools">
        <details>
          <summary>Unavailable on this machine ({{ unavailableProviders.length }})</summary>
          <p class="muted">These providers are not rendered as cleanup cards because their host tool or supported workspace prerequisite is unavailable.</p>
          <ul>
            <li v-for="entry in unavailableProviders" :key="entry.providerId">
              <strong>{{ definitionFor(entry.providerId)?.label ?? entry.providerId }}</strong>
              <span>{{ entry.message || entry.detection.message }}</span>
            </li>
          </ul>
        </details>
      </section>
    </section>
  </main>
</template>
