package email

import (
	"fmt"
	"html/template"
	"strings"
)

// Inline styles and a table-free layout: mail clients strip <style> blocks and
// Gmail in particular ignores most of what a browser would honour.
var passwordResetTemplate = template.Must(template.New("password_reset").Parse(`
<!DOCTYPE html>
<html dir="rtl" lang="ar">
  <body style="margin:0;padding:24px;background:#f6f7f9;font-family:Tahoma,Arial,sans-serif;">
    <div style="max-width:480px;margin:0 auto;background:#ffffff;border-radius:16px;padding:32px;text-align:center;">
      <h1 style="margin:0 0 8px;font-size:22px;color:#111827;">إعادة تعيين كلمة المرور</h1>
      <p style="margin:0 0 24px;font-size:14px;color:#6b7280;line-height:1.7;">
        استخدم الرمز التالي لإعادة تعيين كلمة المرور الخاصة بحسابك في ربحانة.
      </p>

      <div style="margin:0 0 24px;padding:16px;background:#f0fdf4;border:1px solid #bbf7d0;border-radius:12px;">
        <span style="font-size:34px;font-weight:bold;letter-spacing:10px;color:#16a34a;direction:ltr;display:inline-block;">{{.Code}}</span>
      </div>

      <p style="margin:0 0 8px;font-size:13px;color:#6b7280;">
        الرمز صالح لمدة {{.TTLMinutes}} دقيقة.
      </p>
      <p style="margin:0;font-size:12px;color:#9ca3af;line-height:1.7;">
        إذا لم تطلب إعادة تعيين كلمة المرور، تجاهل هذه الرسالة ولن يتغير أي شيء في حسابك.
      </p>

      <div style="margin-top:28px;padding-top:16px;border-top:1px solid #f3f4f6;font-size:12px;color:#9ca3af;">
        ربحانة — مع ربحانة دايما ربحانة
      </div>
    </div>
  </body>
</html>`))

// PasswordResetEmail renders the reset code message and its subject.
func PasswordResetEmail(code string, ttlMinutes int) (subject, html string, err error) {
	var b strings.Builder
	err = passwordResetTemplate.Execute(&b, struct {
		Code       string
		TTLMinutes int
	}{Code: code, TTLMinutes: ttlMinutes})
	if err != nil {
		return "", "", fmt.Errorf("email: render password reset: %w", err)
	}
	return "رمز إعادة تعيين كلمة المرور - ربحانة", b.String(), nil
}
