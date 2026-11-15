/**
 * Citizen authentication API — typed `fetch` calls against the OFFICIAL
 * "Nova Auth Module - Frontend Integration Guide" (AUTH_FRONTEND_API.md).
 *
 * CONFIRMED ARCHITECTURE (see integration decisions):
 *  - The Go backend (PostgreSQL/GORM/Redis) owns authentication. This module
 *    only initiates documented requests, parses the documented envelope and
 *    surfaces errors — it never fabricates sessions, OTPs or cookies.
 *  - Sessions are server-side (Redis) via an HttpOnly cookie named `session`,
 *    managed exclusively by the Go backend. Every request here therefore uses
 *    `credentials: 'include'`; the browser stores the cookie, and Next.js
 *    neither generates nor manages it.
 *  - There is NO `/auth/me` (and NO `/auth/refresh`) endpoint — confirmed by
 *    the backend developer. The logged-in user object returned by
 *    register/login is cached client-side by lib/auth/session.ts. Do not add
 *    calls to undocumented endpoints.
 *
 * DOCUMENTED ENDPOINTS (base `${NEXT_PUBLIC_API_BASE_URL}`):
 *   POST /auth/register/consumer  multipart/form-data → {success, data: User}
 *   POST /auth/resend-otp         {phone}             → {success, data: null}
 *   POST /auth/verify-phone       {phone, code}       → {success, data: null}
 *   POST /auth/login              {phone, password}   → {success, data: User} + Set-Cookie
 *   POST /auth/logout             (cookie)            → {success, data: null}
 *   POST /auth/forget-password    {phone}             → {success, data: null}
 *   POST /auth/reset-password     {rest_key, password}→ {success, data: null}
 *
 * ENVELOPE: success → {success: true, data}
 *           failure → {success: false, error: {code: NUMBER, message}}
 *
 * SECURITY: this module never logs anything — no credentials, no bodies.
 * Errors carry the backend's machine-readable numeric code; the Arabic UI
 * text is keyed locally by that code (the API sends English messages only,
 * except code 1204).
 */

/** Base URL of the Go backend (browser-reachable, includes /api/v1). */
// TODO(BACKEND-URL): temporary configuration — the FINAL backend URL will be
// provided by the backend developer. Do not hardcode a production URL here.
const API_BASE_URL = (process.env.NEXT_PUBLIC_API_BASE_URL ?? '').trim().replace(/\/+$/, '')

/** Standard success envelope of the documented API. */
export interface ApiOk<T> {
  success: true
  data: T
}

/** Standard error envelope of the documented API (`code` is numeric in JSON). */
export interface ApiErrorBody {
  success: false
  error: {
    code: number
    message: string
  }
}

/**
 * User object returned in `data` by register-consumer and login.
 * Field set is exactly the documented register success response (AUTH_FRONTEND_API.md §1)
 * — no undocumented fields are assumed.
 */
export interface AuthUser {
  /** UUID string. */
  id: string
  /** 9-digit Yemeni phone (^7[01378]\d{7}$), no country code. */
  phone: string
  is_verified: boolean
  /** Documented values seen: 'pending' | 'active'. */
  status: string
  role_id: number
  /** 'consumer' | 'provider'. */
  role_name: string
  created_at: string
  updated_at: string
  name: string
  id_number: string
  /** Storage path of the encrypted ID photo, e.g. `consumers/id/<uuid>.jpg.enc`. */
  id_image: string | null
  is_driver: boolean
  vehicle_number: string | null
  vehicle_type: string | null
  is_home: boolean
  family_count: number | null
  is_business: boolean
  business_name: string | null
  business_file: string | null
}

/** POST /auth/register/consumer request (multipart/form-data). */
export interface RegisterConsumerInput {
  /** 9 digits, `^7[01378]\d{7}$`. */
  phone: string
  /** Min 8, max 255 chars. */
  password: string
  /** Full name, min 10, max 250 chars. */
  name: string
  /** National ID, min 9, max 50 chars. */
  id_number: string
  /** ID photo — JPEG/PNG/WEBP, max 5 MB, non-empty. Required. */
  id_image: File
  /** At least ONE profile type must be true (documented rule). */
  is_driver: boolean
  /** Required when is_driver=true (min 2, max 50). */
  vehicle_number?: string
  /** Required when is_driver=true (min 2, max 50). */
  vehicle_type?: string
  is_home: boolean
  /** Required when is_home=true (min 1, max 50). */
  family_count?: number
  is_business: boolean
  /** Required when is_business=true (min 2, max 200). */
  business_name?: string
  /** Optional license file — JPEG/PNG/WEBP/PDF, max 5 MB. */
  business_file?: File
}

