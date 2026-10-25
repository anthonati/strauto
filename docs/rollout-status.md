# Rollout status — October 25, 2026

## Completed

- Resumed the paused `strauto` Supabase project and verified it is healthy.
- Applied `002_automation_history.sql` to the existing initial schema. There were no connected athletes. Verified the dashboard function, subscription consistency, row-level security, and service-role-only access to the new tables and RPCs.
- Deployed the dashboard and callback fixes to `https://strauto.fortunati.dev` after staged checks. Current application commit: `45355fbfd707e80cddeb51bb0cf2746dce05128d`. Current production deployment: `dpl_CrjYzJebhdQUh4FNryTUSPUySGcV`.
- Added production `STRAVA_WEBHOOK_VERIFY_TOKEN`, `STRAVA_WEBHOOK_SECRET`, and `WORKER_SECRET` as Vercel Secrets.
- Registered Strava webhook subscription `379652`, saved `STRAVA_WEBHOOK_SUBSCRIPTION_ID` in Vercel, and deployed the updated configuration. The live health endpoint reports `automation_configured: true`.
- Fixed callback registration compatibility: the public callback uses `/hooks/strava/<secret>` without an existing query string. Vercel rewrites it to the Go handler. Verification GETs authenticate with Strava's verification token; event POSTs require the callback key and subscription ID. Strava accepted and independently verified this callback.
- Enabled Supabase `pg_cron` and `pg_net`. The `strauto-process-events` job calls the production worker every minute, using `strauto_worker_secret` from Vault. Restricted public/client-role access to the HTTP queue and response tables.
- Confirmed recurring scheduled runs through 22:30 UTC succeeded and received HTTP 200 with `processed: 0` and `retrying: 0`.
- Verified the live app and health endpoint return HTTP 200, private endpoints reject unauthenticated requests, the authenticated worker reaches Supabase and returns zero pending work, and the webhook challenge succeeds with the correct credentials.
- Verified an authenticated ignored activity-update event returns HTTP 200 without creating work or changing an activity. This is a handler check, not proof of delivery from a real Strava activity.
- Go tests, Go vet, frontend lint, and the production build pass. Regression tests cover verification tokens, callback keys, subscription IDs, and challenge responses.
- The post-deployment runtime error scan returned no errors in the available five-minute window.

## Remaining

1. Connect an athlete using **Connect with Strava** and enable the mute rule. The database currently has no connected athletes.
2. Upload Weight Training and verify `hide_from_home`, feed behavior, and dashboard counts after the worker runs. No real activity has been tested yet.
3. Test nonmatching activities, retries, disable/re-enable, disconnect, and deauthorization before broader use.

The live dashboard was deployed from the feature branch's tested commit; this does not merge that branch into `master`. Preserve these changes when making the next Git-based production release.

Local `.env.local` and `.env.rollout.local` files are ignored by Git. Keep callback URLs and secret values out of documentation and logs.
