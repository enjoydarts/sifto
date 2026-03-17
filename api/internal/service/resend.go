package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/enjoydarts/sifto/api/internal/model"
)

type ResendClient struct {
	apiKey   string
	from     string
	fromName string
	http     *http.Client
}

type DigestEmailCopy struct {
	Subject string
	Body    string
}

type BudgetAlertEmail struct {
	MonthJST           string
	MonthlyBudgetUSD   float64
	UsedCostUSD        float64
	RemainingBudgetUSD float64
	RemainingPct       float64
	ThresholdPct       int
}

type BudgetForecastAlertEmail struct {
	MonthJST         string
	MonthlyBudgetUSD float64
	UsedCostUSD      float64
	ForecastCostUSD  float64
	ForecastDeltaUSD float64
}

type OpenRouterModelAlertEmail struct {
	Added       []string
	Constrained []string
	Removed     []string
	TargetURL   string
}

var digestSubjectPrefixPattern = regexp.MustCompile(`^\s*(?:【[^】]*ダイジェスト】\s*|Sifto\s*Digest\s*[-:]?\s*\d{4}-\d{1,2}-\d{1,2}\s*[-:：]?\s*|Sifto\s*Digest\s*\d{4}-\d{1,2}-\d{1,2}\s*[-:：]?\s*|\d{4}年\d{1,2}月\d{1,2}日ダイジェスト\s*[-:：]?\s*)+`)

func FormatDigestEmailSubject(digestDate string, subject string) string {
	dateText := strings.TrimSpace(digestDate)
	if parsed, err := time.Parse("2006-01-02", dateText); err == nil {
		dateText = fmt.Sprintf("%d年%d月%d日", parsed.Year(), parsed.Month(), parsed.Day())
	}
	prefix := fmt.Sprintf("【%sダイジェスト】", dateText)
	trimmed := strings.TrimSpace(digestSubjectPrefixPattern.ReplaceAllString(strings.TrimSpace(subject), ""))
	if trimmed == "" {
		return prefix
	}
	if strings.HasPrefix(trimmed, prefix) {
		return trimmed
	}
	return prefix + trimmed
}

func NewResendClient() *ResendClient {
	return &ResendClient{
		apiKey:   os.Getenv("RESEND_API_KEY"),
		from:     os.Getenv("RESEND_FROM_EMAIL"),
		fromName: os.Getenv("RESEND_FROM_NAME"),
		http:     &http.Client{Timeout: 15 * time.Second},
	}
}

func (r *ResendClient) Enabled() bool {
	return r != nil && r.apiKey != "" && r.from != ""
}

