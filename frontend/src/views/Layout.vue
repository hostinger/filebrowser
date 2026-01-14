<template>
  <div>
    <div v-if="uploadStore.totalBytes" class="progress">
      <div
        v-bind:style="{
          width: sentPercent + '%',
        }"
      ></div>
    </div>
    <sidebar></sidebar>
    <main>
      <router-view></router-view>
      <shell
        v-if="
          enableExec && authStore.isLoggedIn && authStore.user?.perm.execute
        "
      />
    </main>
    <prompts></prompts>
    <context-menu v-if="contextMenuVisible"></context-menu>

    <upload-files></upload-files>
  </div>
</template>

<script setup lang="ts">
import { useAuthStore } from "@/stores/auth";
import { useLayoutStore } from "@/stores/layout";
import { useFileStore } from "@/stores/file";
import { useUploadStore } from "@/stores/upload";
import { useContextMenuStore } from "@/stores/contextMenu";
import Sidebar from "@/components/Sidebar.vue";
import Prompts from "@/components/prompts/Prompts.vue";
import Shell from "@/components/Shell.vue";
import UploadFiles from "@/components/prompts/UploadFiles.vue";
import ContextMenu from "@/components/files/ContextMenu.vue";
import { enableExec } from "@/utils/constants";
import { computed, watch } from "vue";
import { useRoute } from "vue-router";

const layoutStore = useLayoutStore();
const authStore = useAuthStore();
const fileStore = useFileStore();
const uploadStore = useUploadStore();
const contextMenuStore = useContextMenuStore();
const route = useRoute();

const sentPercent = computed(() =>
  ((uploadStore.sentBytes / uploadStore.totalBytes) * 100).toFixed(2)
);

watch(route, () => {
  fileStore.selected = [];
  fileStore.multiple = false;
  contextMenuStore.hide();
  if (layoutStore.currentPromptName !== "success") {
    layoutStore.closeHovers();
  }
});

const contextMenuVisible = computed(
  (): boolean =>
    (fileStore.isListing || false) && contextMenuStore.position !== null
);
</script>
