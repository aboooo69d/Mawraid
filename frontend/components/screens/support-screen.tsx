'use client'

import { useState } from 'react'
import { ArrowLeft, CircleHelp, Headphones, Phone, Plus } from 'lucide-react'
import { JourneyHeader } from '@/components/shared/journey-header'

export function SupportScreen({ onBack }: { onBack: () => void }) {
  const [openQuestion, setOpenQuestion] = useState<number | null>(null)
  const questions = ['كيف أتابع حالة طلبي؟', 'كيف أغيّر عنوان التوصيل؟', 'ما طرق الدفع المتاحة؟']
  return <div className="journey-screen"><JourneyHeader title="الدعم والمساعدة" onBack={onBack} /><main className="phone-content support-content"><div className="screen-intro"><span>نحن هنا لمساعدتك</span><h1>كيف نقدر نخدمك؟</h1></div><div className="support-options"><button type="button"><span className="support-option-icon"><CircleHelp size={19} /></span><span><strong>الأسئلة الشائعة</strong><small>إجابات سريعة لأسئلتك</small></span><ArrowLeft size={16} /></button><button type="button"><span className="support-option-icon chat"><Headphones size={19} /></span><span><strong>تواصل مع الدعم</strong><small>فريق موارد جاهز لمساعدتك</small></span><ArrowLeft size={16} /></button><button type="button"><span className="support-option-icon call"><Phone size={19} /></span><span><strong>اتصل بنا</strong><small dir="ltr">800 123 456</small></span><ArrowLeft size={16} /></button></div><div className="faq-heading"><h2>أسئلة يكثر طرحها</h2></div><div className="faq-list">{questions.map((question, index) => <button className="faq-item" type="button" key={question} onClick={() => setOpenQuestion(openQuestion === index ? null : index)} aria-expanded={openQuestion === index}><span>{question}</span><Plus size={16} />{openQuestion === index && <small>{index === 0 ? 'افتح تبويب النشاط ثم اختر طلبك لمشاهدة آخر تحديث.' : index === 1 ? 'يمكنك التواصل مع فريق الدعم لتحديث عنوان التوصيل قبل تجهيز الطلب.' : 'يمكنك الدفع نقداً عند الاستلام أو باستخدام محفظة موارد.'}</small>}</button>)}</div></main></div>
}
