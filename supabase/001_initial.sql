-- Run in the Supabase SQL editor. Only server code uses the service role key.
create table if not exists public.athletes (
  id bigint primary key,
  first_name text not null default '',
  last_name text not null default '',
  access_token text not null,
  refresh_token text not null,
  expires_at bigint not null,
  granted_scopes text not null,
  mute_weight_training boolean not null default false,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists public.activity_events (
  id bigint generated always as identity primary key,
  owner_id bigint not null references public.athletes(id) on delete cascade,
  activity_id bigint not null,
  subscription_id bigint not null,
  event_time bigint not null,
  status text not null default 'pending' check (status in ('pending', 'processing', 'done', 'failed')),
  attempts integer not null default 0,
  next_attempt_at timestamptz not null default now(),
  created_at timestamptz not null default now(),
  unique (subscription_id, owner_id, activity_id, event_time)
);

create index if not exists activity_events_ready_idx
  on public.activity_events (next_attempt_at, id) where status in ('pending', 'processing');

alter table public.athletes enable row level security;
alter table public.activity_events enable row level security;
revoke all on public.athletes, public.activity_events from anon, authenticated;
grant select, insert, update, delete on public.athletes, public.activity_events to service_role;
grant usage, select on sequence public.activity_events_id_seq to service_role;

-- Claim work atomically so concurrent workers do not edit the same upload.
create or replace function public.claim_activity_events(batch_size integer)
returns table (id bigint, owner_id bigint, activity_id bigint, attempts integer)
language sql security definer set search_path = public
as $$
  with ready as (
    select e.id from public.activity_events e
    where (e.status = 'pending' and e.next_attempt_at <= now())
       or (e.status = 'processing' and e.next_attempt_at <= now())
    order by e.id
    for update skip locked
    limit least(greatest(batch_size, 1), 10)
  ), claimed as (
    update public.activity_events e
    set status = 'processing', attempts = e.attempts + 1,
        next_attempt_at = now() + interval '5 minutes'
    from ready where e.id = ready.id
    returning e.id, e.owner_id, e.activity_id, e.attempts
  )
  select * from claimed;
$$;

revoke all on function public.claim_activity_events(integer) from public, anon, authenticated;
grant execute on function public.claim_activity_events(integer) to service_role;

-- Only connected athletes with this subscription enabled enter the queue.
create or replace function public.enqueue_activity_event(
  p_owner_id bigint, p_activity_id bigint, p_subscription_id bigint, p_event_time bigint
)
returns void language sql security definer set search_path = public
as $$
  insert into public.activity_events (owner_id, activity_id, subscription_id, event_time)
  select p_owner_id, p_activity_id, p_subscription_id, p_event_time
  from public.athletes where id = p_owner_id and mute_weight_training = true
  on conflict (subscription_id, owner_id, activity_id, event_time) do nothing;
$$;

revoke all on function public.enqueue_activity_event(bigint, bigint, bigint, bigint) from public, anon, authenticated;
grant execute on function public.enqueue_activity_event(bigint, bigint, bigint, bigint) to service_role;
