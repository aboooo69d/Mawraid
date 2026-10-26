# تقرير مراجعة مشروع Nova (Go + Gin + Service/Repo)

> تاريخ المراجعة: 2026-10-26
> نتيجة `go build ./...` ✅ نجح | نتيجة `go vet ./...` ✅ نجح
> **لكن:** نجاح البناء لا يعني أن المشروع يعمل. التطبيق حالياً **لا يمكن تشغيله** (دالة `main` فارغة)، وهناك أخطاء منطقية ستسبب فشلاً عند التشغيل الفعلي (أهمها مفاتيح Redis وعدم تعيين الدور عند التسجيل).

---

## 1. ملخص تنفيذي

| الجانب | الحالة |
|---|---|
| هيكلة المجلدات (modules / dto / repo / service / pkg) | ✅ جيدة وواضحة |
| فصل الطبقات (Service ↔ Repo عبر Interfaces) | ✅ صحيح |
| البناء والتحليل الساكن | ✅ |
| طبقة الـ Handlers/Controllers والـ Routes | ❌ غير موجودة |
| `main.go` (تشغيل الخادم، DB، Redis) | ❌ فارغ |
| الاتصال بقاعدة البيانات (driver + migration) | ❌ غير موجود |
| المصادقة (JWT / Session) | ❌ غير موجودة |
| إرسال الـ SMS الفعلي | ❌ غير موصول |
| أخطاء منطقية حرجة | ⚠️ 6 أخطاء (انظر القسم 2) |

---

## 2. أخطاء حرجة (ستكسر التشغيل) 🔴

### 2.1 مفاتيح Redis تستخدم `%d` مع نص (String)
**الملف:** `pkg/constants/redis_keys/redis_keys.go`

```go
OtpCode = "otp:code:%d"
...
func BuildOtpKey(key string, phone string) string {
    return fmt.Sprintf(key, phone)   // phone نص!
}
```
النتيجة: المفتاح يصبح `otp:code:%!d(string=0501234567)`. يعمل «بالصدفة» لأن الصيغة ثابتة، لكنه مفتاح خاطئ ويكسر `go vet` مستقبلاً ويصعّب التتبع.
**الحل:** استبدل `%d` بـ `%s` في كل الثوابت الأربعة.

### 2.2 لا يتم تعيين `RoleID` عند التسجيل
**الملف:** `modules/auth/service/auth_service.go` (`RegisterConsumer` / `RegisterProvider`)

`req.RoleID` يُستقبل في `CreateUserRequest` لكنه **لا يُنسخ** إلى `newUser`، فيُحفظ `RoleID = 0`، وهذا يخالف الـ Foreign Key على جدول `roles` → فشل الإدراج (خطأ 500 دائماً).

**والأسوأ أمنياً:** لو نسخته من الطلب فسيستطيع أي مستخدم اختيار `role_id` لنفسه (مثلاً أدمن) → **ثغرة تصعيد صلاحيات**.
**الحل:** احذف `RoleID` من `CreateUserRequest`، وحدده في الخدمة من الجدول (`consumer` / `provider`) عبر جلب الدور بالاسم من `RoleRepo`، مع عمل Seed للأدوار عند بدء التطبيق.

### 2.3 الحساب لا يصبح `IsActive` أبداً → لا أحد يستطيع تسجيل الدخول
- `IsActive` الافتراضي `false`.
- `VerifyPhone` يضبط `IsVerified = true` فقط.
- `Login` يرفض الحساب إن لم يكن `IsActive`.

النتيجة: بعد التسجيل والتحقق من OTP **لا يمكن تسجيل الدخول** إلا إذا فعّله أدمن يدوياً. إن كان هذا مقصوداً فوثّقه، وإلا فعّل الحساب في `VerifyPhone`.

### 2.4 رمز OTP لا يُرسل أبداً
**الملف:** `auth_service.go` السطر `go s.otpService.SendOtp(...)`
- القيمة المرجعة (الكود) تُهمل، و`smsSender.Provider` غير مستخدم ولا مُحقن في أي خدمة، ودالة `SendSMS` فارغة.
- أي خطأ (cooldown, Redis down) يُبتلع بصمت داخل الـ goroutine.
- `context.Background()` بلا Timeout.

**الحل:** حقن `smsSender.Provider` داخل `OtpService`، وإرسال الرسالة داخل `SendOtp` بعد `Save`، وتسجيل الأخطاء (log). وأضف endpoint لإعادة إرسال الكود.

