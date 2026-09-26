export interface ConnectedAthlete {
  first_name: string
  last_name: string
  mute_weight_training: boolean
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`/api/${path}`, {
    credentials: 'same-origin',
    cache: 'no-store',
    ...options,
  })
  if (response.status === 401) throw new Error('not connected')
  if (!response.ok) {
    const detail = response.status === 503 ? await response.text() : ''
    if (detail.trim() === 'automation is not configured') {
      throw new Error('Mute weight training is not ready yet. Its Strava webhook and activity worker still need setup.')
    }
    throw new Error(response.status === 503 ? 'Strauto is not configured yet.' : 'The request failed. Please try again.')
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

export function getMe() {
  return request<ConnectedAthlete>('me')
}

export function setMuteWeightTraining(enabled: boolean) {
  return request<{ mute_weight_training: boolean }>('automation', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ mute_weight_training: enabled }),
  })
}

export function disconnect() {
  return request<void>('disconnect', { method: 'POST' })
}
