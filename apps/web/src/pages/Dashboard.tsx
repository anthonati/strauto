import { useState } from 'react'
import { disconnect, setMuteWeightTraining, type ConnectedAthlete } from '../lib/api'

interface DashboardProps {
  athlete: ConnectedAthlete
  onChange: (athlete: ConnectedAthlete) => void
  onDisconnect: () => void
}

export function Dashboard({ athlete, onChange, onDisconnect }: DashboardProps) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function toggle() {
    setBusy(true)
    setError('')
    try {
      const result = await setMuteWeightTraining(!athlete.mute_weight_training)
      onChange({ ...athlete, ...result })
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
          <button onClick={leave} disabled={busy} className='text-sm text-slate-600 underline disabled:opacity-50'>Disconnect</button>
        </header>

        <section className='rounded-xl bg-white p-6 shadow-sm border border-orange-100'>
          <div className='flex items-start justify-between gap-6'>
            <div>
              <h2 className='text-xl font-semibold text-slate-900'>Mute weight training</h2>
              <p className='mt-2 text-slate-600'>New Weight Training activities will be muted in Strava home and club feeds after upload.</p>
            </div>
            <button
              type='button'
              role='switch'
              aria-checked={athlete.mute_weight_training}
              aria-label='Mute weight training activities'
              onClick={toggle}
              disabled={busy}
              className={`shrink-0 rounded-full w-12 h-7 p-1 transition-colors disabled:opacity-50 ${athlete.mute_weight_training ? 'bg-orange-500' : 'bg-slate-300'}`}
            >
              <span className={`block rounded-full bg-white w-5 h-5 shadow transition-transform ${athlete.mute_weight_training ? 'translate-x-5' : ''}`} />
            </button>
          </div>
          <p className='mt-5 text-sm text-slate-500'>Muting happens after upload, so an activity may appear briefly in a feed. Muted activities remain visible on your profile according to your Strava privacy setting. Strauto cannot set an activity to “Only You” through the Strava API.</p>
        </section>
        {error && <p role='alert' className='mt-4 text-red-700'>{error}</p>}
      </div>
    </main>
  )
}
