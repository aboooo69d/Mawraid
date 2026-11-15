'use client'

import { useState } from 'react'
import { ShieldCheck } from 'lucide-react'
import {
  AuthApiError,
  isPasswordValid,
  isRestKeyValid,
  PASSWORD_AR,
  resetPassword,
} from '@/lib/auth/api'
import { JourneyHeader } from '@/components/shared/journey-header'
import { AuthError } from '@/components/screens/auth/auth-error'

/**
 * Reset password — step 2 (Arabic RTL). Official Workflow C:
 *   POST /auth/reset-password {rest_key, password}
 *
 * The reset key arrives via SMS, is valid for 15 minutes, and consists of 10
 * alphanumeric characters (confirmed by the backend developer).
 *
 * TODO(BACKEND-FIELD-NAME): the request field is spelled `rest_key` in the
 * current backend DTO (a typo on the backend side). It is preserved EXACTLY
 * here for compatibility — do not "fix" it in the frontend. When the backend
 * renames it to `reset_key`, update lib/auth/api.ts and this screen in the
 * same commit.
 */
export function ResetPasswordScreen({
  phone,
  onBack,
  onReset,
}: {
  /** Shown in the intro only (context); NOT sent — the API takes no phone here. */
  phone?: string
  onBack: () => void
  onReset: () => void
}) {
  const [restKey, setRestKey] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<AuthApiError | null>(null)

  const submit = async (event: React.FormEvent) => {
    event.preventDefault()
    if (busy) return
    setError(null)

    const reject = (messageAr: string) =>
      setError(
        new AuthApiError({ status: 0, code: 'VALIDATION_ERROR', message: 'Invalid input.', messageAr }),
      )

    if (!isRestKeyValid(restKey.trim())) {
      return reject('رمز الاستعادة يتكوّن من ١٠ أحرف وأرقام كما ورد في رسالة النصية.')
    }
    if (!isPasswordValid(password)) return reject(PASSWORD_AR)
    if (password !== confirmPassword) return reject('كلمتا المرور غير متطابقتين.')

    setBusy(true)
    try {
      await resetPassword(restKey.trim(), password)
      onReset()
    } catch (caught) {
      setError(caught instanceof AuthApiError ? caught : null)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="journey-screen">
      <JourneyHeader title="تعيين كلمة مرور جديدة" onBack={onBack} />
      <main className="phone-content auth-content">
        <div className="auth-intro">
          <h1>كلمة المرور الجديدة</h1>
          <p>
            أدخل رمز الاستعادة المرسل
            {phone ? <> إلى <strong dir="ltr">{phone}</strong></> : null} وكلمة المرور الجديدة.
            الرمز صالح لمدة ١٥ دقيقة.
          </p>
        </div>
        <form className="auth-form" onSubmit={submit} noValidate>
          <label className="form-field">
            <span>رمز الاستعادة</span>
            <input
              dir="ltr"
              autoComplete="one-time-code"
              value={restKey}
              onChange={(event) => setRestKey(event.target.value.trim().slice(0, 10))}
              placeholder="XXXXXXXXXX"
            />
            <span className="auth-field-hint">١٠ خانات أحرف وأرقام كما وردت في رسالة النصية</span>
          </label>
          <label className="form-field">
            <span>كلمة المرور الجديدة</span>
            <input
              type="password"
              autoComplete="new-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              placeholder="اختر كلمة مرور جديدة"
            />
            <span className="auth-field-hint">٨ أحرف على الأقل</span>
          </label>
          <label className="form-field">
            <span>تأكيد كلمة المرور</span>
            <input
              type="password"
              autoComplete="new-password"
              value={confirmPassword}
              onChange={(event) => setConfirmPassword(event.target.value)}
              placeholder="أعد إدخال كلمة المرور"
            />
          </label>
          <AuthError error={error} />
          <button className="confirm-button" type="submit" disabled={busy}>
            {busy ? 'جارٍ تعيين كلمة المرور…' : 'تعيين كلمة المرور'}
          </button>
        </form>
        <p className="auth-switch">
          <button type="button" onClick={onBack}>العودة إلى تسجيل الدخول</button>
        </p>
        <p className="secure-note">
          <ShieldCheck size={14} /> لا تشارك رمز الاستعادة أو كلمة المرور مع أي شخص
        </p>
      </main>
    </div>
  )
}