### 2.5 التحديث بـ `Updates(struct)` يتجاهل القيم الصفرية
**الملف:** `modules/user/repo/user_repo.go` → `UpdateUser`

GORM مع Struct يتجاهل `false` و`0` و`""`. لذلك:
- لا يمكن إيقاف تفعيل مستخدم (`IsActive=false`) أو إلغاء توثيقه.
- تعديل `IsHome=false`، `IsDriver=false`… لن يُحفظ.

كذلك تمرير كائن `User` كامل مُحمّل معه `Role/Provider/Consumer` قد يعيد كتابة الـ associations بشكل غير مقصود.
**الحل:** استخدم `Select("*")`/`Select(حقول محددة)` أو `map[string]any`، ويفضل تحديث `Provider` و`Consumer` عبر repo مستقل أو داخل Transaction.

### 2.6 الحذف الناعم (Soft Delete) يتعارض مع `uniqueIndex`
`TimeStamp` فيه `DeletedAt`، و`Phone` و`IdNumber` عليهما `uniqueIndex`. بعد حذف مستخدم:
- `GetUserByPhone` لا يجده (لأنه محذوف ناعماً) فيسمح بالتسجيل،
- لكن الإدراج يفشل بسبب الـ unique index → خطأ 500.

**الحل:** استخدم Partial Unique Index في PostgreSQL: `uniqueIndex:idx_phone,where:deleted_at IS NULL`.

---

## 3. أجزاء ناقصة لإكمال المشروع 🟠

1. **`cmd/api/main.go` فارغ**: يحتاج تحميل الإعدادات، الاتصال بـ Postgres وRedis، `AutoMigrate`، Seed للأدوار، تهيئة Repo→Service→Handler، تسجيل المسارات، `Run` مع Graceful Shutdown.
2. **لا يوجد Driver لقاعدة البيانات**: `go.mod` يحتوي `gorm` فقط. أضف مثلاً:
   `go get gorm.io/driver/postgres`
   (وقيمة `gen_random_uuid()` تتطلب PostgreSQL 13+).
3. **لا توجد طبقة Handler/Controller ولا Router** (Gin غير مستخدم فعلياً في الكود). هي التي تقوم بـ `ShouldBindJSON` وتحويل `*respons.AppError` إلى استجابة JSON بالـ Status الصحيح.
4. **`config.go` ناقص**: لا يحتوي إعدادات DB، Redis، المنفذ، JWT secret، SMS. وأضف تحققاً من وجود المتغيرات المطلوبة.
5. **لا توجد مصادقة**: `Login` يُرجع بيانات المستخدم فقط بلا Token. أضف JWT (Access + Refresh) أو Session، مع Middleware للتحقق والصلاحيات (RBAC) — جدول `Permission` موجود لكن غير مستخدم.
5. **لا يوجد Middleware** للـ Recovery/Logging/CORS/Rate limiting.
6. **لا توجد اختبارات** (unit tests للـ services بواسطة mocks للـ repo).
7. **لا يوجد `RoleRepo`** ولا خدمة للأدوار والصلاحيات.
8. **`UserService` لا يوجد فيه `CreateUser`** (التسجيل في AuthService) — مقبول، لكن تأكد أنه مقصود.

---

## 4. ملاحظات على الكود (متوسطة) 🟡

### الكيانات (`database/entities`)
- `user_entity.go` سطر 15: **خطأ إملائي** `gorm:"forignKey:RoleID"` ← الصحيح `foreignKey`. (يعمل حالياً بالتخمين التلقائي من GORM لكن الوسم خاطئ.) واجعل `RoleID` بوسم `gorm:"not null"` وأضف `json:"role_id"`.
- `Consumer.IdNumber` له `uniqueIndex` بدون `not null`؛ والـ DTO يجعله `required` لكن DB يسمح بنص فارغ → يسبب تعارضاً لأكثر من مستخدم فارغ. أضف `not null`.
- `FamilyCount *int8` يحدّه 127 كحد أقصى؛ استخدم `*int`/`*uint8` إن لزم.
- `Provider/Consumer` لا يحتويان `User` كعلاقة عكسية ولا FK صريح؛ GORM يتعامل معها لكن الأفضل التصريح بـ `constraint`.

