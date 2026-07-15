# Phrase In-Context Editor (ICE)

Staging-only integration with the [Phrase In-Context Editor](https://support.phrase.com/hc/en-us/articles/5784095916188)
so translators can edit strings directly on the staging deployment
([HSTG-21088](https://hostingers.atlassian.net/browse/HSTG-21088)). It follows
the same convention as hpanel/hcart/hwebsites: `PHRASEAPP_CONFIG` global,
`cdn.phrase.com` editor script, `[[__ __]]` decorators around translation keys.

## Two builds

There is no separate staging frontend deployment — the frontend is embedded
into the single Go binary. ICE is therefore compiled in (or out) at build time,
keyed on the Vite build mode:

| Command              | ICE | Use for                           |
| -------------------- | --- | --------------------------------- |
| `make build`         | no  | production — ICE fully eliminated |
| `make build-staging` | yes | staging shared-hosting servers    |

The Phrase account and project ids are constants in `src/i18n/phrase.ts`
(they are not secrets — the staging build serves them to every visitor as
part of `PHRASEAPP_CONFIG`). The project id comes from hTranslate under
Manage Brand > WH Tools > Phrase Project ID.

## Using it on staging

Even in the staging build ICE stays dormant so regular staging users are not
affected. Translators opt in per browser:

- open any page with `?phraseapp=enabled` — sets a cookie for a week and
  reloads without the parameter
- `?phraseapp=disabled` turns it off again
- sessions with the `hostinger_qa_automation=1` cookie never activate ICE

To try it locally, run the dev server in staging mode:
`pnpm run dev -- --mode staging`.

## Implementation notes

Everything lives in `src/i18n/phrase.ts` (wired up in `src/i18n/index.ts`):

- translation keys are decorated via a vue-i18n `postTranslation` hook, but
  only after the CDN script has initialised its MutationObserver (reactive
  flag flipped on script load + 1s) — decorating earlier leaves markers
  without edit bubbles
- ICE only re-scans the page on scroll events, so a synthetic scroll is
  dispatched on DOM mutations/resizes to keep edit bubbles positioned after
  async content loads (same workaround as hpanel)
