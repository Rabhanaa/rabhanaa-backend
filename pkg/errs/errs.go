package errs

import "errors"

var (
	ErrInvalidCredentials       = errors.New("INVALID_CREDENTIALS")
	ErrEmailAlreadyExists       = errors.New("EMAIL_ALREADY_EXISTS")
	ErrInvalidPhone             = errors.New("INVALID_PHONE")
	ErrInvalidOTP               = errors.New("INVALID_OTP")
	ErrOTPExpired               = errors.New("OTP_EXPIRED")
	ErrSessionExpired           = errors.New("SESSION_EXPIRED")
	ErrUnauthorized             = errors.New("UNAUTHORIZED")
	ErrAccountNotActive         = errors.New("ACCOUNT_NOT_ACTIVE")
	ErrPendingReview            = errors.New("PENDING_REVIEW")
	ErrInvalidPostType          = errors.New("INVALID_POST_TYPE")
	ErrPostNotModeratable       = errors.New("POST_NOT_MODERATABLE")
	ErrModerationReasonRequired = errors.New("MODERATION_REASON_REQUIRED")
	ErrAdminOnly                = errors.New("ADMIN_ONLY")
	ErrInvalidRegionID          = errors.New("INVALID_REGION_ID")
	ErrInvalidJobID             = errors.New("INVALID_JOB_ID")
	ErrInvalidSignupSource      = errors.New("INVALID_SIGNUP_SOURCE")
)

var (
	ErrAuctionNotFound        = errors.New("AUCTION_NOT_FOUND")
	ErrAuctionNotActive       = errors.New("AUCTION_NOT_ACTIVE")
	ErrAuctionStillActive     = errors.New("AUCTION_STILL_ACTIVE")
	ErrAuctionExpired         = errors.New("AUCTION_EXPIRED")
	ErrCannotBidOwnAuction    = errors.New("CANNOT_BID_OWN")
	ErrAlreadyBid             = errors.New("ALREADY_BID")
	ErrMaxBidders             = errors.New("MAX_BIDDERS")
	ErrMaxActiveBids          = errors.New("MAX_ACTIVE_BIDS")
	ErrBidBelowMinimum        = errors.New("BID_BELOW_MINIMUM")
	ErrCannotCancelWithBids   = errors.New("CANNOT_CANCEL_WITH_BIDS")
	ErrMaxCancellations       = errors.New("MAX_CANCELLATIONS")
	ErrNotAuctionOwner        = errors.New("NOT_AUCTION_OWNER")
	ErrSelectionWindowExpired = errors.New("SELECTION_WINDOW_EXPIRED")
	ErrInvalidQuantity        = errors.New("INVALID_QUANTITY")
)

var (
	ErrOrderNotFound            = errors.New("ORDER_NOT_FOUND")
	ErrNotOrderParticipant      = errors.New("NOT_ORDER_PARTICIPANT")
	ErrAlreadyConfirmed         = errors.New("ALREADY_CONFIRMED")
	ErrOrderConfirmationExpired = errors.New("ORDER_CONFIRMATION_EXPIRED")
	ErrOrderAlreadyExpired      = errors.New("ORDER_ALREADY_EXPIRED")
)

var (
	ErrMaxOpenIssues = errors.New("MAX_OPEN_ISSUES")
	ErrIssueNotFound = errors.New("ISSUE_NOT_FOUND")
)

var (
	ErrInvalidFileType = errors.New("INVALID_FILE_TYPE")
	ErrFileTooLarge    = errors.New("FILE_TOO_LARGE")
)

var (
	ErrUserSuspended            = errors.New("USER_SUSPENDED")
	ErrUserBanned               = errors.New("USER_BANNED")
	ErrInvalidStatusTransition  = errors.New("INVALID_STATUS_TRANSITION")
	ErrCannotActOnAdmin         = errors.New("CANNOT_ACT_ON_ADMIN")
	ErrCannotActOnSelf          = errors.New("CANNOT_ACT_ON_SELF")
	ErrSuspensionReasonRequired = errors.New("SUSPENSION_REASON_REQUIRED")
	ErrBanReasonRequired        = errors.New("BAN_REASON_REQUIRED")
)

var (
	ErrNoSubscription            = errors.New("NO_SUBSCRIPTION")
	ErrInsufficientTier          = errors.New("INSUFFICIENT_TIER")
	ErrMonthlyLimit              = errors.New("MONTHLY_LIMIT_REACHED")
	ErrStorageUnavailable        = errors.New("STORAGE_UNAVAILABLE")
	ErrSubscriptionAlreadyExists = errors.New("SUBSCRIPTION_ALREADY_EXISTS")
	ErrSubscriptionNotFound      = errors.New("SUBSCRIPTION_NOT_FOUND")
	ErrInvalidTier               = errors.New("INVALID_TIER")
)

