# Strauto

Strauto is a small Strava automation service. Athletes connect their account and enable rules for future uploads. The first rule mutes new `WeightTraining` activities in home and club feeds with Strava's `hide_from_home` activity field. Muting does **not** make an activity private: it remains visible on the athlete's profile according to their Strava visibility setting. The Strava activity update API does not expose an “Only You” setting.

## Current implementation

- `apps/web` is a React/Vite UI with Go API functions. The UI connects through Strava OAuth, shows the active rule and supports disconnecting.
- `apps/web/server` validates OAuth state and scopes, keeps Strava tokens server side, signs session cookies, receives Strava webhooks, and processes queued uploads.
- `supabase/001_initial.sql` stores connected athletes and a durable event queue. Only the server's service role key can access these tables.
- Activity create webhooks queue work quickly. A scheduled HTTP call to `/api/process_events` claims up to three events, refreshes expiring tokens, checks activity ownership and `sport_type`, and sends `hide_from_home: true` only for matching uploads. Failed events retry up to five times.

The first rule is opt-in and applies to future uploads after webhook registration. There is no backfill. Processing is asynchronous, so an activity may briefly appear in feeds before it is muted. For sensitive uploads, set Strava's default activity visibility appropriately before uploading.

## Plan of attack

1. **Single-athlete proof:** Create a Strava developer app and a Supabase project, run the SQL migration, configure OAuth and a webhook, then verify one Weight Training upload end to end. Inspect Strava's returned `hide_from_home` value and the activity's feed behavior.
2. **Reliable small service:** Deploy `apps/web` to Vercel, schedule the worker through Supabase Cron, monitor failed queue rows and Strava rate limits, and periodically delete completed events. Test disconnect and deauthorization handling.
3. **More automations:** Move subscriptions into a typed rule table, expose only fields in Strava's documented `UpdatableActivity` model (name, description, gear, commute, trainer, sport type, mute), and add conflict handling when multiple rules edit the same activity. Each rule should preview its exact effect before opt-in.
4. **More athletes:** Increase Strava's athlete capacity in its API dashboard and seek review when needed. Add rate-aware throttling and operational alerts before inviting users broadly.

The hosting path can fit free tiers at small scale, subject to their limits. You already have the [Strava subscription required to create an API app](https://developers.strava.com/docs/getting-started/), so that prerequisite adds no new subscription cost. New apps start with one connected athlete; the [API dashboard can raise that to ten](https://developers.strava.com/docs/rate-limits/). [Vercel Hobby cron runs only daily](https://vercel.com/docs/cron-jobs/usage-and-pricing), so the near-real-time worker needs [Supabase Cron](https://supabase.com/docs/guides/cron) or another free scheduler. [Supabase may pause a free project for low activity](https://supabase.com/docs/guides/platform/free-project-pausing).

See [architecture and domain layout](docs/architecture.md) for the existing `strauto.fortunati.dev` deployment and how it can coexist with a future portfolio site at `fortunati.dev`. The [backlog](BACKLOG.md) tracks the automation dashboard and run history.

## Setup

1. Create a Supabase project and run [`supabase/001_initial.sql`](supabase/001_initial.sql) in its SQL editor. Use a server-only Supabase secret key; the legacy service-role key also works.
2. Register a Strava API app. Set its authorization callback domain to the domain in `APP_URL` (use `localhost` for local development). Copy the variables in [`apps/web/.env.example`](apps/web/.env.example) into `apps/web/.env.local` for local development, and set them as server environment variables in Vercel for deployment. Generate fresh random values for `SESSION_SECRET`, `STRAVA_WEBHOOK_VERIFY_TOKEN`, `STRAVA_WEBHOOK_SECRET`, and `WORKER_SECRET`.
3. Locally, from `apps/web`, run `set -a; source .env.local; set +a; go run ./api/_local` in one terminal and `pnpm dev` in another. Vite proxies `/api` to the Go server on port 8080. `APP_URL` should be `http://localhost:5173`.
4. The existing Vercel project serves `strauto.fortunati.dev` from `apps/web`. Set its server-side `APP_URL` to `https://strauto.fortunati.dev` before deploying this branch. Set the Strava app's authorization callback domain to `strauto.fortunati.dev`. Register **one** Strava webhook subscription with callback URL `https://strauto.fortunati.dev/api/webhook?key=YOUR_STRAVA_WEBHOOK_SECRET` and the verify token from your environment. Save the returned subscription ID as `STRAVA_WEBHOOK_SUBSCRIPTION_ID` in Vercel. Strava sends all connected athletes' events to that one subscription.
5. In Supabase Cron, create an HTTP job that calls `POST https://strauto.fortunati.dev/api/process_events` every minute with `Authorization: Bearer YOUR_WORKER_SECRET`. Keep that secret in Supabase Vault or the Cron dashboard's secret management; do not put it in client code or a checked-in SQL file. You can make the same call manually for an end-to-end local test.

The API expects `activity:read` and `activity:write` scopes. It intentionally requests no access to private activities. The webhook callback has a secret URL parameter because Strava's webhook events are not signed. Keep the full callback URL private. Disconnect revokes the Strava refresh token and deletes the athlete's data and queued events.

For the exact production order and troubleshooting of “server configuration is incomplete,” see [server setup](docs/server-setup.md). `/api/health` returns `200` once the minimum environment variables are valid and `503` otherwise; it does not check whether the external accounts or webhook are connected.

## Verification

From `apps/web`, run `go test ./...`, `go vet ./...`, `pnpm lint`, and `pnpm build`. A live end-to-end check also needs a Strava developer app, Supabase project, deployed webhook callback, and one authorized athlete; these credentials are not in the repository.
