'use client'

import { useState } from 'react'
import { ShieldCheck } from 'lucide-react'
import {
  AuthApiError,
  ID_NUMBER_AR,
  isBusinessFileValid,
  isIdImageValid,
  isPasswordValid,
  NAME_AR,
  normalizePhone,
  PASSWORD_AR,
  PHONE_AR,
  registerConsumer,
} from '@/lib/auth/api'
import { JourneyHeader } from '@/components/shared/journey-header'
import { AuthError } from '@/components/screens/auth/auth-error'

/**
 * Citizen registration (Arabic RTL) — `POST /auth/register/consumer` as
 * multipart/form-data per the official AUTH_FRONTEND_API.md contract:
 *   phone, password, name (10–250), id_number (9–50), id_image (required file,
 *   JPEG/PNG/WEBP ≤ 5 MB) plus at least ONE profile type (is_driver / is_home /
 *   is_business — multiple allowed, exact documented identifiers) with their
 *   conditional fields.
 *
 * Registration success is NOT approval: the account is created with
 * `is_verified: false` and the flow continues to the OTP screen; the password
 * is handed to the shell so the real `POST /auth/login` runs after
 * verification — the session is never faked.
 */
export function RegisterScreen({
  onBack,
  onSwitchToLogin,
  onRegistered,
}: {
  onBack: () => void
  onSwitchToLogin: () => void
  onRegistered: (context: { phone: string; password: string }) => void
}) {
  const [fullName, setFullName] = useState('')
  const [phone, setPhone] = useState('')
  const [password, setPassword] = useState('')
  const [idNumber, setIdNumber] = useState('')
  const [idImage, setIdImage] = useState<File | null>(null)
  // Profile types — at least one required, multiple allowed (documented rule).
  const [isDriver, setIsDriver] = useState(false)
  const [vehicleNumber, setVehicleNumber] = useState('')
  const [vehicleType, setVehicleType] = useState('')
  const [isHome, setIsHome] = useState(false)
  const [familyCount, setFamilyCount] = useState('')
  const [isBusiness, setIsBusiness] = useState(false)
  const [businessName, setBusinessName] = useState('')
  const [businessFile, setBusinessFile] = useState<File | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<AuthApiError | null>(null)

  const reject = (messageAr: string) =>
    setError(
      new AuthApiError({ status: 0, code: 'VALIDATION_ERROR', message: 'Invalid input.', messageAr }),
    )

  const submit = async (event: React.FormEvent) => {
    event.preventDefault()
    if (busy) return
    setError(null)

    const name = fullName.trim()
    const normalizedPhone = normalizePhone(phone)
    const family = Number(familyCount)

    if (name.length < 10 || name.length > 250) return reject(NAME_AR)
    if (!normalizedPhone) return reject(PHONE_AR)
    if (!isPasswordValid(password)) return reject(PASSWORD_AR)
    if (idNumber.trim().length < 9 || idNumber.trim().length > 50) return reject(ID_NUMBER_AR)
    const imageError = isIdImageValid(idImage)
    if (imageError) return reject(imageError)
    // Documented rule: at least one profile type must be active.
    if (!isDriver && !isHome && !isBusiness) {
      return reject('اختر نوع الحساب على الأقل (منزلي، سائق، أو تجاري) — يمكن اختيار أكثر من نوع.')
    }
    if (isDriver) {
      if ((vehicleNumber.trim().length < 2 || vehicleNumber.trim().length > 50) ||
          (vehicleType.trim().length < 2 || vehicleType.trim().length > 50)) {
        return reject('بيانات المركبة مطلوبة عند اختيار حساب السائق: رقم المركبة ونوعها من حرفين إلى ٥٠.')
      }
    }
    if (isHome) {
      if (!Number.isInteger(family) || family < 1 || family > 50) {
        return reject('عدد أفراد الأسرة رقم صحيح من ١ إلى ٥٠.')
      }
    }
    if (isBusiness) {
      if (businessName.trim().length < 2 || businessName.trim().length > 200) {
        return reject('اسم النشاط التجاري من حرفين إلى ٢٠٠ حرف.')
      }
      const businessError = isBusinessFileValid(businessFile)
      if (businessError) return reject(businessError)
    }

    setBusy(true)
    try {
      await registerConsumer({
        phone: normalizedPhone,
        password,
        name,
        id_number: idNumber.trim(),
        id_image: idImage as File,
        is_driver: isDriver,
        vehicle_number: isDriver ? vehicleNumber.trim() : undefined,
        vehicle_type: isDriver ? vehicleType.trim() : undefined,
        is_home: isHome,
        family_count: isHome ? family : undefined,
        is_business: isBusiness,
        business_name: isBusiness ? businessName.trim() : undefined,
        business_file: isBusiness && businessFile ? businessFile : undefined,
      })
      // Account created (status: pending, is_verified: false) — SMS OTP was
      // triggered server-side. Continue to the OTP screen; never fake approval.
      onRegistered({ phone: normalizedPhone, password })
    } catch (caught) {
      setError(caught instanceof AuthApiError ? caught : null)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="journey-screen">
      <JourneyHeader title="إنشاء حساب جديد" onBack={onBack} />
      <main className="phone-content auth-content">
        <div className="auth-intro">
          <h1>انضم إلى موارد</h1>
          <p>أدخل بياناتك وهويةك، وسنرسل رمز تحقق إلى رقم هاتفك.</p>
        </div>
        <form className="auth-form" onSubmit={submit} noValidate>
          <label className="form-field">
            <span>الاسم الكامل</span>
            <input
              autoComplete="name"
              value={fullName}
              onChange={(event) => setFullName(event.target.value)}
              placeholder="أدخل اسمك الثلاثي"
            />
            <span className="auth-field-hint">من ١٠ إلى ٢٥٠ حرفاً</span>
          </label>
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
            <span className="auth-field-hint">أرقام يمنية تبدأ بـ ٧ — سنرسل رمز تحقق إليه</span>
          </label>
          <label className="form-field">
            <span>كلمة المرور</span>
            <input
              type="password"
              autoComplete="new-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              placeholder="اختر كلمة مرور قوية"
            />
            <span className="auth-field-hint">٨ أحرف على الأقل</span>
          </label>
          <label className="form-field">
            <span>رقم الهوية الوطنية</span>
            <input
              dir="ltr"
              inputMode="text"
              autoComplete="off"
              value={idNumber}
              onChange={(event) => setIdNumber(event.target.value)}
              placeholder="أدخل رقم هويتك"
            />
            <span className="auth-field-hint">من ٩ إلى ٥٠ خانة — مطلوب لإنشاء الحساب</span>
          </label>
          <label className="form-field">
            <span>صورة الهوية</span>
            <input
              className="file-input"
              type="file"
              accept="image/jpeg,image/png,image/webp"
              onChange={(event) => setIdImage(event.target.files?.[0] ?? null)}
            />
            <span className="auth-field-hint">صورة واضحة للهوية — JPEG أو PNG أو WEBP، بحد أقصى ٥ ميجابايت</span>
          </label>

          <fieldset className="type-fieldset">
            <legend>نوع الحساب</legend>
            <span className="auth-field-hint">اختر نوعاً واحداً على الأقل — ويمكنك اختيار أكثر من نوع</span>
            <label className="type-option">
              <input
                type="checkbox"
                checked={isHome}
                onChange={(event) => setIsHome(event.target.checked)}
              />
              <span>حساب منزلي (أسرة)</span>
            </label>
            {isHome && (
              <label className="form-field type-conditional">
                <span>عدد أفراد الأسرة</span>
                <input
                  dir="ltr"
                  inputMode="numeric"
                  value={familyCount}
                  onChange={(event) => setFamilyCount(event.target.value.replace(/\D/g, '').slice(0, 2))}
                  placeholder="مثال: 5"
                />
              </label>
            )}
            <label className="type-option">
              <input
                type="checkbox"
                checked={isDriver}
                onChange={(event) => setIsDriver(event.target.checked)}
              />
              <span>حساب سائق</span>
            </label>
            {isDriver && (
              <>
                <label className="form-field type-conditional">
                  <span>رقم المركبة</span>
                  <input
                    dir="ltr"
                    value={vehicleNumber}
                    onChange={(event) => setVehicleNumber(event.target.value.slice(0, 50))}
                    placeholder="أدخل رقم المركبة"
                  />
                </label>
                <label className="form-field type-conditional">
                  <span>نوع المركبة</span>
                  <input
                    value={vehicleType}
                    onChange={(event) => setVehicleType(event.target.value.slice(0, 50))}
                    placeholder="مثال: شاحنة صهريج"
                  />
                </label>
              </>
            )}
            <label className="type-option">
              <input
                type="checkbox"
                checked={isBusiness}
                onChange={(event) => setIsBusiness(event.target.checked)}
              />
              <span>حساب تجاري</span>
            </label>
            {isBusiness && (
              <>
                <label className="form-field type-conditional">
                  <span>اسم النشاط التجاري</span>
                  <input
                    value={businessName}
                    onChange={(event) => setBusinessName(event.target.value.slice(0, 200))}
                    placeholder="أدخل اسم النشاط التجاري"
                  />
                </label>
                <label className="form-field type-conditional">
                  <span>السجل التجاري أو رخصة العمل (اختياري)</span>
                  <input
                    className="file-input"
                    type="file"
                    accept="image/jpeg,image/png,image/webp,application/pdf"
                    onChange={(event) => setBusinessFile(event.target.files?.[0] ?? null)}
                  />
                  <span className="auth-field-hint">JPEG أو PNG أو WEBP أو PDF، بحد أقصى ٥ ميجابايت</span>
                </label>
              </>
            )}
          </fieldset>

          <AuthError error={error} />
          <button className="confirm-button" type="submit" disabled={busy}>
            {busy ? 'جارٍ إنشاء الحساب…' : 'إنشاء الحساب'}
          </button>
        </form>
        <p className="auth-switch">
          لديك حساب بالفعل؟
          <button type="button" onClick={onSwitchToLogin}>تسجيل الدخول</button>
        </p>
        <p className="secure-note">
          <ShieldCheck size={14} /> بياناتك تُرسل مباشرة إلى خادم موارد — لا نعرض كلمة المرور أو الرموز في السجلات
        </p>
      </main>
    </div>
  )
}
