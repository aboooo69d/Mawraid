'use client'

/**
 * Client session store for the citizen auth flow — COOKIE-BASED CONTRACT.
 *
 * CONFIRMED ARCHITECTURE (integration decisions):
 *  - The Go backend owns the session: login sets an HttpOnly `session` cookie
 *    (Redis-backed); logout clears it. Next.js never generates or manages
 *    authentication cookies; the browser attaches them automatically because
 *    every auth request uses `credentials: 'include'`.
 *  - There is NO `/auth/me` endpoint (confirmed) and NO replacement may be
 *    invented. The user object returned by the documented register/login
 *    responses is cached HERE (user data only — no tokens, no passwords) so
 *    the UI survives reloads. The backend remains authoritative: any
 *    protected request that fails with the session-invalid envelope
 *    (code 1000) must clear this cache.
 *  - Nothing here logs. Storage holds only the public user object.
 *
 * @see lib/auth/api.ts contract types
 */
import { useSyncExternalStore } from 'react'
import { logout as apiLogout, type AuthUser } from './api'

/** Bump when the stored shape changes so stale payloads are discarded. */
const STORAGE_KEY = 'mawarid.session.v2'

/** What components may observe: the documented user object only. */
export interface SessionSnapshot {
  user: AuthUser
}

let current: SessionSnapshot | null = null
let hydrated = false
const listeners = new Set<() => void>()

function isAuthUser(value: unknown): value is AuthUser {
  if (typeof value !== 'object' || value === null) return false
  const record = value as Record<string, unknown>
  // Minimal shape check against the documented user object.
  return typeof record.id === 'string' && typeof record.phone === 'string' && typeof record.name === 'string'
}

function notify(): void {
  for (const listener of listeners) listener()
}

/** Load the cached user once per browser page load. Safe during SSR. */
function hydrate(): void {
  if (hydrated) return
  hydrated = true
  if (typeof window === 'undefined') return
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (!raw) return
    const parsed: unknown = JSON.parse(raw)
    if (isAuthUser(parsed)) {
      current = { user: parsed }
    } else {
      window.localStorage.removeItem(STORAGE_KEY)
    }
  } catch {
    // Corrupt/unavailable storage — behave as signed out. No logging.
    current = null
  }
}

function persist(): void {
  if (typeof window === 'undefined') return
  try {
    if (current) window.localStorage.setItem(STORAGE_KEY, JSON.stringify(current.user))
    else window.localStorage.removeItem(STORAGE_KEY)
  } catch {
    // Storage may be disabled (private mode) — session then stays in memory.
  }
}

function subscribe(listener: () => void): () => void {
  hydrate()
  listeners.add(listener)
  return () => listeners.delete(listener)
}

function getSnapshot(): SessionSnapshot | null {
  hydrate()
  return current
}

/** React server render must see the same (empty) snapshot as SSR. */
function getServerSnapshot(): SessionSnapshot | null {
  return null
}

/** Subscribe to `{ user }`. */
export function useSession(): SessionSnapshot | null {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot)
}

/**
 * Store the user object returned by a documented success response
 * (register-consumer or login). The session cookie itself lives in the
 * browser cookie jar, owned by the backend.
 */
export function setSessionFromUser(user: AuthUser): void {
  current = { user }
  persist()
  notify()
}

/** Drop the cached user immediately (logout / session-invalid responses). */
export function clearSession(): void {
  current = null
  persist()
  notify()
}

/**
 * Server-side logout (backend destroys the Redis session and expires the
 * cookie) then clears the local cache. The cache is cleared regardless of
 * network outcome — the UI never keeps a session the user asked to end.
 */
export async function logoutSession(): Promise<void> {
  try {
    await apiLogout()
  } catch {
    // Network/server failure — the local cache is still cleared below.
  }
  clearSession()
}