func (r *ResendClient) SendDigest(ctx context.Context, to string, digest *model.DigestDetail, copy *DigestEmailCopy) error {
	if !r.Enabled() {
		log.Printf("resend disabled (missing RESEND_API_KEY or RESEND_FROM_EMAIL), skip send to %s", to)
		return nil
	}

	subject := FormatDigestEmailSubject(digest.DigestDate, "")
	if copy != nil && strings.TrimSpace(copy.Subject) != "" {
		subject = FormatDigestEmailSubject(digest.DigestDate, copy.Subject)
	}
	html := buildDigestHTML(digest, copy)

	body, _ := json.Marshal(map[string]any{
		"from":    r.formattedFrom(),
		"to":      []string{to},
		"subject": subject,
		"html":    html,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("resend: status %d", resp.StatusCode)
	}
	return nil
}

func (r *ResendClient) SendBudgetAlert(ctx context.Context, to string, alert BudgetAlertEmail) error {
	if !r.Enabled() {
		log.Printf("resend disabled (missing RESEND_API_KEY or RESEND_FROM_EMAIL), skip budget alert to %s", to)
		return nil
	}

	subject := fmt.Sprintf("Sifto: 月次LLM予算の残りが%d%%を下回りました", alert.ThresholdPct)
	htmlBody := buildBudgetAlertHTML(alert)

	body, _ := json.Marshal(map[string]any{
		"from":    r.formattedFrom(),
		"to":      []string{to},
		"subject": subject,
		"html":    htmlBody,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("resend: status %d", resp.StatusCode)
	}
	return nil
}

func (r *ResendClient) SendBudgetForecastAlert(ctx context.Context, to string, alert BudgetForecastAlertEmail) error {
	if !r.Enabled() {
		log.Printf("resend disabled (missing RESEND_API_KEY or RESEND_FROM_EMAIL), skip budget forecast alert to %s", to)
		return nil
	}

	subject := "Sifto: 月次LLM予算の着地予測が予算を超えそうです"
	htmlBody := buildBudgetForecastAlertHTML(alert)

	body, _ := json.Marshal(map[string]any{
		"from":    r.formattedFrom(),
		"to":      []string{to},
		"subject": subject,
		"html":    htmlBody,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("resend: status %d", resp.StatusCode)
	}
	return nil
}

func (r *ResendClient) SendOpenRouterModelAlert(ctx context.Context, to string, alert OpenRouterModelAlertEmail) error {
	if !r.Enabled() {
		log.Printf("resend disabled (missing RESEND_API_KEY or RESEND_FROM_EMAIL), skip openrouter alert to %s", to)
		return nil
	}
	total := len(alert.Added) + len(alert.Constrained) + len(alert.Removed)
	subject := fmt.Sprintf("Sifto: OpenRouter モデル更新 %d 件", total)
	htmlBody := buildOpenRouterModelAlertHTML(alert)
	body, _ := json.Marshal(map[string]any{
		"from":    r.formattedFrom(),
		"to":      []string{to},
		"subject": subject,
		"html":    htmlBody,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("resend: status %d", resp.StatusCode)
	}
	return nil
}

func (r *ResendClient) formattedFrom() string {
	if r == nil {
		return ""
	}
	addr := strings.TrimSpace(r.from)
	if addr == "" {
		return ""
	}
	if strings.Contains(addr, "<") && strings.Contains(addr, ">") {
		return addr
	}
	name := strings.TrimSpace(r.fromName)
	if name == "" {
		name = "Sifto"
	}
	return fmt.Sprintf("%s <%s>", name, addr)
}

func buildDigestHTML(d *model.DigestDetail, copy *DigestEmailCopy) string {
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html><html><body style="font-family:sans-serif;max-width:640px;margin:0 auto;padding:20px">`)
	sb.WriteString(fmt.Sprintf(`<h1 style="font-size:24px;border-bottom:2px solid #eee;padding-bottom:8px">Sifto Digest — %s</h1>`, html.EscapeString(d.DigestDate)))
	if copy != nil && strings.TrimSpace(copy.Body) != "" {
		for _, para := range strings.Split(strings.TrimSpace(copy.Body), "\n\n") {
			p := strings.TrimSpace(para)
			if p == "" {
				continue
			}
			lines := strings.Split(p, "\n")
			if len(lines) > 1 {
				sb.WriteString(`<div style="margin:12px 0 18px;color:#333;line-height:1.6">`)
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line == "" {
						continue
					}
					sb.WriteString(fmt.Sprintf(`<p style="margin:0 0 6px">%s</p>`, html.EscapeString(line)))
				}
				sb.WriteString(`</div>`)
			} else {
				sb.WriteString(fmt.Sprintf(`<p style="margin:12px 0 18px;color:#333;line-height:1.7">%s</p>`, html.EscapeString(p)))
			}
		}
	}

	for _, item := range d.Items {
		title := "（タイトルなし）"
		if item.Item.Title != nil {
			title = *item.Item.Title
		}
		topics := strings.Join(item.Summary.Topics, " · ")
		escapedTopics := html.EscapeString(topics)
		escapedTitle := html.EscapeString(title)
		escapedSummary := html.EscapeString(item.Summary.Summary)
		escapedURL := html.EscapeString(item.Item.URL)

		sb.WriteString(fmt.Sprintf(`
<div style="margin-bottom:24px;padding:16px;border:1px solid #eee;border-radius:8px">
  <p style="margin:0 0 4px;font-size:12px;color:#888">#%d &nbsp;·&nbsp; %s</p>
  <h2 style="margin:0 0 8px;font-size:18px">
    <a href="%s" style="color:#1a1a1a;text-decoration:none">%s</a>
  </h2>
  <p style="margin:0 0 8px;color:#444;line-height:1.6">%s</p>
  <p style="margin:0;font-size:12px;color:#888">%s</p>
</div>`,
			item.Rank, escapedTopics, escapedURL, escapedTitle, escapedSummary, escapedTopics))
	}

	sb.WriteString(`</body></html>`)
	return sb.String()
}

func buildBudgetAlertHTML(a BudgetAlertEmail) string {
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html><html><body style="font-family:sans-serif;max-width:640px;margin:0 auto;padding:20px">`)
	sb.WriteString(`<h1 style="font-size:22px;margin:0 0 12px">Sifto 予算アラート</h1>`)
	sb.WriteString(fmt.Sprintf(`<p style="line-height:1.7;color:#333">%s の月次LLM予算の残りが <strong>%d%%</strong> を下回りました。</p>`,
		html.EscapeString(a.MonthJST), a.ThresholdPct))
	sb.WriteString(`<div style="border:1px solid #e4e4e7;border-radius:10px;padding:14px 16px;background:#fafafa">`)
	sb.WriteString(fmt.Sprintf(`<p style="margin:0 0 6px;color:#444">月次予算: <strong>$%.4f</strong></p>`, a.MonthlyBudgetUSD))
	sb.WriteString(fmt.Sprintf(`<p style="margin:0 0 6px;color:#444">利用額（推定）: <strong>$%.4f</strong></p>`, a.UsedCostUSD))
	sb.WriteString(fmt.Sprintf(`<p style="margin:0 0 6px;color:#444">残額（推定）: <strong>$%.4f</strong></p>`, a.RemainingBudgetUSD))
	sb.WriteString(fmt.Sprintf(`<p style="margin:0;color:#444">残り比率: <strong>%.1f%%</strong></p>`, a.RemainingPct))
	sb.WriteString(`</div>`)
	sb.WriteString(`<p style="margin-top:12px;color:#666;line-height:1.6">設定画面で予算・警告しきい値・Anthropic APIキー（ユーザー別）を管理できます。</p>`)
	sb.WriteString(`</body></html>`)
	return sb.String()
}

func buildBudgetForecastAlertHTML(a BudgetForecastAlertEmail) string {
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html><html><body style="font-family:sans-serif;max-width:640px;margin:0 auto;padding:20px">`)
	sb.WriteString(`<h1 style="font-size:22px;margin:0 0 12px">Sifto 予算着地アラート</h1>`)
	sb.WriteString(fmt.Sprintf(`<p style="color:#444;line-height:1.7">%s の月末着地予測が、設定予算を上回っています。</p>`, html.EscapeString(a.MonthJST)))
	sb.WriteString(`<div style="margin:20px 0;padding:16px;border:1px solid #eee;border-radius:8px;background:#fafafa">`)
	sb.WriteString(fmt.Sprintf(`<p style="margin:0 0 8px"><strong>月次予算:</strong> $%.2f</p>`, a.MonthlyBudgetUSD))
	sb.WriteString(fmt.Sprintf(`<p style="margin:0 0 8px"><strong>今月使用額:</strong> $%.4f</p>`, a.UsedCostUSD))
	sb.WriteString(fmt.Sprintf(`<p style="margin:0 0 8px"><strong>月末着地予測:</strong> $%.4f</p>`, a.ForecastCostUSD))
	sb.WriteString(fmt.Sprintf(`<p style="margin:0"><strong>予算差分:</strong> +$%.4f</p>`, a.ForecastDeltaUSD))
	sb.WriteString(`</div>`)
	sb.WriteString(`<p style="color:#666;line-height:1.7">LLM Usage 画面で直近の利用状況と予測ペースを確認してください。</p>`)
	sb.WriteString(`</body></html>`)
	return sb.String()
}

func buildOpenRouterModelAlertHTML(a OpenRouterModelAlertEmail) string {
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html><html><body style="font-family:sans-serif;max-width:640px;margin:0 auto;padding:20px">`)
	sb.WriteString(`<h1 style="font-size:24px;border-bottom:2px solid #eee;padding-bottom:8px">OpenRouter モデル更新</h1>`)
	sb.WriteString(`<p style="color:#444;line-height:1.7">OpenRouter のモデル状態に変化がありました。</p>`)
	appendList := func(title string, models []string) {
		if len(models) == 0 {
			return
		}
		sb.WriteString(fmt.Sprintf(`<h2 style="font-size:18px;margin-top:20px">%s</h2>`, html.EscapeString(title)))
		sb.WriteString(`<ul style="padding-left:20px;color:#333;line-height:1.7">`)
		for _, modelID := range models {
			sb.WriteString(fmt.Sprintf(`<li>%s</li>`, html.EscapeString(modelID)))
		}
		sb.WriteString(`</ul>`)
	}
	appendList("新規追加", a.Added)
	appendList("制約ありに変更", a.Constrained)
	appendList("削除・非公開", a.Removed)
	if strings.TrimSpace(a.TargetURL) != "" {
		sb.WriteString(fmt.Sprintf(`<p style="margin-top:20px"><a href="%s" style="display:inline-block;background:#18181b;color:#fff;padding:10px 14px;border-radius:8px;text-decoration:none">OpenRouter Models を開く</a></p>`, html.EscapeString(a.TargetURL)))
	}
	sb.WriteString(`</body></html>`)
	return sb.String()
}
