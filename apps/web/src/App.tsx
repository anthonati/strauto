import { useEffect, useState } from 'react'
import { Dashboard } from './pages/Dashboard'
import { LoginPage } from './pages'
import { getMe, type ConnectedAthlete } from './lib/api'

export function App() {
	const [athlete, setAthlete] = useState<ConnectedAthlete | null>(null)
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState('')

	useEffect(() => {
		getMe()
			.then(setAthlete)
			.catch((err: Error) => {
			if (err.message !== 'not connected') setError(err.message)
			})
			.finally(() => setLoading(false))
	}, [])

	if (loading) return <main className='min-h-screen grid place-items-center text-slate-600'>Loading Strauto…</main>
	if (athlete) return <Dashboard athlete={athlete} onChange={setAthlete} onDisconnect={() => setAthlete(null)} />

	return <LoginPage error={error} />
}
