package model

type EventType string

const (
	EventNewBid             EventType = "new_bid"
	EventAuctionEnded       EventType = "auction_ended"
	EventAuctionEndedNoBids EventType = "auction_ended_no_bids"
	EventBidSelected        EventType = "bid_selected"
	EventOrderCreated       EventType = "order_created"
	EventOrderConfirmed     EventType = "order_confirmed"
	EventSelectionExpiring  EventType = "selection_expiring"
	EventAccountApproved    EventType = "account_approved"
	EventAccountRejected    EventType = "account_rejected"

	EventNewSellAuction       EventType = "new_sell_auction"
	EventNewBuyRequest        EventType = "new_buy_request"
	EventNewOffer             EventType = "new_offer"
	EventWinnerSelected       EventType = "winner_selected"
	EventOfferAccepted        EventType = "offer_accepted"
	EventBidNotSelected       EventType = "bid_not_selected"
	EventOfferNotAccepted     EventType = "offer_not_accepted"
	EventRequestEnded         EventType = "request_ended"
	EventRequestEndedNoOffers EventType = "request_ended_no_offers"
	EventSelectionExpired     EventType = "selection_expired"
	EventOrderCompleted       EventType = "order_completed"
	EventOrderExpired         EventType = "order_expired"

	EventPostApproved  EventType = "post_approved"
	EventPostRejected  EventType = "post_rejected"
	EventPostSuspended EventType = "post_suspended"

	// Shipping quotes (#14).
	EventShippingQuoteReceived EventType = "shipping_quote_received"
	EventShippingQuoteAccepted EventType = "shipping_quote_accepted"
	EventShippingQuoteRejected EventType = "shipping_quote_rejected"

	// Platform commission (#13). Issued weekly; the seller settles it with the
	// admin off-platform.
	EventCommissionInvoiceIssued EventType = "commission_invoice_issued"
)

var NotificationMessages = map[EventType]struct{ Title, Body string }{
	EventNewBid:               {"عرض جديد", "عرض جديد على صفقتك"},
	EventAuctionEnded:         {"انتهت الصفقة", "انتهت الصفقة - اختر الفائز خلال 24 ساعة"},
	EventAuctionEndedNoBids:   {"انتهت الصفقة", "انتهت الصفقة بدون عروض"},
	EventBidSelected:          {"مبروك!", "تم اختيارك كفائز!"},
	EventOrderCreated:         {"طلب جديد", "تم إنشاء طلب جديد"},
	EventOrderConfirmed:       {"تأكيد الطلب", "الطرف الآخر أكد الطلب"},
	EventSelectionExpiring:    {"تنبيه", "باقي ساعة لاختيار الفائز"},
	EventAccountApproved:      {"تم التفعيل", "تم تفعيل حسابك"},
	EventAccountRejected:      {"تم الرفض", "تم رفض حسابك"},
	EventNewSellAuction:       {"صفقة جديدة", ""},
	EventNewBuyRequest:        {"طلب شراء جديد", ""},
	EventNewOffer:             {"عرض جديد", "عرض جديد على طلبك"},
	EventWinnerSelected:       {"مبروك!", "تم اختيارك كفائز!"},
	EventOfferAccepted:        {"تم القبول", "تم قبول عرضك"},
	EventBidNotSelected:       {"للأسف لم يتم اختيارك", "تم اختيار عرض آخر على الصفقة"},
	EventOfferNotAccepted:     {"للأسف لم يتم قبول عرضك", "تم قبول عرض آخر على الطلب"},
	EventRequestEnded:         {"انتهى الطلب", "انتهى الطلب - اختر المورد خلال 24 ساعة"},
	EventRequestEndedNoOffers: {"انتهى الطلب", "انتهى الطلب بدون عروض"},
	EventSelectionExpired:     {"انتهت فترة الاختيار", "انتهت مهلة الاختيار وتم إلغاء الصفقة"},
	EventOrderCompleted:       {"اكتمال الطلب", "تم تأكيد الطلب من الطرفين وهو جاهز للتنفيذ"},
	EventOrderExpired:         {"انتهاء مهلة الطلب", "انتهت مهلة تأكيد الطلب (30 دقيقة) وتم إلغاؤه"},
	// Rejection and suspension append the admin's reason to the body.
	EventPostApproved:  {"تم نشر منشورك", "تمت الموافقة على منشورك وهو الآن متاح للجميع"},
	EventPostRejected:  {"لم تتم الموافقة على منشورك", "تم رفض المنشور"},
	EventPostSuspended: {"تم إيقاف منشورك", "تم إيقاف المنشور مؤقتاً"},
	// The merchant is told a price arrived, not what it is: they open the deal to
	// compare it against the others, and a push notification is not private.
	EventShippingQuoteReceived:   {"عرض شحن جديد", "وصلك عرض لشحن صفقتك"},
	EventShippingQuoteAccepted:   {"تم قبول عرض الشحن", "وافق التاجر على عرضك — تفاصيل التواصل متاحة الآن"},
	EventCommissionInvoiceIssued: {"عمولة المنصة", "صدرت فاتورة عمولة الأسبوع — يمكنك مراجعتها من حسابك"},
	EventShippingQuoteRejected:   {"لم يتم اختيار عرضك", "تم اختيار شركة شحن أخرى لهذه الصفقة"},
}
