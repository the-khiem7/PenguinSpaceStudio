<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { backend, ElevationProbeMode, type ElevationStatus, type ProviderAvailability, type Scenario } from "./backend";
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
