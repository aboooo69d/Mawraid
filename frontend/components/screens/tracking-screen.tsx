'use client'

import type { Order } from '@/components/mawared-types'
import { money, serviceInfo } from '@/components/mawared-types'
import { JourneyHeader } from '@/components/shared/journey-header'

export function TrackingScreen({ onBack, order }: { onBack: () => void; order: Order }) {
  return <div className="journey-screen"><JourneyHeader title="متابعة الطلب" onBack={onBack} /><main className="phone-content tracking-content">
    <div className="screen-intro"><span>رقم الطلب: {order.id}</span><h1>الطلب قيد المراجعة (تجريبي)</h1></div>
    <section className="receipt-card"><div className="receipt-row"><span>الخدمة</span><strong>{serviceInfo[order.service].fullTitle}</strong></div><div className="receipt-row"><span>الكمية</span><strong>{money(order.quantity)}</strong></div><div className="receipt-row"><span>المنطقة</span><strong>{order.district}</strong></div><div className="receipt-row"><span>العنوان</span><strong>{order.address}</strong></div></section>
    <p className="demo-note">هذه محاكاة للواجهة فقط. لم يُرسل الطلب إلى وكيل، ولم تُحجز كمية من المخزون بعد. حالة التسليم الفعلية ستظهر بعد ربط الخادم.</p>
  </main></div>
}
