# Fixed: console delete-instance modal + logout alignment (2026-09-03)

Fixed in creaves-console commits `598d950`, `c5b9f13`, `3b81ffc`.
Validated with agent-browser.

## creaves-console: Delete instance — FIXED
Deleting an instance does not do anything - pressing delete just refreshes the page

**Root cause**: the delete modal script bound `show.bs.modal` with a
native `addEventListener`, but Bootstrap 4 fires its modal events through
jQuery, so the handler never ran. The form `action` stayed empty and the
page just reloaded on submit.

**Fix**: bind via `window.jQuery(modal).on('show.bs.modal', ...)` with a
jQuery guard, in all 4 locales of `templates/instances/index.plush.*.html`.
Verified end-to-end: action set to `/instances/<id>/cleanup`, typed
confirmation submits, instance data purged, flash "Instance cleaned;
trigger a full resync from Creaves".

## creaves-console: logout in menu is not at same level — FIXED
The logout button in the menu is not at the same level as the other menu items - it's a couple pixel higher

**Root cause**: logout submit button used `btn btn-link nav-link p-0`;
`p-0` stripped the nav-link padding and `btn-link` changed font metrics.

**Fix**: plain `.nav-link` button with `background:none;border:none` in
all 4 application templates. Verified: all navbar links at top=8px,
height=40px.