/**
 * Arabic UI messages keyed by the backend's NUMERIC error codes
 * (AUTH_FRONTEND_API.md "Error Codes Dictionary"). Client-side transport
 * codes (NETWORK_ERROR/TIMEOUT/CONFIG_ERROR) are strings and handled separately.
 */
const CODE_AR: Record<string, string> = {
  '2000': 'تحقق من صحة البيانات المدخلة وحاول مرة أخرى.',
  '1000': 'المستخدم غير موجود.',
  '1001': 'رقم الهاتف مسجل مسبقاً. سجّل الدخول بدلاً من ذلك.',
  '1002': 'هذا الحساب غير موجود.',
  '1100': 'رقم الهاتف أو كلمة المرور غير صحيحة.',
  '1101': 'الحساب غير نشط. تواصل مع الدعم.',
  '1102': 'لم يتم التحقق من رقم هاتفك بعد.',
  '1103': 'تم التحقق من هذا الحساب مسبقاً. يمكنك تسجيل الدخول.',
  '1104': 'رمز إعادة تعيين كلمة المرور غير صالح أو منتهي الصلاحية.',
  '1201': 'يرجى الانتظار دقيقة واحدة قبل طلب رمز جديد.',
  '1202': 'تم بلوغ الحد الأقصى لطلب رمز التحقق. حاول بعد فترة.',
  '1203': 'تم بلوغ الحد الأقصى لمحاولات التحقق. اطلب رمزاً جديداً.',
  '1204': 'رمز التحقق غير صحيح أو منتهي الصلاحية.',
  '1400': 'حجم الملف يتجاوز الحد الأقصى (٥ ميجابايت).',
  '1401': 'نوع الملف غير مسموح به.',
  '1402': 'الملف المرفق فارغ.',
  '1403': 'المستندات مطلوبة أو قيد المراجعة، أو تم رفض الحساب.',
  '1500': 'حسابك قيد المراجعة من قبل الإدارة ولا يمكن تسجيل الدخول بعد.',
  '1502': 'تم رفض الحساب أو إيقافه من قبل الإدارة.',
  '5000': 'خطأ داخلي في الخادم. حاول مرة أخرى لاحقاً.',
  // Client-side transport/config codes (not backend numeric codes).
  NETWORK_ERROR: 'تعذر الاتصال بالخادم. تحقق من اتصالك وحاول مجدداً.',
  TIMEOUT: 'استغرق الطلب وقتاً طويلاً. حاول مرة أخرى.',
  CONFIG_ERROR: 'إعدادات الاتصال غير مكتملة على الواجهة الأمامية.',
}

/** Normalized client-side error — mirrors the backend envelope for the UI. */
export class AuthApiError extends Error {
  readonly status: number
  /** Backend numeric code as a string (e.g. '1100'), or a transport code. */
  readonly code: string
  readonly messageAr: string

