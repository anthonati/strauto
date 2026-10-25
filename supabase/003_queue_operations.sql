-- Apply after 002_automation_history.sql. Compatible with existing workers.
begin;

alter table public.activity_events add column if not exists completed_at timestamptz;

-- Earlier workers did not record completion time. Start retention now rather
-- than guessing from creation time and deleting recently completed retries.
update public.activity_events set completed_at = now()
where status = 'done' and completed_at is null;

create or replace function public.set_activity_event_completed_at()
returns trigger language plpgsql set search_path = public
as $$
begin
  if new.status <> 'done' then
    new.completed_at := null;
  elsif tg_op = 'INSERT' then
    new.completed_at := coalesce(new.completed_at, now());
  elsif old.status <> 'done' then
    new.completed_at := now();
  else
    new.completed_at := coalesce(old.completed_at, now());
  end if;
  return new;
end;
$$;
revoke all on function public.set_activity_event_completed_at() from public, anon, authenticated;
drop trigger if exists set_activity_event_completed_at on public.activity_events;
create trigger set_activity_event_completed_at
before insert or update of status, completed_at on public.activity_events
for each row execute function public.set_activity_event_completed_at();

create index if not exists activity_events_completed_idx
  on public.activity_events (completed_at, id) where status = 'done';

create or replace function public.activity_queue_health()
returns table (
  pending bigint, processing bigint, done bigint, failed bigint,
  due bigint, oldest_due_at timestamptz
)
language sql stable security definer set search_path = public
as $$
  select
    count(*) filter (where status = 'pending'),
    count(*) filter (where status = 'processing'),
    count(*) filter (where status = 'done'),
    count(*) filter (where status = 'failed'),
    count(*) filter (where status in ('pending', 'processing') and next_attempt_at <= now()),
    min(next_attempt_at) filter (where status in ('pending', 'processing') and next_attempt_at <= now())
  from public.activity_events;
$$;

-- Keep a month of successful deliveries for diagnosis. Failed and unfinished
-- work is never pruned, and automation_runs is independent of this queue.
create or replace function public.cleanup_activity_events(batch_size integer default 1000)
returns bigint language sql security definer set search_path = public
as $$
  with expired as (
    select e.id from public.activity_events e
    where e.status = 'done' and e.completed_at < now() - interval '30 days'
    order by e.completed_at, e.id
    for update skip locked
    limit least(greatest(coalesce(batch_size, 1000), 0), 1000)
  ), deleted as (
    delete from public.activity_events e using expired
    where e.id = expired.id
    returning e.id
  )
  select count(*) from deleted;
$$;

revoke all on function public.activity_queue_health() from public, anon, authenticated;
revoke all on function public.cleanup_activity_events(integer) from public, anon, authenticated;
grant execute on function public.activity_queue_health() to service_role;
grant execute on function public.cleanup_activity_events(integer) to service_role;

commit;
