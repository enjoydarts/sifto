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
	"strings"
	"time"

	"github.com/minoru-kitayama/sifto/api/internal/model"
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

	subject := fmt.Sprintf("Sifto Digest - %s", digest.DigestDate)
	if copy != nil && strings.TrimSpace(copy.Subject) != "" {
		subject = copy.Subject
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
