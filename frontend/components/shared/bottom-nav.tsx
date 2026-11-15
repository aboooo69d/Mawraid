'use client'

import { Activity, HelpCircle, Home } from 'lucide-react'
import type { Screen } from '@/components/mawared-types'

export function BottomNav({ active, onScreen }: { active: 'home' | 'activity' | 'support'; onScreen: (screen: Screen) => void }) {
  const items = [
    { id: 'home' as const, label: 'الرئيسية', icon: Home, screen: 'home' as Screen },
    { id: 'activity' as const, label: 'النشاط', icon: Activity, screen: 'activity' as Screen },
    { id: 'support' as const, label: 'الدعم', icon: HelpCircle, screen: 'support' as Screen },
  ]
  return <nav className="bottom-nav" aria-label="التنقل الرئيسي">{items.map(({ id, label, icon: Icon, screen }) => <button key={id} type="button" className={active === id ? 'active' : ''} aria-current={active === id ? 'page' : undefined} onClick={() => onScreen(screen)}><Icon size={19} strokeWidth={active === id ? 2.3 : 1.8} /><span>{label}</span></button>)}</nav>
}
