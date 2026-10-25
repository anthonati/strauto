import type { AutomationSummary } from '../lib/api'

interface AutomationCardProps {
  automation: AutomationSummary
  configured: boolean
  busy: boolean
  onToggle: () => void
}

const outcomes = {
  applied: 'Activity updated',
  skipped: 'No change needed',
  failed: 'Attempt failed',
}

export function AutomationCard({ automation, configured, busy, onToggle }: AutomationCardProps) {
  const status = !configured ? 'Setup pending' : automation.enabled ? 'Enabled' : 'Disabled'
  return (
    <section aria-labelledby={`automation-${automation.id}`} className='rounded-xl bg-white p-6 shadow-sm border border-orange-100'>
      <div className='flex items-start justify-between gap-6'>
        <div>
          <p className='mb-2 text-sm font-medium text-slate-500'>{status}</p>
          <h2 id={`automation-${automation.id}`} className='text-xl font-semibold text-slate-900'>Mute weight training</h2>
          <p className='mt-2 text-slate-600'>Mute new Weight Training activities in Strava home and club feeds after upload.</p>
        </div>
        <button
          type='button'
          role='switch'
          aria-checked={automation.enabled}
          aria-label='Mute weight training activities'
          onClick={onToggle}
          disabled={busy || (!configured && !automation.enabled)}
          className={`shrink-0 rounded-full w-12 h-7 p-1 transition-colors disabled:opacity-50 focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-orange-600 ${automation.enabled ? 'bg-orange-500' : 'bg-slate-300'}`}
        >
          <span className={`block rounded-full bg-white w-5 h-5 shadow transition-transform ${automation.enabled ? 'translate-x-5' : ''}`} />
        </button>
      </div>
      {!configured && <p className='mt-4 text-sm text-amber-800'>Automatic processing is still being set up. Your saved preference will apply when setup is complete.</p>}
      <dl className='mt-6 grid grid-cols-2 gap-4 border-t border-slate-100 pt-5'>
        <div>
          <dt className='text-sm text-slate-500'>Activities checked</dt>
          <dd className='mt-1 text-2xl font-semibold tabular-nums text-slate-900'>{automation.activities_checked.toLocaleString()}</dd>
        </div>
        <div>
          <dt className='text-sm text-slate-500'>Activities updated</dt>
          <dd className='mt-1 text-2xl font-semibold tabular-nums text-slate-900'>{automation.activities_updated.toLocaleString()}</dd>
        </div>
      </dl>
      <p className='mt-4 text-sm text-slate-600'>
        {automation.last_run_at ? <>
          Last run: <time dateTime={automation.last_run_at}>{new Date(automation.last_run_at).toLocaleString()}</time>
          {automation.last_outcome && ` · ${outcomes[automation.last_outcome]}`}
        </> : 'No activities processed yet. Counts start with new uploads while this rule is enabled.'}
      </p>
      {automation.failed_activities > 0 && <p className='mt-3 text-sm text-red-700'>
        {automation.failed_activities.toLocaleString()} {automation.failed_activities === 1 ? 'activity has' : 'activities have'} a failed attempt. Temporary failures retry automatically; persistent failures may need attention.
      </p>}
      <p className='mt-5 text-sm text-slate-500'>Each activity counts once. Turning this rule off keeps its history.</p>
    </section>
  )
}
