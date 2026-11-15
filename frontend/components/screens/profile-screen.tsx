'use client'

import { useState } from 'react'
import { ArrowLeft, BadgeCheck, Calendar, LogOut, Phone, ShieldCheck, User, Wallet } from 'lucide-react'
import type { AuthUser } from '@/lib/auth/api'
import { logoutSession } from '@/lib/auth/session'
import { JourneyHeader } from '@/components/shared/journey-header'

/** Arabic labels for the backend's documented account status values. */
const STATUS_LABELS: Record<string, string> = {
  active: 'الحساب نشط',
  pending: 'قيد المراجعة',
}
const STATUS_TONES: Record<string, string> = {
  active: 'ok',
  pending: 'warn',
}
const ROLE_LABELS: Record<string, string> = { consumer: 'مواطن', provider: 'مقدم خدمة' }

function formatDate(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return '—'
  return new Intl.DateTimeFormat('ar-YE', { dateStyle: 'long' }).format(date)
}

/**
 * Profile (Arabic RTL). Shows the cached REAL account object returned by the
 * documented login response (there is NO /auth/me endpoint — confirmed), with
 * a working logout that asks the backend to destroy the session/cookie.
 * Guests see a sign-in prompt instead — no fabricated identity.
 */
export function ProfileScreen({
  onBack,
  user,
  onLogin,
  onLogout,
}: {
  onBack: () => void
  user: AuthUser | null
  onLogin: () => void
  onLogout: () => void
}) {
  const [loggingOut, setLoggingOut] = useState(false)

  const logout = async () => {
    if (loggingOut) return
    setLoggingOut(true)
    try {
      await logoutSession()
    } finally {
      setLoggingOut(false)
      onLogout()
    }
  }

  if (!user) {
    return <div className="journey-screen">
      <JourneyHeader title="الملف الشخصي" onBack={onBack} />
      <main className="phone-content profile-content">
        <div className="profile-hero"><span className="profile-large-avatar">ض</span><h1>أنت تتصفح كزائر</h1><span>سجّل الدخول لإدارة طلباتك وخدماتك</span></div>
        <div className="profile-actions">
          <button className="confirm-button" type="button" onClick={onLogin}><ShieldCheck size={16} /> تسجيل الدخول</button>
        </div>
        <p className="demo-note">يمكنك تصفح الخدمات والأسعار التوضيحية دون تسجيل الدخول — يُطلب الدخول فقط عند تأكيد الطلب.</p>
      </main>
    </div>
  }

  return <div className="journey-screen">
    <JourneyHeader title="الملف الشخصي" onBack={onBack} />
    <main className="phone-content profile-content">
      <div className="profile-hero">
        <span className="profile-large-avatar">{user.name.trim().charAt(0) || 'م'}</span>
        <h1>{user.name}</h1>
        <span className={`profile-pill ${STATUS_TONES[user.status] ?? 'warn'}`}>{STATUS_LABELS[user.status] ?? user.status}</span>
      </div>
      <section className="profile-card">
        <div className="profile-row"><span className="profile-row-icon"><Phone size={17} /></span><div><span>رقم الهاتف</span><strong dir="ltr">{user.phone}</strong></div></div>
        <div className="profile-row"><span className="profile-row-icon"><BadgeCheck size={17} /></span><div><span>حالة توثيق الرقم</span><strong>{user.is_verified ? 'موثق' : 'غير موثق'}</strong></div></div>
        <div className="profile-row"><span className="profile-row-icon"><User size={17} /></span><div><span>نوع الحساب</span><strong>{ROLE_LABELS[user.role_name] ?? user.role_name}</strong></div></div>
        <div className="profile-row"><span className="profile-row-icon"><Calendar size={17} /></span><div><span>تاريخ إنشاء الحساب</span><strong>{formatDate(user.created_at)}</strong></div></div>
        <div className="profile-row"><span className="profile-row-icon"><Wallet size={17} /></span><div><span>رصيد محفظة موارد</span><strong>غير متاحة بعد</strong></div><ArrowLeft size={16} className="profile-chevron" /></div>
      </section>
      <div className="profile-actions">
        <button className="profile-edit-button" type="button" onClick={logout} disabled={loggingOut}>
          <LogOut size={15} /> {loggingOut ? 'جارٍ تسجيل الخروج…' : 'تسجيل الخروج'}
        </button>
      </div>
      <p className="demo-note">تعديل بيانات الحساب (الاسم/العنوان) سيُتاح بعد ربط واجهات الحساب في الخادم.</p>
    </main>
  </div>
}
