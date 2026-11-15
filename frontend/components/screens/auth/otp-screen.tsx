'use client'

import { useEffect, useState } from 'react'
import { ShieldCheck } from 'lucide-react'
import {
  AuthApiError,
  isOtpCodeValid,
  login as apiLogin,
  resendOtp,
  verifyPhone,
} from '@/lib/auth/api'
import { setSessionFromUser } from '@/lib/auth/session'
import { JourneyHeader } from '@/components/shared/journey-header'
import { AuthError } from '@/components/screens/auth/auth-error'

/** Documented server-side resend cooldown: 60 seconds per phone. */
const RESEND_COOLDOWN_SECONDS = 60

/**
 * OTP verification (Arabic RTL) — official contract:
 *   POST /auth/verify-phone {phone, code}  → then a REAL POST /auth/login
 *   POST /auth/resend-otp   {phone}        → new 6-digit code (SMS by backend)
 *
 * OTP generation/delivery belongs entirely to the backend — this screen only
 * initiates documented requests and renders the responses. There is no
 * simulated verification and no dev bypass (none exists in the contract).
 * The 60s resend cooldown mirrors the documented rate limit.
 *
 * TODO(#6 PENDING BACKEND CLARIFICATION): post-verification routing for
 * accounts that still require administrative approval (pending-review
 * behavior) awaits confirmation from the backend developer. Only the
 * documented behavior is implemented here: verify → real login; a `1500`
 * login response renders the pending-approval message (registration success
 * is never treated as approval). Do not invent additional states until the
 * clarification arrives.
 */
export function OtpScreen({
  phone,
  password,
  onBack,
  onVerified,
}: {
  phone: string
  password: string
  onBack: () => void
  onVerified: () => void
}) {
  const [code, setCode] = useState('')
  const [busy, setBusy] = useState(false)
  const [resendBusy, setResendBusy] = useState(false)
  const [notice, setNotice] = useState<string | null>(null)
  const [error, setError] = useState<AuthApiError | null>(null)
  // The registration request already triggered an SMS → start the documented
  // 60s cooldown immediately, and restart it after every successful resend.
  const [cooldown, setCooldown] = useState(RESEND_COOLDOWN_SECONDS)

  useEffect(() => {
    if (cooldown <= 0) return
    const timer = setInterval(() => setCooldown((seconds) => Math.max(0, seconds - 1)), 1000)
    return () => clearInterval(timer)
  }, [cooldown])

  const verify = async (event: React.FormEvent) => {
    event.preventDefault()
    if (busy) return
    setError(null)
    setNotice(null)

    if (!isOtpCodeValid(code)) {
      setError(
        new AuthApiError({
          status: 0,
          code: 'VALIDATION_ERROR',
          message: 'Code must be 6 digits.',
          messageAr: 'أدخل رمز التحقق المكوّن من ٦ أرقام.',
        }),
      )
      return
    }

    setBusy(true)
    try {
      await verifyPhone({ phone, code })
      // Verified — now really sign in with the carried password. Never fabricate a session.
      const user = await apiLogin(phone, password)
      setSessionFromUser(user)
      onVerified()
    } catch (caught) {
      setError(caught instanceof AuthApiError ? caught : null)
    } finally {
      setBusy(false)
    }
  }

  const resend = async () => {
    if (resendBusy || cooldown > 0) return
    setError(null)
    setNotice(null)
    setResendBusy(true)
    try {
      await resendOtp(phone)
      setCooldown(RESEND_COOLDOWN_SECONDS)
      setNotice('تم إرسال رمز تحقق جديد إلى رقم هاتفك.')
    } catch (caught) {
      setError(caught instanceof AuthApiError ? caught : null)
    } finally {
      setResendBusy(false)
    }
  }

  return (
    <div className="journey-screen">
      <JourneyHeader title="تأكيد رقم الهاتف" onBack={onBack} />
      <main className="phone-content auth-content">
        <div className="auth-intro">
          <h1>رمز التحقق</h1>
          <p>
            أدخل الرمز المكوّن من ٦ أرقام المرسل إلى <strong dir="ltr">{phone}</strong>
          </p>
        </div>
        <form className="auth-form" onSubmit={verify} noValidate>
          <label className="form-field">
            <span>رمز التحقق</span>
            <input
              className="otp-field"
              dir="ltr"
              inputMode="numeric"
              autoComplete="one-time-code"
              maxLength={6}
              value={code}
              onChange={(event) => setCode(event.target.value.replace(/\D/g, '').slice(0, 6))}
              placeholder="000000"
            />
          </label>
          <AuthError error={error} />
          {notice && !error && <p className="auth-notice">{notice}</p>}
          <button className="confirm-button" type="submit" disabled={busy || !isOtpCodeValid(code)}>
            {busy ? 'جارٍ التحقق…' : 'تأكيد الرمز وتسجيل الدخول'}
          </button>
        </form>
        <div className="otp-meta">
          <span>لم يصلك الرمز؟</span>
          <button type="button" onClick={resend} disabled={resendBusy || cooldown > 0}>
            {resendBusy
              ? 'جارٍ الإرسال…'
              : cooldown > 0
                ? `إعادة إرسال الرمز (${cooldown} ث)`
                : 'إعادة إرسال الرمز'}
          </button>
        </div>
        <p className="secure-note">
          <ShieldCheck size={14} /> رمز التحقق يُستخدم مرة واحدة — لا تشاركه مع أي شخص
        </p>
      </main>
    </div>
  )
}
