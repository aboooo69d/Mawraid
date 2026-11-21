# دليل تكامل واجهة المستخدم — إدارة تسجيلات المستخدمين (Admin) — Nova API

> **النطاق:** قبول أو حظر المستخدمين الجدد من لوحة تحكم الأدمن  
> **المسار الأساسي:** `/api/v1/admin/users`

---

## 1. قواعد أساسية

- **المصادقة:** الجلسات عبر Cookie، فعّل إرسال الكوكيز في كل طلب:
  - Axios: `withCredentials: true`
  - Fetch: `credentials: 'include'`
- **الصلاحية:** جميع المسارات للأدمن فقط (`user.role_name === "admin"` من استجابة تسجيل الدخول).
  - بدون جلسة ← `401`
  - جلسة بدور غير أدمن ← `403` (الكود `1107`)
- **الاستجابة الموحدة:**

```json
// نجاح
{ "success": true, "data": { }, "meta": { "page": 1, "per_page": 10, "total": 45, "total_pages": 5 } }

// خطأ
{ "success": false, "error": { "status": 404, "code": 1000, "message": "user not found" } }
```

> `meta` تظهر في مسار القائمة فقط.

## 2. حالات المستخدم (`status`)

| القيمة | المعنى |
|---|---|
| `pending` | تسجيل جديد بانتظار المراجعة (الافتراضي في القائمة) |
| `docs` | بانتظار المستندات |
| `active` | مقبول / فعّال |
| `rejected` | محظور / مرفوض |
| `suspended` | موقوف |

---

## 3. المسارات

### 3.1 قائمة المستخدمين الجدد (مع ترقيم الصفحات)

`GET /api/v1/admin/users`

**Query Params (كلها اختيارية):**

| الاسم | النوع | الافتراضي | ملاحظات |
|---|---|---|---|
| `page` | int | `1` | ≥ 1 |
| `per_page` | int | `10` | من 1 إلى 100 |
| `status` | string | `pending` | `pending` \| `docs` \| `active` \| `rejected` \| `suspended` |

**مثال:** `GET /api/v1/admin/users?page=1&per_page=10`

**استجابة 200:**

```json
{
  "success": true,
  "data": [
    {
      "id": "b3c1f0e2-6a1d-4c55-9d1e-2f7a8d9e1a10",
      "phone": "771234567",
      "is_verified": true,
      "status": "pending",
      "role_id": 3,
      "role_name": "consumer",
      "created_at": "2026-11-20T10:00:00Z",
      "updated_at": "2026-11-20T10:00:00Z"
    }
  ],
  "meta": { "page": 1, "per_page": 10, "total": 45, "total_pages": 5 }
}
```

الترتيب: الأحدث أولاً. عند عدم وجود نتائج تكون `data` فارغة (قد يتم حذفها من JSON) فتعامل مع `data ?? []`.

### 3.2 تفاصيل مستخدم

`GET /api/v1/admin/users/:id`  (`id` من نوع UUID)

**استجابة 200** (يظهر `consumer` أو `provider` حسب نوع الحساب):

```json
{
  "success": true,
  "data": {
    "id": "b3c1f0e2-6a1d-4c55-9d1e-2f7a8d9e1a10",
    "phone": "771234567",
    "is_verified": true,
    "status": "pending",
    "role_id": 3,
    "role_name": "consumer",
    "created_at": "2026-11-20T10:00:00Z",
    "updated_at": "2026-11-20T10:00:00Z",
    "consumer": {
      "name": "اسم المستخدم الكامل",
      "id_number": "01234567890",
      "id_image": "path/to/image.jpg",
      "is_driver": true,
      "vehicle_number": "123456",
      "vehicle_type": "truck",
      "is_home": false,
      "is_business": true,
      "business_name": "متجر النور",
      "business_file": "path/to/file.pdf"
    }
  }
}
```

حساب مزود الخدمة يحتوي بدلاً من ذلك على:

```json
"provider": { "name": "...", "description": "...", "start_at": "2026-11-20T08:00:00Z", "end_at": "2026-11-20T16:00:00Z" }
```

الحقول الاختيارية (`vehicle_*`, `family_count`, `business_*`, `id_image`) قد تكون غير موجودة.

### 3.3 قبول أو حظر مستخدم

`PATCH /api/v1/admin/users/:id/status`

**Body:**

```json
{ "action": "accept" }
```

| `action` | الحالة الناتجة |
|---|---|
| `accept` | `active` |
| `block` | `rejected` |

**استجابة 200:** بيانات المستخدم بعد التحديث (نفس حقول عنصر القائمة) و`status` الجديدة.

---

## 4. أكواد الأخطاء

| HTTP | code | الحالة | التصرف المقترح في الواجهة |
|---|---|---|---|
| 400 | `2000` | `id` غير صالح أو Body/Query غير صحيح (مثلاً `action` غير مدعوم) | عرض رسالة خطأ عامة |
| 401 | `1106` | لا توجد جلسة | تحويل لصفحة تسجيل الدخول |
| 403 | `1107` | المستخدم ليس أدمن | عرض "غير مصرح" |
| 404 | `1000` | المستخدم غير موجود | العودة للقائمة وتحديثها |
| 409 | — | المستخدم بالفعل في هذه الحالة | تحديث البيانات وإخفاء الزر |
| 500 | `5000` | خطأ في الخادم | رسالة "حاول لاحقاً" |

---

## 5. تدفق الشاشات

1. **قسم "المستخدمون الجدد":** استدعاء `GET /admin/users?page=1&per_page=10`، عرض الجدول وأزرار الصفحات اعتماداً على `meta.total_pages`.
2. **الضغط على مستخدم:** الانتقال لصفحة التفاصيل واستدعاء `GET /admin/users/:id`، وعرض بيانات `consumer` أو `provider`.
3. **زر "قبول" / "حظر":** استدعاء `PATCH /admin/users/:id/status` ثم عند النجاح عرض إشعار نجاح والعودة للقائمة (المستخدم لن يظهر فيها لأن حالته لم تعد `pending`).
   - عطّل الأزرار أثناء الطلب لتجنب النقر المزدوج، ويُفضّل تأكيد قبل "حظر".

## 6. أمثلة كود

```ts
import axios from "axios";

const api = axios.create({ baseURL: "/api/v1", withCredentials: true });

export const listNewUsers = (page = 1, perPage = 10, status = "pending") =>
  api.get("/admin/users", { params: { page, per_page: perPage, status } })
     .then(r => ({ items: r.data.data ?? [], meta: r.data.meta }));

export const getUser = (id: string) =>
  api.get(`/admin/users/${id}`).then(r => r.data.data);

export const changeUserStatus = (id: string, action: "accept" | "block") =>
  api.patch(`/admin/users/${id}/status`, { action }).then(r => r.data.data);
```

## 7. ملاحظات

- مستخدم بحالة `rejected` أو `suspended` لا يستطيع تسجيل الدخول (يعرض الخادم أخطاء الحالة 1500–1502).
- يمكن للأدمن قبول مستخدم محظور سابقاً بتغيير `status` إلى `rejected` في القائمة ثم استخدام `accept`.
- يجب أن يكون دور `admin` موجوداً في جدول الأدوار ومسنداً لحسابات الأدمن.
