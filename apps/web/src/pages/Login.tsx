// import {
//   Button,
//   Card,
//   CardDescription,
//   CardHeader,
//   CardTitle,
// } from '@/components/ui'
import { Activity } from 'lucide-react'
import { ConnectWithStrava } from '../components/branding'

interface LoginProps {
  error?: string
}

export function Login({ error }: LoginProps) {
  const reason = new URLSearchParams(window.location.search).get('error')
  return (
    <div className='min-h-screen bg-gradient-to-br from-orange-50 to-orange-100 flex items-center justify-center p-4'>
      <div className='bg-white rounded-lg shadow-xl p-8 max-w-md w-full'>
        <div className='text-center mb-8'>
          <div className='inline-flex items-center justify-center w-16 h-16 bg-orange-500 rounded-xl mb-4'>
            <Activity className='w-8 h-8 text-white' strokeWidth={2.5} />
          </div>
          <h1 className='text-slate-900 mb-2'>Strauto</h1>
          <p className='text-slate-600'>Automate your Strava activities</p>
        </div>

        <div className='space-y-4'>
          <a
            href='/api/auth_start'
            aria-label='Connect with Strava'
            className='block w-fit mx-auto rounded-md focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-orange-600'
          >
            <ConnectWithStrava />
          </a>

          {(error || reason) && <p role='alert' className='text-center text-sm text-red-700'>{error || (reason === 'missing_scopes' ? 'Please grant activity read and write access to use Strauto.' : 'Strava connection was cancelled.')}</p>}

          <div className='text-center text-sm text-slate-500'>
            <p>
              By connecting, you allow Strauto to read and edit your Strava activities. You can disconnect at any time.
            </p>
          </div>
        </div>

        <div className='mt-8 pt-6 border-t border-slate-200'>
          <h3 className='text-slate-900 mb-3 text-center'>
            First automation:
          </h3>
          <ul className='space-y-2 text-sm text-slate-600'>
            <li className='flex items-start gap-2'>
              <span className='text-orange-500 mt-0.5 select-none'>
                ✓
              </span>
              <span>Mute new Weight Training activities in home and club feeds. They remain visible on your profile according to your Strava privacy setting.</span>
            </li>
          </ul>
        </div>
      </div>
    </div>
  )
}
