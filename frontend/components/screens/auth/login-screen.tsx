'use client'

import { useState } from 'react'
import { Eye, EyeOff, ShieldCheck } from 'lucide-react'
import { AuthApiError, login as apiLogin, normalizePhone, PHONE_AR } from '@/lib/auth/api'
import { setSessionFromUser } from '@/lib/auth/session'
import { JourneyHeader } from '@/components/shared/journey-header'
import { AuthError } from '@/components/screens/auth/auth-error'

/**
 * Citizen login (Arabic RTL) — real `POST /auth/login` against the Go backend.
 *
 * Contract: `{phone, password}` → `{success, data: user}` + HttpOnly `session`
 * cookie set by the backend (credentials are always included; the cookie is
 * never handled by Next.js). The user object is cached by the session store.
 *
 * A `1102` (account is not verified) response is not shown as an error here:
 * it hands the phone + password to the shell, which routes to the OTP screen
 * and re-attempts the same real login after verification.
 * `1500` (account pending admin review) renders the pending-approval message —
 * registration success is never treated as approval.
 */
export function LoginScreen({
  initialPhone,
  notice,
  onBack,
  onSwitchToRegister,
  onSuccess,
  onNeedOtp,
  onForgotPassword,
}: {
  initialPhone?: string
  /** Optional success notice (e.g. after a completed password reset). */
  notice?: string | null
  onBack: () => void
  onSwitchToRegister: () => void
  onSuccess: () => void
  onNeedOtp: (context: { phone: string; password: string }) => void
  onForgotPassword: () => void
}) {
  const [phone, setPhone] = useState(initialPhone ?? '')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
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
    if (!password) {
      setError(
        new AuthApiError({
          status: 0,
          code: 'VALIDATION_ERROR',
          message: 'Missing password.',
          messageAr: 'أدخل كلمة المرور.',
        }),
      )
      return
    }

    setBusy(true)
    try {
      const user = await apiLogin(normalized, password)
      if (user.role_name !== 'consumer') {
        // The citizen app only hosts consumer sessions; provider accounts
        // log in through the provider portal.
        setError(
          new AuthApiError({
            status: 0,
            code: 'VALIDATION_ERROR',
            message: 'Non-consumer account.',
            messageAr: 'هذا التطبيق مخصص لحسابات المواطنين. استخدم بوابة مقدمي الخدمة للدخول.',
          }),
        )
        return
      }
      setSessionFromUser(user)
      onSuccess()
    } catch (caught) {
      if (caught instanceof AuthApiError && caught.code === '1102') {
        // Documented: redirect the unverified account to the OTP screen.
        onNeedOtp({ phone: normalized, password })
        return
      }
      setError(caught instanceof AuthApiError ? caught : null)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="journey-screen">
      <JourneyHeader title="تسجيل الدخول" onBack={onBack} />
      <main className="phone-content auth-content">
        <div className="auth-intro">
          <h1>أهلاً بعودتك</h1>
          <p>سجّل الدخول لمتابعة طلباتك وإدارة خدمات منزلك في موارد.</p>
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
          <label className="form-field">
            <span>كلمة المرور</span>
            <span className="password-wrap">
              <input
                type={showPassword ? 'text' : 'password'}
                autoComplete="current-password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                placeholder="أدخل كلمة المرور"
              />
              <button
                type="button"
                className="password-toggle"
                onClick={() => setShowPassword((visible) => !visible)}
                aria-label={showPassword ? 'إخفاء كلمة المرور' : 'إظهار كلمة المرور'}
              >
                {showPassword ? <EyeOff size={15} /> : <Eye size={15} />}
              </button>
            </span>
          </label>
          <AuthError error={error} />
          {notice && !error && <p className="auth-notice">{notice}</p>}
          <button className="confirm-button" type="submit" disabled={busy}>
            {busy ? 'جارٍ تسجيل الدخول…' : 'تسجيل الدخول'}
          </button>
        </form>
        <p className="auth-switch">
          <button type="button" onClick={onForgotPassword}>نسيت كلمة المرور؟</button>
        </p>
        <p className="auth-switch">
          ليس لديك حساب؟
          <button type="button" onClick={onSwitchToRegister}>إنشاء حساب جديد</button>
        </p>
        <p className="secure-note">
          <ShieldCheck size={14} /> اتصال مباشر وآمن مع خادم موارد — لا تشارك كلمة المرور مع أحد
        </p>
      </main>
    </div>
  )
}
