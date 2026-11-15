'use client'

import { ArrowLeft, BadgeCheck, Flame, Waves, Zap } from 'lucide-react'
import type { Screen, Service } from '@/components/mawared-types'
import { Brand } from '@/components/shared/brand'
import { BottomNav } from '@/components/shared/bottom-nav'

export function HomeScreen({ onChoose, onScreen, onProfile, avatarInitial }: { onChoose: (service: Service) => void; onScreen: (screen: Screen) => void; onProfile: () => void; avatarInitial?: string }) {
  const serviceCards = [
    { id: 'gas' as const, name: 'الغاز', status: 'تجريبي', icon: Flame, tone: 'gold' },
    { name: 'الماء', status: 'قريباً', icon: Waves, tone: 'muted', disabled: true },
    { name: 'الكهرباء', status: 'قريباً', icon: Zap, tone: 'muted', disabled: true },
  ]
  return <section className="home-screen">
    <header className="phone-header"><Brand /><button type="button" className="avatar-button" onClick={onProfile} aria-label={avatarInitial ? 'الملف الشخصي' : 'تسجيل الدخول'}>{avatarInitial ?? 'ض'}</button></header>
    <main className="phone-content home-content">
      <div className="greeting"><span className="greeting-mark"><Waves size={17} /></span><span>أهلاً وسهلاً</span><h1>منصة موارد</h1><p>إدارة احتياجات منزلك بضغطة زر</p></div>
      <div className="home-section-title"><h2>خدماتك الأساسية</h2><span>اختر الخدمة</span></div>
      <div className="service-grid">{serviceCards.map(({ id, name, status, icon: Icon, tone, disabled }) => <button type="button" key={name} disabled={disabled} className={`service-card ${tone}`} onClick={() => id && onChoose(id)}><span className="service-card-top"><span className="service-icon"><Icon size={23} strokeWidth={1.8} /></span><span className="service-status">{status}</span></span><strong>{name}</strong><span className="service-card-bottom">{disabled ? 'نعمل على توفيرها' : 'اطلب الآن'}{!disabled && <ArrowLeft size={14} />}</span></button>)}</div>
      <div className="home-hint"><span className="hint-icon"><BadgeCheck size={18} /></span><p>خدمات موثوقة، وخطوات سهلة لتلبية احتياجات منزلك.</p></div>
    </main>
    <BottomNav active="home" onScreen={onScreen} />
  </section>
}
