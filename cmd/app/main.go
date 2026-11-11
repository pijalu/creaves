package main

import (
	"log"

	"creaves/actions"
)

// main is the starting point for your Buffalo application.
// You can feel free and add to this `main` method, change
// what it does, etc...
// All we ask is that, at some point, you make sure to
// call `app.Serve()`, unless you don't want to start your
// application that is. :)
func main() {
	app := actions.App()

	// Register webhook graceful shutdown
	actions.RegisterWebhookShutdown(app)

	// Load config and start the webhook delivery worker at boot so that
	// events queued before a restart (or while the app was down) are still
	// forwarded once the webhook is configured.
	actions.InitWebhookAtBoot()

	// Care-plan startup converter (docs/care-expert.md §8.1): one-shot,
	// idempotent legacy→care-plan conversion. Runs after migrations (the
	// deploy entrypoint executes `/bin/app migrate` before this binary)
	// and before the HTTP server accepts traffic; a failure aborts the
	// boot so a half-converted database can never serve.
	if err := actions.RunCarePlanConverterAtBoot(); err != nil {
		log.Fatal(err)
	}

	// startup_v3 one-shot migration (bugs.md t10, 2026-10-27): retire the
	// converter's cage-name/species-sweep cluster rules in favor of
	// per-animal plans — "cage names are per centers". Same failure
	// contract: a failed migration aborts the boot.
	if err := actions.RunCarePlanMigrationAtBoot(); err != nil {
		log.Fatal(err)
	}

	// startup_v4 one-shot feeding completion (bugs.md BUG-10, 2026-10-14):
	// convert the legacy feeding rows the startup_v2 converter skipped
	// (NULL start/end, period-only, diet-text-only) into per-animal
	// fallback plans — "all replaced action must be converted".
	if err := actions.RunCarePlanFeedingCompletionAtBoot(); err != nil {
		log.Fatal(err)
	}

	// startup_v5 one-shot feeding repair + validation (bugs.md BUG-50,
	// 2026-10-27): converge converter-owned feeding plans to the corrected
	// BUG-50 semantics (start==end rows → plain single-slot schedule,
	// legacy grace period/2) and persist the per-animal validation report
	// against the original legacy columns — "All animals must be validated
	// with original information!". Human-authored plans are never touched.
	if err := actions.RunCarePlanFeedingRepairAtBoot(); err != nil {
		log.Fatal(err)
	}

	// Treatment-series refresh (bugs.md BUG-52, 2026-10-27): NOT
	// marker-guarded — the startup_v2 converter converted only the
	// treatments that existed at conversion time, so any series
	// prescribed later silently had no plan. The conversion is
	// idempotent (per-series name guard) and future-rows-only, so this
	// is cheap on every boot and closes the coverage hole.
	if err := actions.RunCarePlanTreatmentRefreshAtBoot(); err != nil {
		log.Fatal(err)
	}

	if err := app.Serve(); err != nil {
		log.Fatal(err)
	}
}

/*
# Notes about `main.go`

## SSL Support

We recommend placing your application behind a proxy, such as
Apache or Nginx and letting them do the SSL heavy lifting
for you. https://gobuffalo.io/en/docs/proxy

## Buffalo Build

When `buffalo build` is run to compile your binary, this `main`
function will be at the heart of that binary. It is expected
that your `main` function will start your application using
the `app.Serve()` method.

*/
