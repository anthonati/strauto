-- Run only in a fresh, disposable PostgreSQL database with role creation rights:
-- psql -v ON_ERROR_STOP=1 -f supabase/tests/queue_operations.sql DATABASE_URL
-- Includes the existing migration and dashboard regression suite.
\ir automation_history.sql

insert into public.activity_events
  (owner_id, activity_id, subscription_id, event_time, status, created_at, next_attempt_at)
values (2, 201, 9, 1, 'done', now() - interval '90 days', now() - interval '60 days');

\ir ../003_queue_operations.sql
create temp table original_completion as
select completed_at from public.activity_events where activity_id = 201;
\ir ../003_queue_operations.sql

begin;
do $$
declare
  health record;
  dashboard record;
  subscription bigint;
  completed timestamptz;
begin
  select completed_at into strict completed from public.activity_events where activity_id = 201;
  assert completed >= now() - interval '1 minute', 'legacy completion was guessed from old timestamps';
  assert completed = (select completed_at from original_completion), 'migration rerun changed completion time';

  insert into public.activity_events
    (owner_id, activity_id, subscription_id, event_time, status, next_attempt_at, completed_at)
  values
    (2, 202, 9, 1, 'pending', now() - interval '40 minutes', null),
    (2, 203, 9, 1, 'processing', now() - interval '20 minutes', null),
    (2, 204, 9, 1, 'failed', now() - interval '90 days', now() - interval '90 days'),
    (2, 205, 9, 1, 'done', now(), now() - interval '29 days'),
    (2, 206, 9, 1, 'done', now(), now() - interval '30 days'),
    (2, 207, 9, 1, 'done', now(), now() - interval '31 days'),
    (2, 208, 9, 1, 'done', now(), now() - interval '40 days'),
    (2, 209, 9, 1, 'pending', now() + interval '1 hour', null),
    (2, 210, 9, 1, 'processing', now() + interval '5 minutes', null);

  select * into strict health from public.activity_queue_health();
  assert health.pending = 2 and health.processing = 2 and health.done = 5 and health.failed = 1,
    'incorrect queue status counts';
  assert health.due = 2 and health.oldest_due_at = now() - interval '40 minutes',
    'due work must include expired leases and exclude future retries';
  assert (select completed_at is null from public.activity_events where activity_id = 204),
    'failed work received a successful completion timestamp';

  select id into strict subscription from public.automation_subscriptions where owner_id = 2;
  perform public.record_automation_run(2, subscription, 207, true, 'applied');
  perform public.record_automation_run(2, subscription, 208, true, 'skipped');

  assert public.cleanup_activity_events(0) = 0, 'zero batch deleted rows';
  assert public.cleanup_activity_events(-1) = 0, 'negative batch deleted rows';
  assert public.cleanup_activity_events(1) = 1, 'cleanup ignored batch size';
  assert not exists (select 1 from public.activity_events where activity_id = 208), 'oldest completed row was not removed';
  assert exists (select 1 from public.activity_events where activity_id = 207), 'cleanup exceeded its batch';
  assert public.cleanup_activity_events(null) = 1, 'null batch did not use bounded default';
  assert public.cleanup_activity_events() = 0, 'cleanup removed fresh, failed, or unfinished work';
  assert (select count(*) = 8 from public.activity_events), 'wrong queue rows survived cleanup';

  select * into strict dashboard from public.automation_dashboard(2);
  assert dashboard.activities_checked = 2 and dashboard.activities_updated = 1,
    'queue cleanup changed durable dashboard history';

  -- Old workers update only status and next_attempt_at; the trigger records the
  -- actual transition and preserves its timestamp across unrelated updates.
  update public.activity_events set status = 'done', completed_at = now() - interval '90 days'
    where activity_id = 202;
  assert (select completed_at = now() from public.activity_events where activity_id = 202),
    'completion transition did not record the current time';
  update public.activity_events set completed_at = now() - interval '90 days' where activity_id = 202;
  assert (select completed_at = now() from public.activity_events where activity_id = 202),
    'completed row lost its original completion time';
  update public.activity_events set status = 'pending' where activity_id = 202;
  assert (select completed_at is null from public.activity_events where activity_id = 202),
    'requeued event retained a completion timestamp';

  insert into public.activity_events
    (owner_id, activity_id, subscription_id, event_time, status, completed_at)
  select 2, activity_id, 9, 1, 'done', now() - interval '60 days'
    from generate_series(10000, 11000) as activity_id;
  assert public.cleanup_activity_events(5000) = 1000, 'cleanup exceeded its hard cap';
  assert public.cleanup_activity_events() = 1, 'bounded cleanup left the wrong remainder';
  assert public.cleanup_activity_events() = 0, 'repeated cleanup was not safe';

  assert not has_function_privilege('anon', 'public.activity_queue_health()', 'EXECUTE'), 'anonymous queue health access';
  assert not has_function_privilege('authenticated', 'public.activity_queue_health()', 'EXECUTE'), 'client queue health access';
  assert not has_function_privilege('anon', 'public.cleanup_activity_events(integer)', 'EXECUTE'), 'anonymous queue cleanup';
  assert not has_function_privilege('authenticated', 'public.cleanup_activity_events(integer)', 'EXECUTE'), 'client queue cleanup';
  assert has_function_privilege('service_role', 'public.activity_queue_health()', 'EXECUTE'), 'server cannot read queue health';
  assert has_function_privilege('service_role', 'public.cleanup_activity_events(integer)', 'EXECUTE'), 'server cannot clean queue';
end;
$$;

set local role service_role;
select * from public.activity_queue_health();
select public.cleanup_activity_events(0);
reset role;
commit;
