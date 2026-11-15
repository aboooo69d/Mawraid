export type Service = 'gas' | 'water'
export type Screen =
  | 'home'
  | 'request'
  | 'success'
  | 'tracking'
  | 'activity'
  | 'profile'
  | 'support'
  | 'login'
  | 'register'
  | 'otp'
  | 'forgot-password'
  | 'reset-password'
export type Order = { id: string; service: Service; quantity: number; name: string; district: string; address: string; payment: string }

/**
 * Booking form values captured when an unauthenticated user tries to submit.
 * Held by the app shell so the RequestScreen can restore every value after the
 * login/register/OTP round-trip (requirement: preserve the form during auth).
 */
export type BookingDraft = {
  service: Service
  name: string
  district: string
  quantity: number
  address: string
  payment: string
}

export const serviceInfo: Record<Service, { title: string; fullTitle: string; price: number; unit: string }> = {
  gas: { title: 'غاز', fullTitle: 'تعبئة الغاز المنزلي', price: 2500, unit: 'أسطوانة' },
  water: { title: 'ماء', fullTitle: 'تعبئة المياه المنزلية', price: 4000, unit: 'صهريج' },
}
export const money = (amount: number) => new Intl.NumberFormat('ar-YE').format(amount)
