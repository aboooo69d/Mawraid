# 📋 تقرير مراجعة الكود والتدقيق الأمني والمعماري (Code Review & Security Audit)

**المشروع:** Mawraid (Nova API Backend)  
**اللغة والبيئة:** Go 1.23+ / Gin Framework / PostgreSQL / Redis / Cloudflare R2  
**تاريخ المراجعة:** 13 نوفمبر 2026  
**نوع المراجعة:** مراجعة معمارية، تدقيق أمني، وتحليل جودة الكود (Comprehensive Architecture, Security & Code Quality Audit)  

---

## 📑 الفهرس (Table of Contents)
1. [الملخص التنفيذي (Executive Summary)](#1-الملخص-التنفيذي-executive-summary)
2. [مصفوفة تصنيف الملاحظات (Severity Matrix)](#2-مصفوفة-تصنيف-الملاحظات-severity-matrix)
3. [الملاحظات الحرجة (Critical Severity)](#3-الملاحظات-الحرجة-critical-severity)
4. [الملاحظات عالية الخطورة (High Severity)](#4-الملاحظات-عالية-الخطورة-high-severity)
5. [الملاحظات متوسطة الخطورة (Medium Severity)](#5-الملاحظات-متوسطة-الخطورة-medium-severity)
6. [تحسينات الأداء والمعمارية (Quality, Performance & Clean Code)](#6-تحسينات-الأداء-والمعمارية-quality-performance--clean-code)
7. [قائمة التحقق والتوصيات للإنتاج (Production Checklist)](#7-قائمة-التحقق-والتوصيات-للإنتاج-production-checklist)

---

## 1. الملخص التنفيذي (Executive Summary)

تم إجراء مراجعة شاملة لشيفرة المصدر الخاصة بنظام **Mawraid (Nova API)** والتي تغطي طبقات المعالجة (Handlers)، المنطق التجاري (Services)، طبقة الوصول للبيانات (Repositories & Entities)، التخزين السحابي المشفر (Cloudflare R2 Storage & Crypto)، ووسائط التحقق (Middlewares).

أظهرت المراجعة متانة جيدة في التصميم المعماري العام، واعتماد مبادئ نظيفة مثل:
* استخدام حاوية حقن التبعيات (Dependency Injection Container).
* تشفير الملفات الحساسة المرفوعة قبل حفظها في الـ Object Storage باستخدام AES-256-GCM.
* الفصل الواضح بين الـ DTOs والـ Entities وقواعد التخزين المؤقت عبر Redis.

مع ذلك، تم رصد **مجموعة من الأخطاء الحرجة ومخاطر الأداء** التي تتطلب تدخلاً عاجلاً قبل الإطلاق للإنتاج (Production)، أبرزها:
1. استدعاءات قد تسبب انهياراً مفاجئاً (Panic) في الـ Middleware الخاص بالتحقق من Cloudflare Turnstile بسبب كود حالة HTTP غير معرف (`Status = 0`).
2. خطأ في إعدادات الاتصال بمخزن الجلسات (Sessions Redis Store) يعطل ربط العناوين.
3. استهلاك غير مبرر ومكثف لموارد المعالج (CPU Exhaustion / DoS Risk) ناتج عن استخدام تكلفة تجزئة مفرطة (`Bcrypt Cost = 14`) للتحقق من رموز الـ OTP السريعة.
4. غياب الحد من التكرار (Cooldown/Rate-limiting) في مسار استعادة كلمة المرور مما يعرض النظام لاستنزاف رصيد رسائل SMS (SMS Bombing).

---

## 2. مصفوفة تصنيف الملاحظات (Severity Matrix)

| المعرف | التصنيف | الملف المستهدف | ملخص الملاحظة | الأثر التقني |
| :--- | :--- | :--- | :--- | :--- |
| **SEC-01** | 🔴 **حرج (Critical)** | `middleware/cf_turnstile_handuler.go` | كود حالة HTTP غير محدد (`Status = 0`) عند رفض الطلب | حدوث **Panic** وانهيار الطلب في خادم Gin |
| **CFG-01** | 🔴 **حرج (Critical)** | `config/config.go` | قراءة متغير `SESSION_NETWORK` لحقل العنوان `Address` | فشل الاتصال بخادم Redis للجلسات |
| **PERF-01** | 🟠 **عالي (High)** | `pkg/value_hashing/value_hashing.go` & `modules/auth/repo/otp_repo.go` | استخدام Bcrypt بتكلفة 14 لتجزئة رموز OTP مؤقتة | استنزاف المعالج بنسبة 100% وهجمات حجب خدمة (DoS) |
| **SEC-02** | 🟠 **عالي (High)** | `modules/auth/service/auth_service.go` | غياب الـ Cooldown عند طلب استعادة كلمة المرور | استنزاف باقات رسائل الـ SMS وإزعاج المستخدمين |
| **LOGIC-01** | 🟡 **متوسط (Medium)** | `modules/auth/service/otp_service.go` | إرجاع خطأ `ErrInvalidOtpCode` عند فشل مزود الـ SMS | تضليل المستخدم بخطأ غير صحيح |
| **DB-01** | 🟡 **متوسط (Medium)** | `modules/auth/service/auth_service.go` | عدم فحص قيد تفرد رقم الهوية `id_number` برمجياً | انهيار العملية بخطأ 500 بدلاً من 409 Conflict |
| **CODE-01** | 🟡 **متوسط (Medium)** | `modules/auth/dto/auth_dto.go` | تعارض أكواد الأخطاء وتكرار رسائل غير دقيقة بالـ DTO | أخطاء في الـ API Contract وتجربة المطور |
| **CLEAN-01** | 🟢 **منخفض (Low)** | `modules/auth/handler/auth_handler.go` & `middleware/cf_turnstile_handuler.go` | عبارات `fmt.Println` و `print` في بيئة الإنتاج | تلوث السجلات وتسريب تلميحات تشغيلية |

---

## 3. الملاحظات الحرجة (Critical Severity)

### 🔴 SEC-01: حدوث Panic محتمل بسبب كود حالة HTTP بقيمة 0 في Turnstile Middleware
* **الموقع:** `middleware/cf_turnstile_handuler.go` (الأسطر 35-46)
* **الوصف:**
  يتم إنشاء كائن الخطأ `reqErr` دون تحديد حقل `Status`:
  ```go
  reqErr := &respons.AppError{
      Code: errs_code.ErrTurnstileToken,
  }
  ```
  وعند فشل التحقق يتم استدعاء:
  ```go
  respons.Abort(ctx, reqErr)
  ```
  والتي تقوم بتنفيذ `ctx.AbortWithStatusJSON(err.Status, ...)` حيث تكون قيمة `err.Status` مساوية لـ `0`.
* **الأثر:**
  في معيار Go `net/http` ومكتبة Gin، كتابة كود حالة HTTP غير صالح (مثل `0`) تؤدي إلى حدوث Panic وإرجاع خطأ داخلي فجائي.
* **الحل المقترح:**
  تحديد كود الحالة بوضوح (مثلاً `403 Forbidden` أو `400 Bad Request`):
  ```go
  reqErr := &respons.AppError{
      Status:  http.StatusForbidden,
      Code:    errs_code.ErrTurnstileToken,
      Message: "Cloudflare Turnstile token validation failed",
  }
  ```

---

### 🔴 CFG-01: خطأ في تسمية متغير البيئة الخاص بمخزن الجلسات
* **الموقع:** `config/config.go` (السطر 114)
* **الوصف:**
  في دالة بناء الإعدادات `NewConfig`:
  ```go
  Session: SessionConfg{
      Size:     size,
      Network:  getEnv("SESSION_NETWORK", "tcp"),
      Address:  getEnv("SESSION_NETWORK", "localhost:6379"), // خطأ برمجي هنا
      Username: getEnv("SESSION_USERNAME", ""),
      Password: getEnv("SESSION_PASSWORD", ""),
      KeyPairs: []byte(getEnv("SESSION_ENC_KEY", "encryption-key")),
  },
  ```
* **الأثر:**
  إذا قام مسؤول الخادم بضبط `SESSION_NETWORK=tcp` في بيئة الإنتاج، فسيأخذ المتغير `Address` القيمة `"tcp"` بدلاً من عنوان السيرفر والمنفذ (`redis-host:6379`)، مما يتسبب في فشل الاتصال بريديس لجميع عمليات تسجيل الدخول والجلسات.
* **الحل المقترح:**
  تصحيح اسم المتغير البيئي ليصبح `SESSION_ADDRESS`:
  ```go
  Address: getEnv("SESSION_ADDRESS", "localhost:6379"),
  ```

---

## 4. الملاحظات عالية الخطورة (High Severity)

### 🟠 PERF-01: كلفة تجزئة مفرطة (Bcrypt Cost 14) لرموز الـ OTP السريعة
* **الموقع:** `pkg/value_hashing/value_hashing.go` و `modules/auth/repo/otp_repo.go`
* **الوصف:**
  تستخدم الدالة `HashValue` معامل كلفة مرتفع جداً:
  ```go
  func HashValue(password string) (string, error) {
      bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
      return string(bytes), err
  }
  ```
  هذا التابع يُستدعى أيضاً في `otp_repo.go` لتشفير رمز التحقق المكون من 6 أرقام فقط والصالح لمدة 5 دقائق:
  ```go
  hashedCode, err := valueHashing.HashValue(code)
  ```
* **الأثر:**
  * كلفة `14` في Bcrypt تتطلب حوالي 800ms - 1200ms من طاقة المعالج (100% CPU core) للعملية الواحدة.
  * في حال قام مهاجم بطلب 10 إلى 20 رمز OTP في وقت واحد، سيتجمد معالج الخادم بالكامل ويتوقف عن الرد على المستخدمين الآخرين (Severe CPU Denial of Service).
* **الحل المقترح:**
  1. ضبط كلفة كلمات المرور على المستوى الافتراضي الموصى به من معايير أمان OWASP وهو `bcrypt.DefaultCost` (تكلفة 10) أو بحد أقصى `12`.
  2. استخدام خوارزمية تجزئة سريعة وفعالة ومخصصة للأكواد قصيرة الأجل (مثل `HMAC-SHA256` مع Secret Key داخلي) لتشفير الـ OTP في Redis.

---

### 🟠 SEC-02: غياب Cooldown و Rate-limiting في مسار استعادة كلمة المرور
* **الموقع:** `modules/auth/service/auth_service.go` (دالة `ForgetPassword`)
* **الوصف:**
  ```go
  if code, err := s.redis.Get(c, "forget:"+phone).Result(); err == nil {
      go s.sms.SendSMS(phone, "your repassword code is:"+code+"don't share it")
      return nil
  }
  ```
* **الأثر:**
  يستطيع أي طرف إرسال مئات الطلبات المتتالية إلى مسار `/auth/forget-password` لنفس رقم الهاتف، وفي كل مرة سيقوم النظام باستدعاء خدمة الـ SMS وإرسال رسالة نصية فوراً. يؤدي ذلك إلى هجمات **SMS Flooding / Bombing** واستنزاف فواتير الرسائل القصيرة.
* **الحل المقترح:**
  إضافة قيد زمني (Cooldown) لمدة دقيقة على الأقل في Redis يمنع إعادة إرسال الكود خلال هذه الفترة (مماثل للمنطق المطبق في `OtpCooldown`).

---

## 5. الملاحظات متوسطة الخطورة (Medium Severity)

### 🟡 LOGIC-01: إرجاع رسالة خطأ مضللة عند تعطل مزود الرسائل القصيرة (SMS)
* **الموقع:** `modules/auth/service/otp_service.go` (السطر 87)
* **الوصف:**
  ```go
  if err := s.sms.SendSMS(phone, code); err != nil {
      return dto.ErrInvalidOtpCode // خطأ دلالي
  }
  ```
* **الأثر:**
  إذا فشل مزود الـ SMS بسبب انتهاء الرصيد أو مشاكل في الشبكة، سيتلقى العميل الخطأ: *"رمز التحقق غير صحيح أو منتهي الصلاحية"*، في حين أن المستخدم لم يقم بإدخال أي رمز بعد، مما يربك المستخدمين ويصعّب تشخيص الأعطال التشغيلية.
* **الحل المقترح:**
  إرجاع خطأ داخلي صريح مثل `dto.ErrInternalServer` أو إنشاء خطأ مخصص `ErrSmsDeliveryFailed`.

---

### 🟡 DB-01: تجاهل قيد التفرد لرقم الهوية الوطنية في تسجيل المستهلك
* **الموقع:** `modules/auth/service/auth_service.go` (دالة `RegisterConsumer`)
* **الوصف:**
  يتحقق الكود من عدم تكرار الهاتف فقط:
  ```go
  if _, err := s.userRepo.GetUserByPhone(c, req.Phone); err == nil {
      return nil, dto.ErrPhoneAlreadyExists
  }
  ```
  بينما يحتوي نموذج `Consumer` على قيد فريد في قاعدة البيانات:
  `IdNumber: uniqueIndex:idx_consumers_id_number`.
* **الأثر:**
  إذا قام مستخدم بالتسجيل برقم هوية مستخدم مسبقاً، سيفشل الاستعلام على مستوى الـ Database Constraint ويعيد النظام خطأ `500 Internal Server Error` غير معبر، بدلاً من `409 Conflict` توضح أن رقم الهوية مسجل مسبقاً.
* **الحل المقترح:**
  إضافة دالة فحص `GetConsumerByIdNumber` في الـ Repository، وإرجاع كود خطأ مخصص `ErrIdNumberAlreadyExists` (409 Conflict).

---

### 🟡 CODE-01: تعارض الرموز وأخطاء النسخ واللصق في الـ DTOs
* **الموقع:** `modules/auth/dto/auth_dto.go`
* **المشاكل:**
  1. `ErrAccountRejected`: يستخدم `errsCode.ErrAccountDocs` بدلاً من `errsCode.ErrAccountRejected`.
  2. `StatusSuspended`: نص الرسالة مكتوب `"Acount is rejected"` بدلاً من `"account is suspended"`.
  3. `ErrInvalidResetKey`: نص الرسالة مكتوب `"Acount is rejected"` بدلاً من `"invalid or expired reset key"`.
  4. اسم بنية إعادة التعيين مكتوبة بخطأ إملائي: `RestPasswordRequest` وحقل الـ JSON مكتوب `rest_key` بدلاً من `ResetPasswordRequest` و `reset_key`.
* **الحل المقترح:**
  توحيد ومراجعة قاموس الأخطاء في `auth_dto.go` وتصحيح التسميات.

---

## 6. تحسينات الأداء والمعمارية (Quality, Performance & Clean Code)

| النطاق | الملاحظة | الحل المقترح |
| :--- | :--- | :--- |
| **التسجيل (Logging)** | وجود استدعاءات `fmt.Println("Err ...")` في `cf_turnstile_handuler.go` و `print(session.ID())` في `auth_handler.go`. | إزالة الـ prints واستخدام حزمة `log` أو `log/slog` القياسية بتنسيق JSON. |
| **حماية الـ CORS** | النطاق المسموح به ثابت في الكود: `c.Writer.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")`. | نقل عنوان الـ Origin إلى متغير بيئي `ALLOWED_ORIGIN` لدعم بيئات الـ Staging والـ Production. |
| **المصادقة وحالة الرد** | في `middleware/session_handler.go`: إرجاع كود خطأ `404 ErrUserNotFound` عند غياب الجلسة. | يجب إرجاع كود `401 Unauthorized` القياسي لجميع الطلبات غير المصرح لها بدلاً من 404. |
| **التسميات والتنظيم** | اسم الحزمة والمجلد `pkg/respons` يحتوي على خطأ إملائي (`respons` بدلاً من `response`). | إعادة تسمية المجلد إلى `pkg/response` وتحديث المسارات عبر المشروع. |

---

## 7. قائمة التحقق والتوصيات للإنتاج (Production Checklist)

- [ ] **تصحيح ثغرة Turnstile Middleware:** تعيين `http.StatusForbidden` لـ `reqErr.Status`.
- [ ] **تصحيح متغير البيئة:** تعديل قراءة `SESSION_ADDRESS` في `config/config.go`.
- [ ] **تحسين أداء التجزئة:** خفض تكلفة Bcrypt لكلمات المرور إلى `10`، واعتماد خوارزمية سريعة لرموز OTP.
- [ ] **حماية مسار استعادة كلمة المرور:** فرض Cooldown لمدة 60 ثانية لكل رقم هاتف في `ForgetPassword`.
- [ ] **تحسين التعامل مع الأخطاء:** تصحيح رسائل الـ DTOs والـ HTTP Status Codes في تسجيل الهوية والجلسات.
- [ ] **تنظيف الكود:** إزالة عبارات الطباعة المباشرة وضبط مسارات الـ CORS ديناميكياً.

---

**إعداد:** مهندس مراجعة وتدقيق النظم البرمجية  
**الحالة:** بانتظار اعتماد التوصيات وتطبيق التعديلات البرمجية.

