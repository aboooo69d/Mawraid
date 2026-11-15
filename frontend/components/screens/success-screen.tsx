'use client'

import { ArrowLeft, Check, CheckCircle2 } from 'lucide-react'
import type { Order } from '@/components/mawared-types'
import { money, serviceInfo } from '@/components/mawared-types'
import { JourneyHeader } from '@/components/shared/journey-header'

export function SuccessScreen({ order, onTrack, onBack }: { order: Order; onTrack: () => void; onBack: () => void }) {
  const details = serviceInfo[order.service]
  return <div className="journey-screen"><JourneyHeader title="تأكيد الطلب" onBack={onBack} /><main className="phone-content result-content"><div className="success-symbol"><Check size={39} strokeWidth={2.4} /></div><span className="success-kicker">نسخة تجريبية</span><h1>تم تسجيل الطلب محليًا</h1><p className="success-subtitle">تم إنشاء طلب تجريبي رقم <strong>{order.id}</strong> بنجاح.</p><section className="receipt-card"><div className="receipt-title"><span>ملخص الطلب</span><span className="receipt-status"><CheckCircle2 size={14} /> قيد المراجعة</span></div><div className="receipt-row"><span>الخدمة</span><strong>{details.fullTitle} - {money(details.price)} ريال</strong></div><div className="receipt-row"><span>الكمية</span><strong>{money(order.quantity)} {details.unit}</strong></div><div className="receipt-row"><span>الحي / المنطقة</span><strong>{order.district}</strong></div><div className="receipt-row"><span>عنوان التوصيل</span><strong>{order.address}</strong></div><div className="receipt-row"><span>طريقة الدفع</span><strong>{order.payment}</strong></div><div className="receipt-total"><span>الإجمالي</span><strong>{money(details.price * order.quantity)} ريال</strong></div></section><button type="button" className="confirm-button" onClick={onTrack}>تتبع طلبي <ArrowLeft size={17} /></button><button type="button" className="quiet-button" onClick={onBack}>العودة إلى الخدمات</button></main></div>
}
