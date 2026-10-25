-- Run only in a fresh, disposable PostgreSQL database:
-- psql -v ON_ERROR_STOP=1 -f supabase/tests/automation_history.sql DATABASE_URL
create role anon;
create role authenticated;
create role service_role;
\ir ../001_initial.sql

insert into public.athletes (id, access_token, refresh_token, expires_at, granted_scopes, mute_weight_training)
values (1, 'test', 'test', 9999999999, 'activity:read,activity:write', true),
       (2, 'test', 'test', 9999999999, 'activity:read,activity:write', false);

\ir ../002_automation_history.sql
-- Applying twice must preserve subscriptions and history.
\ir ../002_automation_history.sql

do $$
declare
  subscription bigint;
  other_subscription bigint;
  dashboard record;
begin
  select id into strict subscription from public.automation_subscriptions where owner_id = 1;
  select id into strict other_subscription from public.automation_subscriptions where owner_id = 2;
  select * into strict dashboard from public.automation_dashboard(1);
  assert dashboard.enabled and dashboard.activities_checked = 0 and dashboard.activities_updated = 0
    and dashboard.last_run_at is null and dashboard.last_outcome is null, 'existing opt-in was lost';
  select * into strict dashboard from public.automation_dashboard(2);
  assert not dashboard.enabled and dashboard.activities_checked = 0, 'disabled opt-in was lost';

  insert into public.athletes (id, access_token, refresh_token, expires_at, granted_scopes)
  values (3, 'test', 'test', 9999999999, 'activity:read,activity:write');
  select * into strict dashboard from public.automation_dashboard(3);
  assert not dashboard.enabled and dashboard.activities_checked = 0, 'new athlete missing default rule';

  perform public.record_automation_run(1, subscription, 101, true, 'applied');
  perform public.record_automation_run(1, subscription, 101, true, 'applied');
  perform public.record_automation_run(1, subscription, 101, true, 'skipped');
  perform public.record_automation_run(1, subscription, 101, false, 'failed');
  select * into strict dashboard from public.automation_dashboard(1);
  assert dashboard.activities_checked = 1 and dashboard.activities_updated = 1 and dashboard.failed_activities = 0
    and dashboard.last_outcome = 'applied', 'duplicates changed a confirmed update';

  perform public.record_automation_run(1, subscription, 102, true, 'skipped');
  perform public.record_automation_run(1, subscription, 103, false, 'failed');
  select * into strict dashboard from public.automation_dashboard(1);
  assert dashboard.activities_checked = 2 and dashboard.activities_updated = 1 and dashboard.failed_activities = 1
    and dashboard.last_outcome = 'failed', 'failed fetch counted as checked or updated';
  perform public.record_automation_run(1, subscription, 103, true, 'failed');
  perform public.record_automation_run(1, subscription, 103, true, 'applied');
  select * into strict dashboard from public.automation_dashboard(1);
  assert dashboard.activities_checked = 3 and dashboard.activities_updated = 2 and dashboard.failed_activities = 0,
    'retry inflated counts or did not clear failure';

  perform public.record_automation_run(1, other_subscription, 104, true, 'applied');
  select * into strict dashboard from public.automation_dashboard(2);
  assert dashboard.activities_checked = 0 and dashboard.activities_updated = 0, 'cross-athlete history write';

  perform public.enqueue_activity_event(1, 105, 9, 1);
  assert (select count(*) = 1 from public.activity_events), 'enabled rule did not queue';
  delete from public.activity_events;
  assert (select count(*) = 3 from public.automation_runs), 'queue cleanup erased history';

  update public.athletes set mute_weight_training = false where id = 1;
  select * into strict dashboard from public.automation_dashboard(1);
  assert not dashboard.enabled and dashboard.activities_updated = 2, 'disabling lost history';
  perform public.enqueue_activity_event(1, 106, 9, 2);
  assert (select count(*) = 0 from public.activity_events), 'disabled rule queued an event';
  update public.athletes set mute_weight_training = true where id = 1;
  update public.athletes set access_token = 'reconnected' where id = 1;
  select * into strict dashboard from public.automation_dashboard(1);
  assert dashboard.enabled and dashboard.activities_updated = 2, 'reconnect lost settings or history';

  assert not has_table_privilege('anon', 'public.automation_runs', 'SELECT'), 'anonymous history access';
  assert not has_table_privilege('authenticated', 'public.automation_subscriptions', 'SELECT'), 'client subscription access';
  assert not has_function_privilege('anon', 'public.automation_dashboard(bigint)', 'EXECUTE'), 'anonymous dashboard access';
  assert not has_function_privilege('authenticated', 'public.record_automation_run(bigint,bigint,bigint,boolean,text)', 'EXECUTE'), 'client history writes';
  assert (select relrowsecurity from pg_class where oid = 'public.automation_runs'::regclass), 'history RLS disabled';

  delete from public.athletes where id = 1;
  assert (select count(*) = 0 from public.automation_runs), 'disconnect left history';
  assert (select count(*) = 0 from public.automation_subscriptions where owner_id = 1), 'disconnect left subscriptions';
  assert (select count(*) = 1 from public.automation_subscriptions where owner_id = 2), 'disconnect removed another athlete';
end;
$$;
