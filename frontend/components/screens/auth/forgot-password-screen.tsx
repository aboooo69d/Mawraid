'use client'

import { useState } from 'react'
import { ShieldCheck } from 'lucide-react'
import { AuthApiError, forgetPassword, normalizePhone, PHONE_AR } from '@/lib/auth/api'
import { JourneyHeader } from '@/components/shared/journey-header'
import { AuthError } from '@/components/screens/auth/auth-error'

/**
 * Forgot password — step 1 (Arabic RTL). Official Workflow C:
 *   POST /auth/forget-password {phone} → 200 always (anti-enumeration); when
 *   the account exists and is active, the backend SMSes a reset key valid for
 *   15 minutes. The frontend must NOT reveal whether the phone was registered.
 */
export function ForgotPasswordScreen({
  onBack,
  onSent,
}: {
  onBack: () => void
  onSent: (phone: string) => void
}) {
  const [phone, setPhone] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<AuthApiError | null>(null)

  const submit = async (event: React.FormEvent) => {
    event.preventDefault()
    if (busy) return
    setError(null)

    const normalized = normalizePhone(phone)
    if (!normalized) {
      setError(
        new AuthApiError({ status: 0, code: 'VALIDATION_ERROR', message: 'Invalid phone.', messageAr: PHONE_AR }),
      )
      return
    }

    setBusy(true)
    try {
      await forgetPassword(normalized)
      // Documented: always 200, even for unknown phones (no enumeration).
      onSent(normalized)
    } catch (caught) {
      setError(caught instanceof AuthApiError ? caught : null)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="journey-screen">
      <JourneyHeader title="استعادة كلمة المرور" onBack={onBack} />
      <main className="phone-content auth-content">
        <div className="auth-intro">
          <h1>نسيت كلمة المرور؟</h1>
          <p>أدخل رقم هاتفك المسجّل وسنرسل رمز استعادة صالح لمدة ١٥ دقيقة.</p>
        </div>
        <form className="auth-form" onSubmit={submit} noValidate>
          <label className="form-field">
            <span>رقم الهاتف</span>
            <input
              dir="ltr"
              inputMode="tel"
              autoComplete="tel"
              value={phone}
              onChange={(event) => setPhone(event.target.value)}
              placeholder="7XXXXXXXX"
            />
          </label>
          <AuthError error={error} />
          <button className="confirm-button" type="submit" disabled={busy}>
            {busy ? 'جارٍ إرسال الرمز…' : 'إرسال رمز الاستعادة'}
          </button>
        </form>
        <p className="auth-switch">
          تذكرت كلمة المرور؟
          <button type="button" onClick={onBack}>تسجيل الدخول</button>
        </p>
        <p className="secure-note">
          <ShieldCheck size={14} /> لحماية حسابك، لا نوضح ما إذا كان الرقم مسجّلاً أم لا
        </p>
      </main>
    </div>
  )
}