### أخطاء الـ DTO مكررة ومتضاربة
- `ErrPhoneAlreadyExists` معرّف في `modules/user/dto` بـ Status **404** وفي `modules/auth/dto` بـ **409** (الصحيح 409). `UserService` يستخدم النسخة الخاطئة.
- `ErrUserNotFound`, `ErrAcountNotActive`… مكررة في الحزمتين.
- `ErrInvalidOtp` (errsCode 1015) و`ErrInvalidOtpCode` (1008) متشابهان.
- **الحل:** انقل كل أخطاء التطبيق إلى حزمة واحدة (مثلاً `pkg/apperrors`).
- رسائل الأخطاء مختلطة بين الإنجليزية والعربية؛ وحّدها (أو استخدم i18n).

### `auth_service.go`
- `GetUserByPhone` يُعتبر «غير موجود» عند **أي** خطأ؛ لو سقطت قاعدة البيانات يستمر التسجيل. افحص `errors.Is(err, gorm.ErrRecordNotFound)` فقط.
- فحص ثم إدخال (check-then-insert) فيه Race Condition؛ اعتمد على Unique Index وتعامل مع خطأ التكرار (`23505`).
- كود `RegisterConsumer` و`RegisterProvider` متشابه جداً؛ استخرج دالة مشتركة.
- `Login` يكشف إن كان الرقم مسجلاً (`ErrPhoneNotFound` مقابل `ErrInvalidCredentials`) → **User enumeration**. أرجع رسالة موحدة.
- `VerifyPhone` يحوّل **كل** الأخطاء إلى `ErrInvalidOtpCode`، فتضيع أخطاء 429 (تجاوز المحاولات). أعد الخطأ كما هو إن كان `*AppError`.
- لا يوجد Transaction عند إنشاء المستخدم مع الـ Profile (GORM يغلفها تلقائياً في `Create` لكن تأكد).

### `otp_service.go` / `otp_repo.go`
- `bcrypt` بـ cost = 14 (في `value_hashing`) بطيء جداً (~1 ثانية أو أكثر)، ويُستخدم أيضاً لتجزئة OTP؛ استخدم cost=10–12 لكلمات المرور، و`HMAC-SHA256` لرموز OTP.
- `OtpDailyLimit` ينتهي بعد **6 ساعات** وليس 24 رغم الاسم؛ عدّله أو أعد تسميته.
- `Save` غير ذري: `Expire` يُنفذ بعد `Exec` منفصلاً؛ إن فشل يبقى العدّاد دائماً. ضعه في نفس الـ Pipeline أو Lua script/`TxPipeline`.
- `Verify`: `Incr` ثم المقارنة غير ذريين؛ ولا يُحذف الكود عند تجاوز الحد.
- `GetIsCooldown` وبقية الدوال تُرجع «لا قيود» عند خطأ Redis (Fail-open)، أي تُعطّل الحماية عند سقوط Redis. فكّر بإرجاع `error`.
- ثوابت `MaxDailyOtpRequests` وغيرها داخل الـ service؛ الأفضل نقلها للإعدادات.
- ملاحظة: `Save` يمسح `verifyLimitKey` عند كل طلب كود جديد، فيمكن للمهاجم تجديد المحاولات (محدود بالحد اليومي 5×3 = 15 محاولة) — مقبول لكن انتبه.

### `user_service.go`
- `UpdateUser` يسمح بتعديل `IsVerified/IsActive/RoleID` من نفس الطلب؛ **يجب** حصر ذلك بالأدمن (وإلا ثغرة تصعيد صلاحيات). افصل DTO المستخدم العادي عن DTO الأدمن.
- تحديث `Provider/Consumer` يُتجاهل بصمت إن لم يكن موجوداً (`existingUser.Provider != nil`)؛ أرجع خطأ.
- تغيير `Phone` لا يعيد التحقق منه (OTP) ويفحص التكرار بنفس مشكلة 2.6.
- `GetUserById` بمعرّف غير UUID سيعطي خطأ PostgreSQL (500) بدل 400؛ حوّل المعرف إلى `uuid.UUID` وتحقق منه.
- `GetAllUsers` لا تُرجع العدد الكلي (Total) للتصفح، ولا يوجد ترتيب `Order` فيعطي نتائج غير ثابتة بين الصفحات.

