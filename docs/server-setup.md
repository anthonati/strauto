# Production server setup

The message `server configuration is incomplete` means the Go API cannot read its required server environment variables. The existing `VITE_API_URL` setting belongs to the old frontend scaffold and does not configure the API. A Strava subscription on a personal account enables creation of a developer app, but it does not provide the app's client ID and secret automatically.

## 1. Prepare the external projects

Create a Supabase project and run [`supabase/001_initial.sql`](../supabase/001_initial.sql), followed by [`supabase/002_automation_history.sql`](../supabase/002_automation_history.sql), in its SQL editor. In Supabase **Project Settings → API Keys**, create a secret key (`sb_secret_...`) for the server. The legacy service-role JWT is supported through `SUPABASE_SERVICE_ROLE_KEY`, but new deployments should use `SUPABASE_SECRET_KEY`. Never put either key in a `VITE_` variable or commit it.

Create a Strava API application in the Strava API dashboard. Set **Authorization Callback Domain** to `strauto.fortunati.dev` and obtain its client ID and client secret. The app's callback URL is `https://strauto.fortunati.dev/api/oauth_callback`.

## 2. Make the deployment ready

In the **Strauto Vercel project**, set these variables for the **Production** environment:

| Variable | Value |
| --- | --- |
| `APP_URL` | `https://strauto.fortunati.dev` |
| `STRAVA_CLIENT_ID` | Strava application client ID |
| `STRAVA_CLIENT_SECRET` | Strava application client secret |
| `SUPABASE_URL` | Supabase project URL (`https://...supabase.co`) |
| `SUPABASE_SECRET_KEY` | Supabase server secret key (`sb_secret_...`) |
| `SESSION_SECRET` | Fresh random string of at least 32 characters |

Set the remaining server variables before webhook registration: `STRAVA_WEBHOOK_VERIFY_TOKEN`, `STRAVA_WEBHOOK_SECRET`, and `WORKER_SECRET`, each with a distinct random value. Leave `STRAVA_WEBHOOK_SUBSCRIPTION_ID` empty until Strava returns it. A local example for all names is in [`apps/web/.env.example`](../apps/web/.env.example). Use Vercel's secret storage, not a checked-in environment file.

Create a **new Production deployment** after setting or changing variables; an earlier deployment does not acquire updated values. `GET https://strauto.fortunati.dev/api/health` should then return `200`. Its `status` field is `configured`; `automation_configured` remains `false` until the webhook and worker variables below are all present. If it returns `503`, check Vercel function logs for the missing variable **names**. The endpoint itself deliberately does not reveal them. A `200` means the values passed local validation; it does not prove Supabase or Strava credentials work.

## 3. Connect and process uploads

Use **Connect with Strava** on the deployed site with your own account. This checks the OAuth client and Supabase connection. Enable the mute rule after webhook registration and worker configuration below are complete.

Register one Strava webhook subscription for the API app. Send an HTTP `POST` to `https://www.strava.com/api/v3/push_subscriptions` with form fields:

| Field | Value |
| --- | --- |
| `client_id` | `STRAVA_CLIENT_ID` from the Strava app |
| `client_secret` | `STRAVA_CLIENT_SECRET` from the Strava app |
| `callback_url` | `https://strauto.fortunati.dev/hooks/strava/YOUR_STRAVA_WEBHOOK_SECRET` |
| `verify_token` | The value of `STRAVA_WEBHOOK_VERIFY_TOKEN` in Vercel |

Use the actual values, not the variable names, in the request. Strava immediately sends a verification GET to the callback and returns a numeric `id` if it succeeds. Keep the complete callback URL and request private. Each Strava app supports one subscription; if one already exists, inspect it before attempting to create another. Save the returned ID as `STRAVA_WEBHOOK_SUBSCRIPTION_ID` in Vercel, then deploy again. The same webhook subscription handles all athletes authorized to this API app. At this point `/api/health` should report `"automation_configured":true`; this confirms the variables are present, not that events are being delivered.

The verification GET authenticates with `hub.verify_token` and echoes `hub.challenge`. The registered callback has no query string: its secret is a path segment, and Vercel rewrites that segment to the handler's `key` parameter. Strava can supply its verification query without conflicting with the callback URL. Event POSTs require the callback key and the matching subscription ID.

Set a Supabase Cron HTTP job to `POST https://strauto.fortunati.dev/api/process_events` every minute with `Authorization: Bearer YOUR_WORKER_SECRET`. Store the token in Supabase Vault or its secret management. Upload a test Weight Training activity and verify Strava reports `hide_from_home: true`; confirm it no longer appears in followers' feeds. This action does not make the activity private on your profile. Check the `activity_events` table for failed rows if processing does not complete.

## 4. Roll out the automation dashboard

For an existing installation, apply `002_automation_history.sql` before deploying this release. The migration copies existing opt-ins and creates a disabled subscription for new athletes. It keeps `athletes.mute_weight_training` as a compatibility write path: a database trigger updates the subscription whenever the flag changes, so the previous application can continue running during rollout. Subscription records drive the new dashboard and worker reads. Leave that bridge in place until a later migration removes the legacy write path.

Deploy this branch only after the migration succeeds. New code depends on the subscription table and dashboard functions. A code rollback can use the previous release with the additive migration left in place. No production data needs to be deleted for rollback.

After connecting, confirm the dashboard starts with zero counts, then enable the mute rule and upload a Weight Training activity. Refresh the dashboard after processing: checked and updated should each increase by one. A nonmatching upload should add only one checked activity. Replaying the same event should not increase either total. Verify disabling preserves history and disconnecting removes it, including when deauthorization comes from Strava.

Counts begin with the new worker; historical queue results are not backfilled. Failed attempts are visible, but the dashboard is not proof the scheduler is running: verify the Cron job and one real upload separately. Track remaining work in the [backlog](../BACKLOG.md).
