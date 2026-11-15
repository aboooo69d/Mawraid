'use client'

import { useState } from 'react'
import type { BookingDraft, Order, Screen, Service } from '@/components/mawared-types'
import { useSession } from '@/lib/auth/session'
import { BottomNav } from '@/components/shared/bottom-nav'
import { ActivityScreen } from '@/components/screens/activity-screen'
import { HomeScreen } from '@/components/screens/home-screen'
import { ProfileScreen } from '@/components/screens/profile-screen'
import { RequestScreen } from '@/components/screens/request-screen'
import { SuccessScreen } from '@/components/screens/success-screen'
import { TrackingScreen } from '@/components/screens/tracking-screen'
import { SupportScreen } from '@/components/screens/support-screen'
import { LoginScreen } from '@/components/screens/auth/login-screen'
import { RegisterScreen } from '@/components/screens/auth/register-screen'
import { OtpScreen } from '@/components/screens/auth/otp-screen'
import { ForgotPasswordScreen } from '@/components/screens/auth/forgot-password-screen'
import { ResetPasswordScreen } from '@/components/screens/auth/reset-password-screen'

/** State handed to the OTP screen while a register/login flow is in progress. */
type OtpContext = {
  phone: string
  password: string
  /** Where the flow started, so "back" returns somewhere sensible. */
  origin: 'login' | 'register'
}

export default function MawaredApp() {
  const [screen, setScreen] = useState<Screen>('home')
  const [service, setService] = useState<Service>('gas')
  const [activeTab, setActiveTab] = useState<'home' | 'activity' | 'support'>('home')
  const [order, setOrder] = useState<Order | null>(null)
  // Citizen auth: real backend session (HttpOnly cookie owned by the Go
  // backend) with the documented user object cached locally by the session
  // store. NOTE: there is NO /auth/me endpoint (confirmed) — the cached user
  // is refreshed from backend responses only; no revalidation call is made.
  const session = useSession()
  const user = session?.user ?? null
  const [otpContext, setOtpContext] = useState<OtpContext | null>(null)
  const [draft, setDraft] = useState<BookingDraft | null>(null)
  // Where to return after a successful authentication: the booking form or
  // the profile screen.
  const [authIntent, setAuthIntent] = useState<'profile' | 'request'>('profile')
  // Phone captured in the forgot-password flow, and the success notice shown
  // on the login screen after a completed reset.
  const [forgotPhone, setForgotPhone] = useState<string | null>(null)
  const [loginNotice, setLoginNotice] = useState<string | null>(null)

  const showScreen = (next: Screen) => {
    setScreen(next)
    if (next === 'activity' || next === 'tracking') setActiveTab('activity')
    else if (next === 'support') setActiveTab('support')
    else if (next === 'request' || next === 'home') setActiveTab('home')
  }
  const submitOrder = (nextOrder: Order) => { setOrder(nextOrder); setDraft(null); showScreen('success') }
  const chooseService = (nextService: Service) => { setService(nextService); showScreen('request') }
  const backToHome = () => showScreen('home')
  const openProfile = () => { setAuthIntent('profile'); setLoginNotice(null); showScreen('profile') }

  /** Called by the login/OTP screens once a REAL backend session exists. */
  const finishAuth = () => {
    setOtpContext(null)
    setLoginNotice(null)
    if (authIntent === 'request' && draft) showScreen('request')
    else showScreen('profile')
  }
  /** Called when an unauthenticated booking submit is intercepted. */
  const requireAuthForBooking = (bookingDraft: BookingDraft) => {
    setDraft(bookingDraft)
    setAuthIntent('request')
    showScreen('login')
  }
  /** Back out of an auth screen: an unfinished booking returns to the form. */
  const leaveAuth = () => {
    setLoginNotice(null)
    if (authIntent === 'request' && draft) showScreen('request')
    else showScreen('home')
  }
  const handleLogout = () => {
    setDraft(null)
    setAuthIntent('profile')
    showScreen('home')
  }

  return <main className="app-shell" dir="rtl"><div className="app-frame">
    {screen === 'home' && <HomeScreen onChoose={chooseService} onScreen={showScreen} onProfile={openProfile} avatarInitial={user ? (user.name.trim().charAt(0) || 'م') : undefined} />}
    {screen === 'request' && <RequestScreen key={service} service={service} setService={setService} onSubmit={submitOrder} onBack={() => { setDraft(null); showScreen('home') }} userName={user?.name ?? null} draft={draft} onRequireAuth={requireAuthForBooking} />}
    {screen === 'success' && order && <SuccessScreen order={order} onTrack={() => showScreen('tracking')} onBack={backToHome} />}
    {screen === 'tracking' && order && <TrackingScreen order={order} onBack={() => showScreen('activity')} />}
    {screen === 'activity' && <ActivityScreen order={order} onTrack={() => showScreen(order ? 'tracking' : 'home')} />}
    {screen === 'profile' && <ProfileScreen onBack={backToHome} user={user} onLogin={() => { setAuthIntent('profile'); setLoginNotice(null); showScreen('login') }} onLogout={handleLogout} />}
    {screen === 'support' && <SupportScreen onBack={backToHome} />}
    {screen === 'login' && <LoginScreen initialPhone={otpContext?.phone} notice={loginNotice} onBack={leaveAuth} onSwitchToRegister={() => { setLoginNotice(null); showScreen('register') }} onSuccess={finishAuth} onNeedOtp={(context) => { setOtpContext({ ...context, origin: 'login' }); showScreen('otp') }} onForgotPassword={() => { setForgotPhone(null); showScreen('forgot-password') }} />}
    {screen === 'register' && <RegisterScreen onBack={() => showScreen('login')} onSwitchToLogin={() => showScreen('login')} onRegistered={(context) => { setOtpContext({ ...context, origin: 'register' }); showScreen('otp') }} />}
    {screen === 'otp' && otpContext && <OtpScreen phone={otpContext.phone} password={otpContext.password} onBack={() => showScreen('login')} onVerified={finishAuth} />}
    {screen === 'forgot-password' && <ForgotPasswordScreen onBack={() => showScreen('login')} onSent={(phone) => { setForgotPhone(phone); showScreen('reset-password') }} />}
    {screen === 'reset-password' && <ResetPasswordScreen phone={forgotPhone ?? undefined} onBack={() => showScreen('login')} onReset={() => { setLoginNotice('تم تغيير كلمة المرور بنجاح — سجّل الدخول بكلمة المرور الجديدة.'); showScreen('login') }} />}
    {screen !== 'home' && <BottomNav active={activeTab} onScreen={showScreen} />}
  </div></main>
}
