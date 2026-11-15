'use client'

import { useState } from 'react'
import { ArrowLeft, BadgeCheck, ChevronDown, Flame, Lock, Minus, Plus } from 'lucide-react'
import type { BookingDraft, Order, Service } from '@/components/mawared-types'
import { money, serviceInfo } from '@/components/mawared-types'
import { JourneyHeader } from '@/components/shared/journey-header'

/**
 * Booking form. Publicly browsable, but SUBMISSION requires a real backend
 * session: an unauthenticated submit captures every field into a
 * `BookingDraft` (held by the app shell) and routes to login — after the
 * auth round-trip the form is restored exactly as the user left it.
 *
 * HONESTY: the backend has no orders API in Sprint 1, so a successful submit
 * stays a LOCAL demo order and the note below says exactly that — nothing is
 * claimed to be persisted server-side.
 */
export function RequestScreen({
  service,
  setService,
  onSubmit,
  onBack,
  userName,
  draft,
  onRequireAuth,
}: {
  service: Service
  setService: (service: Service) => void
  onSubmit: (order: Order) => void
  onBack: () => void
  /** Authenticated citizen's full name, or null when browsing as a guest. */
  userName: string | null
  /** Preserved form values from an interrupted authentication, if any. */
  draft: BookingDraft | null
  onRequireAuth: (draft: BookingDraft) => void
}) {
  const restored = draft && draft.service === service ? draft : null
  const [name, setName] = useState(restored?.name ?? userName ?? '')
  const [district, setDistrict] = useState(restored?.district ?? 'حي التحرير')
  const [quantity, setQuantity] = useState(restored?.quantity ?? 1)
  const [address, setAddress] = useState(restored?.address ?? '')
  const [payment, setPayment] = useState(restored?.payment ?? 'الدفع النقدي عند الاستلام')
  const details = serviceInfo[service]
  const total = details.price * quantity

  const submit = (event: React.FormEvent) => {
    event.preventDefault()
    if (!userName) {
      // Preserve everything before leaving the form for the auth flow.
      onRequireAuth({ service, name, district, quantity, address, payment })
      return
    }
    onSubmit({ id: `MR-${Date.now().toString(36).toUpperCase()}`, service, quantity, name, district, address, payment })
  }

  return <div className="journey-screen">
    <JourneyHeader title={service === 'gas' ? 'طلب أسطوانة غاز' : 'طلب صهريج ماء'} onBack={onBack} />
    <main className="phone-content request-content">
      <section className={`service-detail ${service}`}><div className="dome-icon"><Flame size={25} fill="currentColor" /></div><div className="service-detail-copy"><span>خدمة موارد المنزلية</span><h1>{details.fullTitle}</h1><strong>سعر توضيحي: {money(details.price)} ريال</strong></div></section>
      <p className="service-description">{service === 'gas' ? 'أسطوانة غاز منزلية معبأة وجاهزة للاستلام من أقرب نقطة توزيع.' : 'مياه منزلية نظيفة تُسلّم إلى عنوانك أو نقطة التوزيع الأقرب إليك.'}</p>
      <form className="order-form" onSubmit={submit}>
        <label className="form-field"><span>الاسم الكامل</span><input value={name} onChange={(event) => setName(event.target.value)} placeholder="أدخل اسمك الثلاثي" required /></label>
        <label className="form-field"><span>الحي / المنطقة</span><span className="select-wrap"><select value={district} onChange={(event) => setDistrict(event.target.value)}><option>حي التحرير</option><option>حي الثورة</option><option>شارع حدة</option><option>حي الوحدة</option><option>حي الصافية</option><option>شارع الزبيري</option></select><ChevronDown size={16} /></span></label>
        <div className="form-field"><span>عدد {details.unit === 'أسطوانة' ? 'الأسطوانات' : 'الصهاريج'}</span><div className="quantity-row"><div><strong>{details.unit === 'أسطوانة' ? 'أسطوانة غاز قياسية' : 'صهريج مياه منزلي'}</strong><small>الحد الأقصى: ٣ {details.unit}</small></div><div className="quantity-stepper"><button type="button" aria-label="تقليل الكمية" disabled={quantity <= 1} onClick={() => setQuantity((current) => Math.max(1, current - 1))}><Minus size={15} /></button><strong>{money(quantity)}</strong><button type="button" aria-label="زيادة الكمية" disabled={quantity >= 3} onClick={() => setQuantity((current) => Math.min(3, current + 1))}><Plus size={15} /></button></div></div></div>
        <label className="form-field"><span>عنوان التوصيل الدقيق</span><input value={address} onChange={(event) => setAddress(event.target.value)} placeholder="أدخل رقم المنزل والشارع، أو علامة مميزة" required /></label>
        <label className="form-field"><span>طريقة الدفع</span><span className="select-wrap"><select value={payment} onChange={(event) => setPayment(event.target.value)}><option>الدفع النقدي عند الاستلام</option></select><ChevronDown size={16} /></span></label>
        <div className="total-row"><span>الإجمالي التوضيحي</span><strong>{money(total)} <small>ريال</small></strong></div>
        {!userName && <button className="confirm-button" type="submit" formNoValidate><Lock size={16} /> سجّل الدخول ثم أكمل الطلب</button>}
        {userName && <button className="confirm-button" type="submit">تأكيد الطلب <ArrowLeft size={17} /></button>}
        {!userName
          ? <p className="secure-note"><Lock size={14} /> يتطلب تأكيد الطلب تسجيل الدخول — بياناتك محفوظة حتى العودة</p>
          : <p className="secure-note"><BadgeCheck size={14} /> نموذج تجريبي يُحفظ محلياً — ربط الطلبات بخادم موارد ضمن التكامل القادم</p>}
      </form>
    </main>
  </div>
}
