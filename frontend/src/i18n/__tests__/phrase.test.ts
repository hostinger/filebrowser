import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

interface ScriptStub {
  type?: string;
  async?: boolean;
  src?: string;
  onload?: () => void;
}

let appendedScripts: ScriptStub[];
let documentStub: {
  cookie: string;
  body: object;
  createElement: () => ScriptStub;
  dispatchEvent: (event: unknown) => void;
  head: { appendChild: (el: ScriptStub) => void };
};
let locationStub: { href: string; replace: ReturnType<typeof vi.fn> };

beforeEach(() => {
  appendedScripts = [];
  documentStub = {
    cookie: "",
    body: {},
    createElement: () => ({}) as ScriptStub,
    dispatchEvent: () => {},
    head: {
      appendChild: (el: ScriptStub) => {
        appendedScripts.push(el);
      },
    },
  };
  locationStub = {
    href: "https://staging.example.com/files/",
    replace: vi.fn(),
  };
  vi.stubGlobal("window", {
    setTimeout: globalThis.setTimeout.bind(globalThis),
    addEventListener: () => {},
  });
  vi.stubGlobal("document", documentStub);
  vi.stubGlobal("location", locationStub);
  vi.stubGlobal(
    "MutationObserver",
    class {
      observe() {}
      disconnect() {}
    }
  );
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllEnvs();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

// phraseIceEnabled is computed at module scope from import.meta.env.MODE, so
// each case stubs the mode first and imports a fresh copy of the module.
async function importPhrase(mode: string) {
  vi.resetModules();
  vi.stubEnv("MODE", mode);
  return await import("../phrase");
}

describe("loadPhraseIce", () => {
  it("does nothing outside staging builds", async () => {
    const phrase = await importPhrase("production");
    documentStub.cookie = "phraseapp=enabled";

    expect(phrase.phraseIceEnabled).toBe(false);

    phrase.loadPhraseIce();

    expect(window.PHRASEAPP_CONFIG).toBeUndefined();
    expect(appendedScripts).toHaveLength(0);
  });

  it("stays dormant until the translator opts in via cookie", async () => {
    const phrase = await importPhrase("staging");

    phrase.loadPhraseIce();

    expect(window.PHRASEAPP_CONFIG).toBeUndefined();
    expect(appendedScripts).toHaveLength(0);
  });

  it("persists the opt-in from ?phraseapp=enabled and cleans the url", async () => {
    const phrase = await importPhrase("staging");
    locationStub.href = "https://staging.example.com/files/?phraseapp=enabled";

    phrase.loadPhraseIce();

    expect(documentStub.cookie).toContain("phraseapp=enabled");
    expect(locationStub.replace).toHaveBeenCalledWith(
      "https://staging.example.com/files/"
    );
  });

  it("sets PHRASEAPP_CONFIG and injects the ICE script when opted in", async () => {
    const phrase = await importPhrase("staging");
    documentStub.cookie = "phraseapp=enabled";

    phrase.loadPhraseIce();

    expect(window.PHRASEAPP_CONFIG).toMatchObject({
      accountId: "3b42d924cc735b83fa5b486acb7ed6b9",
      projectId: "f2d90a99ff37b4cf5ac4f3037eb394e3",
      phraseEnabled: true,
      prefix: "[[__",
      suffix: "__]]",
    });
    expect(appendedScripts).toHaveLength(1);
    expect(appendedScripts[0]).toMatchObject({
      async: true,
      type: "module",
      src: "https://cdn.phrase.com/strings/plugins/editor/latest/ice/index.js",
    });
  });
});

describe("phrasePostTranslation", () => {
  it("decorates translations only after the ICE script has loaded", async () => {
    vi.useFakeTimers();
    const phrase = await importPhrase("staging");
    documentStub.cookie = "phraseapp=enabled";

    phrase.loadPhraseIce();

    // ICE's MutationObserver is not watching yet — no decoration
    expect(phrase.phrasePostTranslation("Hello", "buttons.save")).toBe("Hello");

    appendedScripts[0].onload?.();
    vi.advanceTimersByTime(1000);

    expect(phrase.phrasePostTranslation("Hello", "buttons.save")).toBe(
      "[[__phrase_buttons.save__]]"
    );
  });

  it("passes non-string messages through untouched", async () => {
    const phrase = await importPhrase("staging");
    const message = [{ type: "text" }];

    expect(phrase.phrasePostTranslation(message as never, "key")).toBe(message);
  });
});
