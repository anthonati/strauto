import { useEffect, useState } from 'react'
import { AutomationCard } from '../components/AutomationCard'
import { disconnect, getAutomations, setMuteWeightTraining, type AutomationDashboard, type AutomationSummary, type ConnectedAthlete } from '../lib/api'

interface DashboardProps {
  athlete: ConnectedAthlete
  onChange: (athlete: ConnectedAthlete) => void
  onDisconnect: () => void
}

export function Dashboard({ athlete, onChange, onDisconnect }: DashboardProps) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [dashboard, setDashboard] = useState<AutomationDashboard | null>(null)
  const [refreshing, setRefreshing] = useState(true)
  const [loadError, setLoadError] = useState('')

  useEffect(() => {
    let active = true
    getAutomations()
      .then((data) => { if (active) setDashboard(data) })
      .catch((err: Error) => { if (active) setLoadError(err.message) })
      .finally(() => { if (active) setRefreshing(false) })
    return () => { active = false }
  }, [])

  async function refresh() {
    setRefreshing(true)
    setLoadError('')
    try {
      setDashboard(await getAutomations())
    } catch (err) {
      setLoadError((err as Error).message)
    } finally {
      setRefreshing(false)
    }
  }

  async function toggle(automation: AutomationSummary) {
    setBusy(true)
    setError('')
    try {
      const result = await setMuteWeightTraining(!automation.enabled)
      onChange({ ...athlete, ...result })
      setDashboard((previous) => previous && ({
        ...previous,
        automations: previous.automations.map((item) => item.id === automation.id ? { ...item, enabled: result.mute_weight_training } : item),
      }))
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  async function leave() {
    setBusy(true)
    setError('')
    try {
      await disconnect()
      onDisconnect()
    } catch (err) {
      setError((err as Error).message)
      setBusy(false)
    }
  }

  return (
    <main className='min-h-screen bg-orange-50 px-4 py-12'>
      <div className='max-w-xl mx-auto'>
        <header className='mb-8 flex items-center justify-between gap-4'>
          <div>
            <h1 className='text-3xl font-bold text-slate-900'>Strauto</h1>
            <p className='text-slate-600'>Connected as {athlete.first_name} {athlete.last_name}</p>
          </div>
          <button onClick={leave} disabled={busy || refreshing} className='text-sm text-slate-600 underline disabled:opacity-50'>Disconnect</button>
        </header>

        <div className='mb-4 flex items-center justify-between gap-4'>
          <h2 className='text-lg font-semibold text-slate-900'>Your automations</h2>
          <button onClick={refresh} disabled={busy || refreshing} className='text-sm text-slate-600 underline disabled:opacity-50'>{refreshing ? 'Refreshing…' : 'Refresh'}</button>
        </div>
        <div aria-busy={refreshing} className='space-y-4'>
          {!dashboard && refreshing && <p role='status' className='text-slate-600'>Loading your automations…</p>}
          {loadError && <p role='alert' className='text-red-700'>Could not load current automation history. {loadError} Use Refresh to try again.</p>}
          {dashboard?.automations.map((automation) => <AutomationCard key={automation.id} automation={automation} configured={dashboard.automation_configured} busy={busy || refreshing || !!loadError} onToggle={() => toggle(automation)} />)}
          {dashboard && dashboard.automations.length === 0 && <p className='text-slate-600'>No automations are available for this account yet.</p>}
        </div>
        <p className='mt-5 text-sm text-slate-500'>Muting happens after upload, so an activity may appear briefly in a feed. Muted activities remain visible on your profile according to your Strava privacy setting. Strauto cannot set an activity to “Only You” through the Strava API.</p>
        {error && <p role='alert' className='mt-4 text-red-700'>{error}</p>}
      </div>
    </main>
  )
}