### `user_repo.go`
- `GetUserById/ByPhone` يُرجع `&user` حتى مع الخطأ؛ أرجع `nil, err`.
- استخدام `gorm.G[...]` (Generics API) صحيح ويتطلب GORM ≥ 1.30 ✅ (لديك 1.31.2).

### `config.go` و `.env`
- `BACKET_NAME` خطأ إملائي (Bucket) في المتغير؛ صحّحه في `.env` والكود معاً.
- `var Env = NewConfig()` يُنفَّذ عند الاستيراد (side-effect)؛ الأفضل استدعاؤها صراحة من `main` وإرجاع `error`.
- `.env` موجود في `.gitignore` ✅ — أضف ملف `.env.example` بدون قيم سرية.

### `go.mod`
- `go 1.26.8` تأكد أنها نسخة موجودة لديك (البناء نجح، فهي صالحة عندك).
- `gin`, `redis`, `validator` معلّمة `// indirect` رغم أنها ستُستخدم مباشرة → نفّذ `go mod tidy` بعد كتابة الكود.
- حزم غير لازمة حالياً: `aws-sdk-go-v2` (بما فيها dynamodb)، `mongo-driver`، `quic-go`، `gorilla/sessions`، `gin-contrib/sessions`. نظّفها بـ `go mod tidy` أو احذفها إن لم تستخدمها.

### التسمية والأسلوب
- أسماء الحزم يجب أن تكون lowercase بلا underscore/camelCase: `smsSender`, `errsCode`, `redisKeys`, `valueHashing` ← `smssender`, `errscode`, `rediskeys`, `valuehashing`، و`respons` ← `response`.
- `pkg/value_hashing` يُستخدم لـ OTP ولكلمات المرور معاً باسم عام؛ افصلهما.
- `ErrAcountNotActive` ← `ErrAccountNotActive` (خطأ إملائي).
- تعليق في `sms_sender.go`: "this file is now empty for test dont test it" يُحذف قبل الإنتاج.
- `http.Status` بدل الأرقام الصريحة (404/429...) أوضح.

---

## 5. ما هو صحيح ✅

- أكواد الأخطاء في `errsCode` (iota من 1001) متسلسلة ومطابقة لتعليقاتها (1012–1014).
- استخدام `crypto/rand` لتوليد OTP ✅ (آمن).
- تجزئة كلمة المرور بـ bcrypt وعدم إرجاعها في JSON (`json:"-"`) ✅.
- حقن التبعيات عبر Interfaces ممتاز لإمكانية الاختبار ✅.
- قواعد التحقق في DTO (`binding`, `gtfield`) صحيحة، وتحقق `EndAt > StartAt` سليم ✅.
- Soft delete وPagination بحد أقصى 100 ✅.
- `.env` مستثنى من Git ✅.

---

## 6. خطة مقترحة لإكمال المشروع (بالترتيب)

1. إصلاح الأخطاء الحرجة 2.1 → 2.6.
2. `go get gorm.io/driver/postgres` + حزمة `database/connection.go` (Postgres + Redis) + `AutoMigrate` + Seed للأدوار.
3. توسيع `config.go` (DB/Redis/PORT/JWT/SMS) مع تحقق.
4. توحيد الأخطاء في حزمة واحدة + دالة `HandleError(c, err)` في Gin.
5. كتابة Handlers (`auth_handler.go`, `user_handler.go`) وملفات Routes.
6. JWT + Middleware (Auth + Role/Permission).
7. ربط SMS Provider الحقيقي داخل `OtpService`.
8. كتابة `main.go` (تجميع الطبقات + Graceful Shutdown).
9. اختبارات الوحدة للخدمات + اختبار تكامل بسيط.
10. `go mod tidy` ثم `go vet ./...` و`go test ./...`.

### مثال هيكل `main.go` المقترح
```go
func main() {
    cfg := config.Load()
    db := database.NewPostgres(cfg)
    rdb := database.NewRedis(cfg)
    db.AutoMigrate(&entities.Role{}, &entities.Permission{}, &entities.User{}, &entities.Provider{}, &entities.Consumer{})

    userRepo := userrepo.NewUserRepo(db)
    otpSvc   := authservice.NewOtpService(authrepo.NewOtpRepo(rdb), smsProvider)
    authSvc  := authservice.NewAuthService(userRepo, otpSvc)

    r := gin.Default()
    handlers.RegisterRoutes(r, authSvc, userservice.NewUserService(userRepo))
    r.Run(":" + cfg.Port)
}
```