  constructor(init: { status: number; code: string; message: string; messageAr?: string }) {
    super(init.message)
    this.name = 'AuthApiError'
    this.status = init.status
    this.code = init.code
    this.messageAr = init.messageAr?.trim() || CODE_AR[init.code] || 'حدث خطأ غير متوقع. حاول مرة أخرى.'
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function pickString(source: Record<string, unknown>, key: string): string | undefined {
  const value = source[key]
  return typeof value === 'string' && value ? value : undefined
}

type AuthJsonOptions = {
  method?: 'GET' | 'POST'
  /** Plain object → JSON body; FormData → sent as multipart (browser sets boundary). */
  body?: unknown
  signal?: AbortSignal
}

/**
 * Low-level request against the Go backend. Cookies are ALWAYS included —
 * the session cookie is issued and validated by the backend only.
 * Never logs.
 */
async function authJson<T>(path: string, options: AuthJsonOptions = {}): Promise<T> {
  const { method = 'POST', body, signal } = options

  if (!API_BASE_URL) {
    throw new AuthApiError({
      status: 0,
      code: 'CONFIG_ERROR',
      message: 'NEXT_PUBLIC_API_BASE_URL is not configured.',
    })
  }

  const headers: Record<string, string> = { Accept: 'application/json' }
  const init: RequestInit = { method, headers, credentials: 'include', cache: 'no-store', signal }
  if (body instanceof FormData) {
    // Content-Type is intentionally NOT set — the browser adds the multipart boundary.
    init.body = body
  } else if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
    init.body = JSON.stringify(body)
  }

  let response: Response
  try {
    response = await fetch(`${API_BASE_URL}${path}`, init)
  } catch (cause) {
    if (signal?.aborted) {
      throw new AuthApiError({ status: 0, code: 'TIMEOUT', message: 'Request aborted.' })
    }
    void cause // no logging of any kind
    throw new AuthApiError({ status: 0, code: 'NETWORK_ERROR', message: 'Network request failed.' })
  }

  let payload: unknown
  const contentType = response.headers.get('content-type') ?? ''
  if (contentType.includes('json')) {
    try {
      payload = await response.json()
    } catch {
      payload = undefined
    }
  }

  if (!response.ok) {
    const envelope = isRecord(payload) ? payload : {}
    const node = isRecord(envelope.error) ? envelope.error : envelope
    const rawCode = node.code
    // The documented `error.code` is a NUMBER; tolerate string forms defensively.
    const code =
      typeof rawCode === 'number' && Number.isFinite(rawCode)
        ? String(rawCode)
        : typeof rawCode === 'string' && rawCode
          ? rawCode
          : `HTTP_${response.status}`
    throw new AuthApiError({
      status: response.status,
      code,
      message: pickString(node, 'message') ?? `Request failed (${response.status}).`,
    })
  }

  // Documented success envelope: { success: true, data }.
  const ok = payload as ApiOk<T> | undefined
  return (isRecord(ok) && 'data' in ok ? ok.data : (payload as T)) as T
}

// ---------------------------------------------------------------------------
// Public endpoint functions (documented payloads only)
// ---------------------------------------------------------------------------

/** POST /auth/register/consumer — multipart; creates the account (status: pending, is_verified: false) and triggers SMS OTP. */
export function registerConsumer(input: RegisterConsumerInput): Promise<AuthUser> {
  const form = new FormData()
  form.set('phone', input.phone)
  form.set('password', input.password)
  form.set('name', input.name)
  form.set('id_number', input.id_number)
  form.set('id_image', input.id_image)
  form.set('is_driver', String(input.is_driver))
  if (input.is_driver) {
    form.set('vehicle_number', input.vehicle_number ?? '')
    form.set('vehicle_type', input.vehicle_type ?? '')
  }
  form.set('is_home', String(input.is_home))
  if (input.is_home) form.set('family_count', String(input.family_count ?? ''))
  form.set('is_business', String(input.is_business))
  if (input.is_business) form.set('business_name', input.business_name ?? '')
  if (input.business_file) form.set('business_file', input.business_file)
  return authJson<AuthUser>('/auth/register/consumer', { method: 'POST', body: form })
}

/** POST /auth/resend-otp — requests a new 6-digit code (60s cooldown, 5 per 6h server-side). */
export function resendOtp(phone: string): Promise<null> {
  return authJson<null>('/auth/resend-otp', { method: 'POST', body: { phone } })
}

/** POST /auth/verify-phone — verifies the 6-digit code; account becomes is_verified: true. */
export function verifyPhone(input: { phone: string; code: string }): Promise<null> {
  return authJson<null>('/auth/verify-phone', { method: 'POST', body: input })
}

/** POST /auth/login — real authentication; the backend sets the HttpOnly `session` cookie. */
export function login(phone: string, password: string): Promise<AuthUser> {
  return authJson<AuthUser>('/auth/login', { method: 'POST', body: { phone, password } })
}

/** POST /auth/logout — destroys the Redis session and expires the cookie (backend-managed). */
export function logout(): Promise<null> {
  return authJson<null>('/auth/logout', { method: 'POST', body: {} })
}

/** POST /auth/forget-password — always 200 (anti-enumeration); SMS reset key valid 15 min. */
export function forgetPassword(phone: string): Promise<null> {
  return authJson<null>('/auth/forget-password', { method: 'POST', body: { phone } })
}

/**
 * POST /auth/reset-password — resets the password with the SMS reset key.
 *
 * TODO(BACKEND-FIELD-NAME): the backend JSON field is currently misspelled
 * `rest_key`; preserved exactly for compatibility. When the backend corrects
 * it to `reset_key`, rename this key in the same commit. Do not silently
 * rename it beforehand — the request would fail.
 */
export function resetPassword(restKey: string, password: string): Promise<null> {
  return authJson<null>('/auth/reset-password', {
    method: 'POST',
    body: { rest_key: restKey, password },
  })
}

// ---------------------------------------------------------------------------
// Client-side validation (mirrors the documented backend rules)
// ---------------------------------------------------------------------------

/**
 * Normalize a Yemeni phone to the backend's local form `7XXXXXXXX`.
 * Accepts: 7XXXXXXXX | 07XXXXXXXX | +9677XXXXXXXX | 009677XXXXXXXX.
 * Returns '' when the input cannot be normalized OR violates the documented
 * regex `^7[01378]\d{7}$` (prefixes 70/71/73/77/78 only).
 */
export function normalizePhone(raw: string): string {
  const digits = raw.replace(/\D/g, '')
  let local = ''
  if (/^7\d{8}$/.test(digits)) local = digits
  else if (/^07\d{8}$/.test(digits)) local = digits.slice(1)
  else if (/^9677\d{8}$/.test(digits)) local = digits.slice(3)
  else if (/^009677\d{8}$/.test(digits)) local = digits.slice(5)
  else return ''
  return /^7[01378]\d{7}$/.test(local) ? local : ''
}

export const PHONE_AR = 'أدخل رقم هاتف يمني صحيح يبدأ بـ ٧٠ أو ٧١ أو ٧٣ أو ٧٧ أو ٧٨ (مثال: 771234567).'
export const NAME_AR = 'الاسم الكامل من ١٠ إلى ٢٥٠ حرفاً.'
export const PASSWORD_AR = 'كلمة المرور من ٨ إلى ٢٥٥ حرفاً.'
export const ID_NUMBER_AR = 'رقم الهوية من ٩ إلى ٥٠ رقماً أو حرفاً.'

/** Documented password rule: min 8, max 255 characters. */
export function isPasswordValid(password: string): boolean {
  return password.length >= 8 && password.length <= 255
}

/** Documented ID image rule: JPEG/PNG/WEBP, max 5 MB, non-empty. */
export function isIdImageValid(file: File | null | undefined): string | null {
  if (!file) return 'صورة الهوية مطلوبة (JPEG أو PNG أو WEBP، بحد أقصى ٥ ميجابايت).'
  const allowed = ['image/jpeg', 'image/png', 'image/webp']
  if (!allowed.includes(file.type)) return 'نوع صورة الهوية غير مسموح به — استخدم JPEG أو PNG أو WEBP.'
  if (file.size === 0) return 'الملف المرفق فارغ.'
  if (file.size > 5 * 1024 * 1024) return 'حجم صورة الهوية يتجاوز الحد الأقصى (٥ ميجابايت).'
  return null
}

/** Documented business license rule: JPEG/PNG/WEBP/PDF, max 5 MB, non-empty. */
export function isBusinessFileValid(file: File | null | undefined): string | null {
  if (!file) return null // optional field
  const allowed = ['image/jpeg', 'image/png', 'image/webp', 'application/pdf']
  if (!allowed.includes(file.type)) return 'نوع الملف غير مسموح به — استخدم JPEG أو PNG أو WEBP أو PDF.'
  if (file.size === 0) return 'الملف المرفق فارغ.'
  if (file.size > 5 * 1024 * 1024) return 'حجم الملف يتجاوز الحد الأقصى (٥ ميجابايت).'
  return null
}

/** Documented OTP rule: exactly 6 numeric digits. */
export function isOtpCodeValid(code: string): boolean {
  return /^\d{6}$/.test(code)
}

/** Reset key: 10 alphanumeric characters (confirmed by the backend developer). */
export function isRestKeyValid(key: string): boolean {
  return /^[A-Za-z0-9]{10}$/.test(key)
}
