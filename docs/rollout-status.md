# Rollout status — October 25, 2026

## Completed

- Resumed the paused `strauto` Supabase project and verified it is healthy.
- Applied `002_automation_history.sql` to the existing initial schema. There were no connected athletes. Verified the dashboard function, subscription consistency, row-level security, and service-role-only access to the new tables and RPCs.
- Deployed application commit `208256796fc03b408065efd34a31fdc2c7cbbc01` to production and promoted it to `https://strauto.fortunati.dev` after staged checks. Deployment: `dpl_EAYPt8SN2Yfd8vcuvQ8FsHoA9yFA`.
- Added production `STRAVA_WEBHOOK_VERIFY_TOKEN`, `STRAVA_WEBHOOK_SECRET`, and `WORKER_SECRET` as Vercel Secrets.
- Enabled Supabase `pg_cron` and `pg_net`. The `strauto-process-events` job calls the production worker every minute, using `strauto_worker_secret` from Vault. Restricted public/client-role access to the HTTP queue and response tables.
- Confirmed the scheduled run at 22:17 UTC succeeded and received HTTP 200 with `processed: 0` and `retrying: 0`.
- Verified the live app and health endpoint return HTTP 200, private endpoints reject unauthenticated requests, the authenticated worker reaches Supabase and returns zero pending work, and the webhook challenge succeeds with the correct credentials.
- The post-deployment runtime error scan returned no errors in the available five-minute window.

## Remaining

1. Supply the Strava app client ID and secret locally, inspect existing subscriptions, and register the production callback if none exists. Vercel's production client secret is unreadable after storage.
2. Set the returned `STRAVA_WEBHOOK_SUBSCRIPTION_ID` in Vercel and redeploy. Until then, `/api/health` correctly reports `automation_configured: false`.
3. Connect an athlete, enable the mute rule, upload Weight Training, and verify `hide_from_home` and dashboard counts. Test nonmatching activities, retries, disable/re-enable, disconnect, and deauthorization before broader use.

The live dashboard was deployed from the feature branch's tested commit; this does not merge that branch into `master`. Preserve these changes when making the next Git-based production release.

Local `.env.local` and `.env.rollout.local` files are ignored by Git. Keep callback URLs and secret values out of documentation and logs.
