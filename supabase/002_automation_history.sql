-- Apply after 001_initial.sql, before deploying the dashboard release.
begin;

create table if not exists public.automation_subscriptions (
  id bigint generated always as identity primary key,
  owner_id bigint not null references public.athletes(id) on delete cascade,
  rule_type text not null check (rule_type in ('mute_weight_training')),
  enabled boolean not null default false,
  created_at timestamptz not null default now(),
  unique (owner_id, rule_type)
);

create table if not exists public.automation_runs (
  subscription_id bigint not null references public.automation_subscriptions(id) on delete cascade,
  activity_id bigint not null,
  checked boolean not null default false,
  updated boolean not null default false,
  outcome text not null check (outcome in ('applied', 'skipped', 'failed')),
  last_run_at timestamptz not null default now(),
  primary key (subscription_id, activity_id),
  check (not updated or checked)
);

alter table public.automation_subscriptions enable row level security;
alter table public.automation_runs enable row level security;
revoke all on public.automation_subscriptions, public.automation_runs from anon, authenticated;
grant select, insert, update, delete on public.automation_subscriptions, public.automation_runs to service_role;
grant usage, select on sequence public.automation_subscriptions_id_seq to service_role;

-- Keep the old flag as a write bridge so the previous deployment can continue
-- serving requests while this migration is applied. Subscriptions drive reads.
create or replace function public.sync_mute_subscription()
returns trigger language plpgsql security definer set search_path = public
as $$
begin
  insert into public.automation_subscriptions (owner_id, rule_type, enabled)
  values (new.id, 'mute_weight_training', new.mute_weight_training)
  on conflict (owner_id, rule_type) do update set enabled = excluded.enabled;
  return new;
end;
$$;
revoke all on function public.sync_mute_subscription() from public, anon, authenticated;
drop trigger if exists sync_mute_subscription on public.athletes;
create trigger sync_mute_subscription
after insert or update of mute_weight_training on public.athletes
for each row execute function public.sync_mute_subscription();

insert into public.automation_subscriptions (owner_id, rule_type, enabled)
select id, 'mute_weight_training', mute_weight_training from public.athletes
on conflict (owner_id, rule_type) do nothing;

create or replace function public.enqueue_activity_event(
  p_owner_id bigint, p_activity_id bigint, p_subscription_id bigint, p_event_time bigint
)
returns void language sql security definer set search_path = public
as $$
  insert into public.activity_events (owner_id, activity_id, subscription_id, event_time)
  select p_owner_id, p_activity_id, p_subscription_id, p_event_time
  where exists (
    select 1 from public.automation_subscriptions
    where owner_id = p_owner_id and rule_type = 'mute_weight_training' and enabled
  )
  on conflict (subscription_id, owner_id, activity_id, event_time) do nothing;
$$;

-- A unique activity contributes at most once to each count. A later duplicate
-- or failed retry cannot erase a confirmed successful update.
create or replace function public.record_automation_run(
  p_owner_id bigint, p_subscription_id bigint, p_activity_id bigint,
  p_checked boolean, p_outcome text
)
returns void language sql security definer set search_path = public
as $$
  insert into public.automation_runs as runs
    (subscription_id, activity_id, checked, updated, outcome)
  select id, p_activity_id, p_checked, p_outcome = 'applied', p_outcome
  from public.automation_subscriptions
  where id = p_subscription_id and owner_id = p_owner_id
  on conflict (subscription_id, activity_id) do update set
    checked = runs.checked or excluded.checked,
    updated = runs.updated or excluded.updated,
    outcome = case when runs.updated then 'applied' else excluded.outcome end,
    last_run_at = now();
$$;

create or replace function public.automation_dashboard(p_owner_id bigint)
returns table (
  id bigint, rule_type text, enabled boolean,
  activities_checked bigint, activities_updated bigint, failed_activities bigint,
  last_run_at timestamptz, last_outcome text
)
language sql stable security definer set search_path = public
as $$
  select s.id, s.rule_type, s.enabled,
    count(*) filter (where r.checked), count(*) filter (where r.updated),
    count(*) filter (where r.outcome = 'failed'), max(r.last_run_at),
    (array_agg(r.outcome order by r.last_run_at desc, r.activity_id desc)
      filter (where r.activity_id is not null))[1]
  from public.automation_subscriptions s
  left join public.automation_runs r on r.subscription_id = s.id
  where s.owner_id = p_owner_id
  group by s.id;
$$;

revoke all on function public.record_automation_run(bigint, bigint, bigint, boolean, text) from public, anon, authenticated;
revoke all on function public.automation_dashboard(bigint) from public, anon, authenticated;
grant execute on function public.record_automation_run(bigint, bigint, bigint, boolean, text) to service_role;
grant execute on function public.automation_dashboard(bigint) to service_role;

commit;
