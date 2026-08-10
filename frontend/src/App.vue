<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { backend, type Scenario } from "./backend";

const stage = ref("Connecting to the local service…");
const safetyMessage = ref("All cleanup requires a reviewed plan.");
const scenario = ref<Scenario | null>(null);
const error = ref("");
const running = ref(false);

const reclaimed = computed(() => scenario.value?.verification.reclaimedActual.bytes ?? 0);
const formattedReclaimed = computed(() => `${(reclaimed.value / 1024 / 1024).toFixed(2)} MiB`);

onMounted(async () => {
  try {
    const dashboard = await backend.dashboard();
    stage.value = dashboard.stage;
    safetyMessage.value = dashboard.safetyMessage;
  } catch (cause) {
    error.value = `Backend connection failed: ${String(cause)}`;
  }
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
        <article><span>Detected providers</span><strong>1</strong><small>Fixture only in M1</small></article>
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
          <h2>Nothing destructive is wired yet.</h2>
          <ul>
            <li>Go owns provider semantics and execution.</li>
            <li>Confirmation is mandatory before execution.</li>
            <li>The M1 fixture mutates memory only.</li>
            <li>SQLite records verified outcomes locally.</li>
          </ul>
        </aside>
      </section>
    </section>
  </main>
</template>