var ArabicMessages = map[string]string{
	"INVALID_CREDENTIALS":         "البريد الإلكتروني أو كلمة المرور غير صحيحة",
	"EMAIL_ALREADY_EXISTS":        "البريد الإلكتروني مستخدم بالفعل",
	"INVALID_PHONE":               "رقم الهاتف غير صحيح",
	"INVALID_OTP":                 "رمز التحقق غير صحيح",
	"OTP_EXPIRED":                 "انتهت صلاحية رمز التحقق",
	"SESSION_EXPIRED":             "تم تسجيل الدخول من جهاز آخر",
	"UNAUTHORIZED":                "غير مصرح",
	"ACCOUNT_NOT_ACTIVE":          "حسابك غير نشط",
	"PENDING_REVIEW":              "حسابك قيد المراجعة - يمكنك تصفح الصفقات فقط",
	"ADMIN_ONLY":                  "هذا الإجراء للمسؤولين فقط",
	"INVALID_REGION_ID":           "المنطقة المحددة غير موجودة",
	"INVALID_JOB_ID":              "المهنة المحددة غير موجودة",
	"INVALID_SIGNUP_SOURCE":       "مصدر التسجيل غير صحيح",
	"AUCTION_NOT_FOUND":           "الصفقة غير موجودة",
	"INVALID_POST_TYPE":           "نوع المنشور غير صحيح",
	"POST_NOT_MODERATABLE":        "المنشور غير موجود أو لا يمكن تنفيذ هذا الإجراء عليه",
	"MODERATION_REASON_REQUIRED":  "يجب كتابة السبب",
	"AUCTION_NOT_ACTIVE":          "الصفقة غير نشطة",
	"AUCTION_STILL_ACTIVE":        "الصفقة لا تزال نشطة - يمكنك اختيار الفائز بعد انتهاء الصفقة",
	"AUCTION_EXPIRED":             "الصفقة منتهية",
	"CANNOT_BID_OWN":              "لا يمكنك تقديم عرض على صفقتك",
	"ALREADY_BID":                 "لقد قدمت عرض بالفعل",
	"MAX_BIDDERS":                 "تم الوصول للحد الأقصى من المزايدين",
	"MAX_ACTIVE_BIDS":             "لديك 3 عروض نشطة بالفعل",
	"BID_BELOW_MINIMUM":           "السعر غير عادل",
	"CANNOT_CANCEL_WITH_BIDS":     "لا يمكن إلغاء الصفقة بعد وجود عروض",
	"MAX_CANCELLATIONS":           "وصلت للحد الأقصى للإلغاء (3 شهرياً)",
	"NOT_AUCTION_OWNER":           "أنت لست صاحب الصفقة",
	"SELECTION_WINDOW_EXPIRED":    "انتهت فترة الاختيار",
	"INVALID_QUANTITY":            "الكمية المطلوبة غير صحيحة",
	"ORDER_NOT_FOUND":             "الطلب غير موجود",
	"NOT_ORDER_PARTICIPANT":       "أنت لست طرف في هذا الطلب",
	"ALREADY_CONFIRMED":           "تم التأكيد بالفعل",
	"ORDER_CONFIRMATION_EXPIRED":  "انتهت مهلة تأكيد الطلب (30 دقيقة)",
	"ORDER_ALREADY_EXPIRED":       "الطلب منتهي الصلاحية ومُلغى",
	"MAX_OPEN_ISSUES":             "لديك 3 استفسارات مفتوحة بالفعل",
	"INVALID_FILE_TYPE":           "نوع الملف غير مدعوم",
	"FILE_TOO_LARGE":              "حجم الملف كبير جداً",
	"NO_SUBSCRIPTION":             "ليس لديك اشتراك نشط - تواصل معنا للترقية",
	"INSUFFICIENT_TIER":           "اشتراكك الحالي لا يسمح بهذا الإجراء - تواصل معنا للترقية",
	"MONTHLY_LIMIT_REACHED":       "وصلت للحد الشهري المسموح به - تواصل معنا للترقية",
	"STORAGE_UNAVAILABLE":         "الخدمة التخزينية غير متاحة",
	"SUBSCRIPTION_ALREADY_EXISTS": "الاشتراك موجود بالفعل",
	"SUBSCRIPTION_NOT_FOUND":      "الاشتراك غير موجود",
	"INVALID_TIER":                "الباقة غير صحيحة",
	"USER_SUSPENDED":              "تم تعليق حسابك مؤقتاً",
	"USER_BANNED":                 "تم حظر حسابك",
	"INVALID_STATUS_TRANSITION":   "لا يمكن تنفيذ هذا الإجراء على حالة الحساب الحالية",
	"CANNOT_ACT_ON_ADMIN":         "لا يمكن تنفيذ هذا الإجراء على حساب مدير",
	"CANNOT_ACT_ON_SELF":          "لا يمكن تنفيذ هذا الإجراء على حسابك",
	"SUSPENSION_REASON_REQUIRED":  "سبب التعليق مطلوب",
	"BAN_REASON_REQUIRED":         "سبب الحظر مطلوب",
}

func GetArabicMessage(err error) string {
	if msg, ok := ArabicMessages[err.Error()]; ok {
		return msg
	}
	return "حدث خطأ غير متوقع"
}
