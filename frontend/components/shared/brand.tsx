'use client'

import { Waves } from 'lucide-react'

export function Brand({ small = false }: { small?: boolean }) {
  return <div className={`brand ${small ? 'brand-small' : ''}`}><span className="brand-icon"><Waves size={small ? 17 : 20} strokeWidth={2.4} /></span><span className="brand-copy"><strong>موارد</strong><small>منصة الخدمات الأساسية</small></span></div>
}
