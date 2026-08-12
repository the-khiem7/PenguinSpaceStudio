<script setup lang="ts">
import { computed, ref } from "vue";
import { backend, type ProviderCleanupOutcome, type ProviderInspection } from "../backend";

const props = defineProps<{
  providerId: string;
  providerLabel: string;
  title: string;
  inspectLabel: string;
  description: string;
}>();

const emit = defineEmits<{
  detection: [payload: { providerId: string; label: string; detected: boolean }];
}>();

const inspection = ref<ProviderInspection | null>(null);
const inspecting = ref(false);
const reviewing = ref(false);
const cleaning = ref(false);
const outcome = ref<ProviderCleanupOutcome | null>(null);
const error = ref("");
const item = computed(() => inspection.value?.scan.items[0]);
const action = computed(() => inspection.value?.plan.actions[0]);
const formattedBytes = computed(() => formatBytes(item.value?.measured.bytes ?? 0));

async function inspectProvider() {
  inspecting.value = true;
  error.value = "";
  try {
    inspection.value = await backend.inspectDeveloperProvider(props.providerId);
    reviewing.value = false;
    outcome.value = null;
    emit("detection", { providerId: props.providerId, label: props.providerLabel, detected: inspection.value.detection.detected });
  } catch (cause) {
    error.value = `${props.providerLabel} inspection failed: ${String(cause)}`;
  } finally {
    inspecting.value = false;
  }
}

async function executeCleanup() {
  cleaning.value = true;
  error.value = "";
  try {
    outcome.value = await backend.executeDeveloperProvider(props.providerId);
    reviewing.value = false;
    inspection.value = await backend.inspectDeveloperProvider(props.providerId);
  } catch (cause) {
    error.value = `${props.providerLabel} cleanup failed: ${String(cause)}`;
  } finally {
    cleaning.value = false;
  }
}

function formatBytes(bytes: number) {
  if (bytes === 0) return "0 B";
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  const exponent = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const value = bytes / 1024 ** exponent;
  return `${value.toLocaleString(undefined, { maximumSignificantDigits: 3 })} ${units[exponent]}`;
}
</script>

<template>
  <section class="panel provider-panel" :aria-labelledby="`${providerId}-title`">
    <div class="panel-heading">
      <div>
        <p class="eyebrow">M2 provider slice</p>
        <h2 :id="`${providerId}-title`">{{ title }}</h2>
      </div>
      <button :disabled="inspecting" @click="inspectProvider">{{ inspecting ? "Inspecting…" : inspectLabel }}</button>
    </div>
    <p class="muted">{{ description }}</p>
    <div v-if="inspection" class="provider-result">
      <article>
        <span>Detection</span>
        <strong>{{ inspection.detection.detected ? `${providerLabel} ${inspection.detection.version || "unknown"}` : "Not detected" }}</strong>
        <small>{{ inspection.detection.message }}</small>
      </article>
      <article>
        <span>Measured cache</span>
        <strong>{{ item ? formattedBytes : "—" }}</strong>
        <small>{{ item?.location || "No cache path scanned" }}</small>
      </article>
      <article>
        <span>Reviewable plan</span>
        <strong>{{ action ? `${action.risk} · ${action.recoveryCost}` : "No plan" }}</strong>
        <small>{{ action?.consequence || "Unsupported or unavailable versions never produce a plan." }}</small>
      </article>
    </div>
    <div v-if="action && inspection?.executionEnabled" class="provider-actions">
      <button v-if="!reviewing" class="secondary" :disabled="cleaning" @click="reviewing = true">Review {{ providerLabel }} cleanup</button>
      <div v-else class="cleanup-confirmation" role="group" :aria-label="`Confirm ${providerLabel} cleanup`">
        <p><strong>Confirm {{ title.toLowerCase() }} cleanup?</strong> {{ action.consequence }}</p>
        <div>
          <button class="secondary" :disabled="cleaning" @click="reviewing = false">Cancel</button>
          <button :disabled="cleaning" @click="executeCleanup">{{ cleaning ? "Cleaning…" : "Confirm and clear" }}</button>
        </div>
      </div>
    </div>
    <p v-if="outcome" class="provider-outcome">{{ outcome.execution.message }} Logical bytes removed: {{ formatBytes(outcome.verification.reclaimedActual.bytes) }}.</p>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
  </section>
</template>
