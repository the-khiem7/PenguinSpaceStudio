<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { backend, ElevationProbeMode, type ElevationStatus, type Scenario } from "./backend";
import ProviderCard from "./components/ProviderCard.vue";

const stage = ref("Connecting to the local service…");
const safetyMessage = ref("All cleanup requires a reviewed plan.");
const scenario = ref<Scenario | null>(null);
const error = ref("");
const running = ref(false);
const elevation = ref<ElevationStatus | null>(null);
const elevationStarting = ref(false);
const detectedDeveloperProviders = ref<Record<string, string>>({});
let elevationPoller: ReturnType<typeof setInterval> | undefined;

const reclaimed = computed(() => scenario.value?.verification.reclaimedActual.bytes ?? 0);
const formattedReclaimed = computed(() => `${(reclaimed.value / 1024 / 1024).toFixed(2)} MiB`);
const elevationBusy = computed(() =>
  elevationStarting.value ||
  (elevation.value !== null && !["succeeded", "failed", "cancelled", "timed-out"].includes(elevation.value.state)),
);
const detectedProviderLabels = computed(() => Object.values(detectedDeveloperProviders.value));
const detectedProviders = computed(() => 1 + detectedProviderLabels.value.length);
const detectedProviderSummary = computed(() =>
  detectedProviderLabels.value.length > 0 ? `Fixture + ${detectedProviderLabels.value.join(" + ")}` : "Fixture; inspect providers to detect them",
);

onMounted(async () => {
  try {
    const dashboard = await backend.dashboard();
    stage.value = dashboard.stage;
    safetyMessage.value = dashboard.safetyMessage;
  } catch (cause) {
    error.value = `Backend connection failed: ${String(cause)}`;
  }

  elevationPoller = setInterval(async () => {
    const status = await backend.elevationStatus();
    if (status.id) elevation.value = status;
  }, 500);
});

onBeforeUnmount(() => {
  if (elevationPoller) clearInterval(elevationPoller);
});

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

function recordProviderDetection(payload: { providerId: string; label: string; detected: boolean }) {
  const next = { ...detectedDeveloperProviders.value };
  if (payload.detected) next[payload.providerId] = payload.label;
  else delete next[payload.providerId];
  detectedDeveloperProviders.value = next;
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
            <button :disabled="running" @click="runFixture">
              {{ running ? "Running…" : "Run safe fixture" }}
            </button>
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

      <section class="provider-stack" id="developer-tools" aria-label="Developer tool providers">
        <ProviderCard
          provider-id="bun.global-cache"
          provider-label="Bun"
          title="Bun global module cache"
          inspect-label="Inspect Bun cache"
          description="Version-aware inspection, reviewed cleanup, and post-operation logical measurement. Physical reclaim may be lower because Bun can use hardlinks."
          @detection="recordProviderDetection"
        />
        <ProviderCard
          provider-id="npm.global-cache"
          provider-label="npm"
          title="npm managed content cache"
          inspect-label="Inspect npm cache"
          description="Measures only npm-managed _cacache content. Logs and npx cache are outside this action; cleanup remains Review because npm requires --force."
          @detection="recordProviderDetection"
        />
        <ProviderCard
          provider-id="pnpm.global-store"
          provider-label="pnpm"
          title="pnpm configured store"
          inspect-label="Inspect pnpm store"
          description="An explicit storeDir can be measured and pruned; default per-disk stores require project-root context. Pruneable bytes remain unavailable before execution."
          @detection="recordProviderDetection"
        />
        <ProviderCard
          provider-id="uv.global-cache"
          provider-label="uv"
          title="uv global cache"
          inspect-label="Inspect uv cache"
          description="Prunes unused entries and centralized project environments. Total cache bytes are observable, but reclaimable bytes remain unavailable before execution."
          @detection="recordProviderDetection"
        />
      </section>
    </section>
  </main>
</template>
