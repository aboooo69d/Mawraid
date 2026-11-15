'use client'

import { AlertCircle } from 'lucide-react'
import type { AuthApiError } from '@/lib/auth/api'

/**
 * Shared Arabic error banner for the citizen auth screens.
 * The documented envelope carries a single `error.message` (numeric code) —
 * the Arabic text is keyed by that code inside AuthApiError. Nothing is
 * logged; the banner renders the normalized error the caller chose to show.
 */
export function AuthError({ error }: { error: AuthApiError | null }) {
  if (!error) return null
  return (
    <div className="auth-error" role="alert">
      <AlertCircle size={15} />
      <div>
        <p>{error.messageAr}</p>
      </div>
    </div>
  )
}
