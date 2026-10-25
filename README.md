# Strauto

Strauto is a small Strava automation service. Athletes connect their account and enable rules for future uploads. The first rule mutes new `WeightTraining` activities in home and club feeds with Strava's `hide_from_home` activity field. Muting does **not** make an activity private: it remains visible on the athlete's profile according to their Strava visibility setting. The Strava activity update API does not expose an “Only You” setting.

## Current implementation

- `apps/web` is a React/Vite UI with Go API functions. The UI connects through Strava OAuth, shows automation cards with unique activity counts, latest outcomes and failure indicators, and supports disconnecting.
- `apps/web/server` validates OAuth state and scopes, keeps Strava tokens server side, signs session cookies, receives Strava webhooks, and processes queued uploads.
- `supabase/001_initial.sql` stores connected athletes and a durable event queue. Only the server's service role key can access these tables.
- `supabase/002_automation_history.sql` adds typed subscriptions and durable per-activity run history. Dashboard reads use only Strauto's database and are scoped to the signed-in athlete.
- `supabase/003_queue_operations.sql` records event completion time and adds server-only queue health and bounded cleanup functions. Cleanup removes successful deliveries older than 30 days while preserving failed work and automation history.
- Activity create webhooks queue work quickly. A scheduled HTTP call to `/api/process_events` claims up to three events, refreshes expiring tokens, checks activity ownership and `sport_type`, and sends `hide_from_home: true` only for matching uploads. Failed events retry up to five times.

The first rule is opt-in and applies to future uploads after webhook registration. There is no backfill. Processing is asynchronous, so an activity may briefly appear in feeds before it is muted. For sensitive uploads, set Strava's default activity visibility appropriately before uploading.

See [architecture](docs/architecture.md) for the request flow and storage model.

## Setup

1. Create a Supabase project and apply the SQL migrations in order: [`001_initial.sql`](supabase/001_initial.sql), [`002_automation_history.sql`](supabase/002_automation_history.sql), and [`003_queue_operations.sql`](supabase/003_queue_operations.sql). Existing installations need only their unapplied migrations. Use a server-only Supabase secret key; the legacy service-role key also works.
2. Register a Strava API app. Set its authorization callback domain to the domain in `APP_URL` (use `localhost` for local development). Copy the template in [`apps/web/.env.example`](apps/web/.env.example) to a private environment file outside this checkout for local development. Set production values in Vercel's environment settings. Generate fresh random values for `SESSION_SECRET`, `STRAVA_WEBHOOK_VERIFY_TOKEN`, `STRAVA_WEBHOOK_SECRET`, and `WORKER_SECRET`.
3. Locally, from `apps/web`, export the variables from that private environment file and run `go run ./api/_local` in one terminal and `pnpm dev` in another. Vite proxies `/api` to the Go server on port 8080. `APP_URL` should be `http://localhost:5173`.
4. Configure the Vercel project root as `apps/web`. Set `APP_URL` to your HTTPS origin, such as `https://strauto.example.com`, and use that hostname as the Strava app's authorization callback domain. Register **one** Strava webhook subscription with callback URL `https://strauto.example.com/hooks/strava/YOUR_STRAVA_WEBHOOK_SECRET` and the verify token from your environment. Save the returned subscription ID as `STRAVA_WEBHOOK_SUBSCRIPTION_ID` in Vercel. Strava sends all connected athletes' events to that one subscription.
5. In Supabase Cron, create an HTTP job that calls `POST https://strauto.example.com/api/process_events` every minute with `Authorization: Bearer YOUR_WORKER_SECRET`. Keep that secret in Supabase Vault or the Cron dashboard's secret management; do not put it in client code or a checked-in SQL file. You can make the same call manually for an end-to-end local test.

The API expects `activity:read` and `activity:write` scopes. It intentionally requests no access to private activities. The webhook callback has a secret path segment because Strava's webhook events are not signed. Keep the full callback URL private. Disconnect revokes the Strava refresh token and deletes the athlete's data, queued events, subscriptions, and run history.

For the exact production order and troubleshooting of “server configuration is incomplete,” see [server setup](docs/server-setup.md). `/api/health` returns `200` once the minimum environment variables are valid and `503` otherwise; it does not check whether the external accounts or webhook are connected.

## Verification

From `apps/web`, run `go test ./...`, `go vet ./...`, `pnpm lint`, and `pnpm build`. A live end-to-end check also needs a Strava developer app, Supabase project, deployed webhook callback, and one authorized athlete; these credentials are not in the repository.


Database regression tests are in [`supabase/tests/queue_operations.sql`](supabase/tests/queue_operations.sql), which also runs the automation-history suite. Run with `psql -v ON_ERROR_STOP=1 -f supabase/tests/queue_operations.sql DATABASE_URL` from the repository root, using a fresh disposable PostgreSQL cluster with permission to create roles. The script creates test roles and athletes; never run it against a real Supabase project. It checks migration reruns, preserved opt-ins, unique counts, retries, account isolation, permissions, deletion cascades, queue health, and retention boundaries.

History starts when this worker version is deployed; previous queue rows are not used to reconstruct counts. A confirmed update counts only after its history is saved. If Strava applies an update but the response or subsequent database write is lost, the retry may find an already-muted activity and count it as checked without attributing an update to Strauto. Counts favor confirmed, persisted outcomes over guessing who made a change.
