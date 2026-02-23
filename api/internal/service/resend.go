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
	apiKey string
	from   string
	http   *http.Client
}

type DigestEmailCopy struct {
	Subject string
	Body    string
}

func NewResendClient() *ResendClient {
	return &ResendClient{
		apiKey: os.Getenv("RESEND_API_KEY"),
		from:   os.Getenv("RESEND_FROM_EMAIL"),
		http:   &http.Client{Timeout: 15 * time.Second},
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
		"from":    r.from,
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
