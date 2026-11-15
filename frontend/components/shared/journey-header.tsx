'use client'

import { ArrowRight } from 'lucide-react'

export function JourneyHeader({ title, onBack }: { title: string; onBack: () => void }) {
  return <header className="journey-header"><button type="button" className="back-button" onClick={onBack} aria-label="العودة"><ArrowRight size={20} /></button><h2>{title}</h2><span className="header-spacer" /></header>
}
