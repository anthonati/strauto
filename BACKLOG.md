# Backlog

## Next: deploy and prove the MVP end to end

- Apply `supabase/002_automation_history.sql` before deploying the dashboard branch.
- Complete webhook and worker configuration. On October 25, 2026, production `/api/health` returned `200` with `automation_configured: false`; this does not identify which settings are missing or verify external connectivity.
- Register or inspect the Strava webhook, save its subscription ID, redeploy, and configure the once-per-minute worker job.
- Connect one athlete, enable the mute rule, upload Weight Training, and verify both the Strava result and dashboard counts. Also check a nonmatching upload, duplicate delivery, disable/re-enable, disconnect, and Strava deauthorization.
- Before broader use, add failed-event monitoring and completed-queue cleanup; history lives separately from the queue.

See [production setup](docs/server-setup.md) for the rollout order.

## Implemented in code: automation dashboard and run counts

**Goal:** A connected athlete can see every automation they have enabled or created, whether it is active, and how often it has run against their activities.

**Placement:** The signed-in dashboard becomes a list of automation cards. Each card shows the rule name and action, enabled state, activities checked, activities updated, and most recent run time. The first card is “Mute weight training”; later rules use the same layout.

**Data work:** Use `automation_subscriptions` records for rule reads while preserving existing opt-ins. The legacy `athletes.mute_weight_training` flag remains as a trigger-backed compatibility write path for rollout. Add a durable `automation_runs` record keyed by subscription and activity. Record one evaluation per activity, its outcome (`applied`, `skipped`, or `failed`), and timestamps. Keep these records separate from the webhook queue so queue cleanup does not erase dashboard history. Do not store activity descriptions or tokens in run records.

**Counting rule:** “Activities checked” counts unique activities whose data was fetched and evaluated by that automation. “Activities updated” counts unique activities for which Strava confirmed a successful change. Retries and duplicate webhooks update the same run record and never inflate either count. An already-muted or nonmatching activity is checked but not updated. A failed request is shown as a failure, not as a successful update.

**API and privacy:** Add an authenticated read endpoint that returns only the signed-in athlete's subscriptions and aggregate counts from Strauto's database. The dashboard must not make extra Strava API calls just to display counts. Deauthorization and disconnect remove the athlete's subscriptions and run history.

**Acceptance criteria:**

- A newly connected athlete sees the available mute rule with zero counts; existing enabled settings survive the migration.
- A successful Weight Training mute adds one checked activity and one updated activity. A nonmatching activity adds one checked activity and no update.
- Retried or duplicate events for the same subscription and activity do not increase counts.
- The view shows the latest run and failure state without exposing another athlete's data.
- Disabling a rule keeps its historical counts; disconnecting removes them.

**Validation:** Go handler tests and disposable PostgreSQL regression tests cover outcomes, counts, retries, account isolation, migration compatibility, and cleanup. Lint and production build pass. Live Strava validation remains part of rollout; no real upload has been verified in this work.
