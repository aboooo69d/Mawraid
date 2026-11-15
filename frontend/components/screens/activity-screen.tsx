'use client'

import { ArrowLeft } from 'lucide-react'
import type { Order } from '@/components/mawared-types'
import { money } from '@/components/mawared-types'
import { JourneyHeader } from '@/components/shared/journey-header'

export function ActivityScreen({ onTrack, order }: { onTrack: () => void; order: Order | null }) {
  return <div className="journey-screen"><JourneyHeader title="النشاط" onBack={onTrack} /><main className="phone-content activity-content"><div className="screen-intro"><span>طلباتي</span><h1>النشاط والطلبات</h1></div>
    {order ? <button type="button" className="activity-order" onClick={onTrack}><strong>طلب غاز ({order.id})</strong><span className="activity-meta">{money(order.quantity)} أسطوانة · قيد المراجعة (تجريبي)</span><span className="activity-link">عرض التفاصيل <ArrowLeft size={14} /></span></button> : <div className="empty-orders">لا توجد طلبات بعد. ابدأ طلب الغاز من الرئيسية.</div>}
  </main></div>
}
