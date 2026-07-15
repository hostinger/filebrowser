import { ref } from "vue";
import type { PostTranslationHandler, VueMessageType } from "vue-i18n";

// Phrase In-Context Editor (ICE) integration.
//
// ICE is compiled in only by the staging build (`pnpm run build:staging`,
// i.e. `vite build --mode staging`) — the production build must never
// include it, because translators edit live on top of the staging
// deployment only. The ids below are not secrets: they are served to every
// visitor of the staging deployment as part of PHRASEAPP_CONFIG.
//
// Even on staging, ICE stays dormant until a translator opts in by visiting
// any page with ?phraseapp=enabled (persisted in a cookie for a week;
// ?phraseapp=disabled clears it again).

const COOKIE_NAME = "phraseapp";
const ONE_WEEK = 7 * 24 * 60 * 60;

const phraseAccountId = "3b42d924cc735b83fa5b486acb7ed6b9";
const phraseProjectId = "f2d90a99ff37b4cf5ac4f3037eb394e3";

const phrasePrefix = "[[__";
const phraseSuffix = "__]]";
const phraseIceScriptUrl =
  "https://cdn.phrase.com/strings/plugins/editor/latest/ice/index.js";

interface PhraseAppConfig {
  accountId: string;
  projectId: string;
  phraseEnabled: boolean;
  fullReparse: boolean;
  prefix: string;
  suffix: string;
  autoLowercase: boolean;
}

declare global {
  interface Window {
    PHRASEAPP_CONFIG?: PhraseAppConfig;
  }
}

export const phraseIceEnabled = import.meta.env.MODE === "staging";

// Flips only after the CDN script has initialised its MutationObserver —
// markers rendered before ICE is watching never get edit bubbles attached.
// Reading it inside phrasePostTranslation (i.e. during component render)
// makes Vue track it, so flipping it re-renders all translated components.
const phraseIceReady = ref(false);

function applyUrlParam() {
  const url = new URL(location.href);
  const param = url.searchParams.get(COOKIE_NAME);
  if (param !== "enabled" && param !== "disabled") return;

  if (param === "enabled") {
    document.cookie = `${COOKIE_NAME}=enabled; max-age=${ONE_WEEK}; path=/`;
  } else {
    document.cookie = `${COOKIE_NAME}=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/`;
  }
  url.searchParams.delete(COOKIE_NAME);
  location.replace(url.toString());
}

function hasCookie(cookie: string) {
  return document.cookie.split(";").some((c) => c.trim() === cookie);
}

export function loadPhraseIce() {
  if (!phraseIceEnabled) return;

  applyUrlParam();

  if (!hasCookie(`${COOKIE_NAME}=enabled`)) return;

  window.PHRASEAPP_CONFIG = {
    accountId: phraseAccountId,
    projectId: phraseProjectId,
    phraseEnabled: true,
    fullReparse: true,
    prefix: phrasePrefix,
    suffix: phraseSuffix,
    autoLowercase: false,
  };

  const script = document.createElement("script");
  script.async = true;
  script.type = "module";
  script.src = phraseIceScriptUrl;
  script.onload = () => {
    setTimeout(() => {
      phraseIceReady.value = true;
      triggerIceRescanOnDomChange();
    }, 1000);
  };
  document.head.appendChild(script);
}

// ICE only re-scans the document from its own (throttled, capturing) scroll
// listener — its MutationObserver ignores added nodes and text changes. Any
// page change without a scroll (route change, async file listing, resize)
// would leave edit bubbles frozen at stale coordinates, so synthesise a
// scroll event whenever the DOM might have shifted.
function triggerIceRescanOnDomChange() {
  let timer: number | null = null;
  const trigger = () => {
    if (timer !== null) return;
    timer = window.setTimeout(() => {
      timer = null;
      document.dispatchEvent(new Event("scroll"));
    }, 200);
  };

  new MutationObserver((mutations) => {
    for (const m of mutations) {
      if (
        (m.type === "childList" && m.addedNodes.length > 0) ||
        m.type === "characterData"
      ) {
        trigger();
        return;
      }
    }
  }).observe(document.body, {
    childList: true,
    subtree: true,
    characterData: true,
  });

  if (typeof ResizeObserver !== "undefined") {
    new ResizeObserver(trigger).observe(document.body);
  }

  window.addEventListener("resize", trigger, { passive: true });
}

// ICE looks for keys decorated as [[__phrase_<key>__]] in the rendered DOM
// and overlays its editor on top of them.
export const phrasePostTranslation: PostTranslationHandler<VueMessageType> = (
  translated,
  key
) =>
  phraseIceReady.value && typeof translated === "string"
    ? `${phrasePrefix}phrase_${key}${phraseSuffix}`
    : translated;
