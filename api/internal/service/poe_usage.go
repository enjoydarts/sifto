package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	poeUsagePageLimit   = 100
	poeUsageMaxPages    = 20
	poeUsageDefaultRecentLimit = 100
)

type PoeUsageService struct {
	http *http.Client
}

func NewPoeUsageService() *PoeUsageService {
	return &PoeUsageService{
		http: &http.Client{Timeout: 20 * time.Second},
	}
}

type PoeUsageOverview struct {
	Configured          bool                  `json:"configured"`
	CurrentPointBalance *int                  `json:"current_point_balance,omitempty"`
	Summary             PoeUsageSummary       `json:"summary"`
	ModelSummaries      []PoeUsageModelRollup `json:"model_summaries"`
	Entries             []PoeUsageEntry       `json:"entries"`
	Truncated           bool                  `json:"truncated"`
}

type PoeUsageSummary struct {
	EntryCount      int        `json:"entry_count"`
	APIEntryCount   int        `json:"api_entry_count"`
	ChatEntryCount  int        `json:"chat_entry_count"`
	TotalCostPoints int        `json:"total_cost_points"`
	TotalCostUSD    float64    `json:"total_cost_usd"`
	LatestEntryAt   *time.Time `json:"latest_entry_at,omitempty"`
}

type PoeUsageModelRollup struct {
	BotName         string     `json:"bot_name"`
	EntryCount      int        `json:"entry_count"`
	TotalCostPoints int        `json:"total_cost_points"`
	TotalCostUSD    float64    `json:"total_cost_usd"`
	LatestEntryAt   *time.Time `json:"latest_entry_at,omitempty"`
}

type PoeUsageEntry struct {
	QueryID    string            `json:"query_id"`
	BotName    string            `json:"bot_name"`
	CreatedAt  time.Time         `json:"created_at"`
	CostUSD    float64           `json:"cost_usd"`
	RawCostUSD string            `json:"raw_cost_usd"`
	CostPoints int               `json:"cost_points"`
	Breakdown  map[string]string `json:"cost_breakdown_in_points"`
	UsageType  string            `json:"usage_type"`
	ChatName   string            `json:"chat_name,omitempty"`
}

type poeCurrentBalanceResponse struct {
	CurrentPointBalance int `json:"current_point_balance"`
}

type poePointsHistoryResponse struct {
	HasMore bool                  `json:"has_more"`
	Length  int                   `json:"length"`
	Data    []poePointsHistoryRow `json:"data"`
}

type poePointsHistoryRow struct {
	BotName    string            `json:"bot_name"`
	Creation   int64             `json:"creation_time"`
	QueryID    string            `json:"query_id"`
	CostUSD    string            `json:"cost_usd"`
	CostPoints int               `json:"cost_points"`
	Breakdown  map[string]string `json:"cost_breakdown_in_points"`
	UsageType  string            `json:"usage_type"`
	ChatName   string            `json:"chat_name"`
}

func (s *PoeUsageService) FetchOverview(ctx context.Context, apiKey string, recentLimit int) (*PoeUsageOverview, error) {
	balance, err := s.fetchCurrentBalance(ctx, apiKey)
	if err != nil {
		return nil, err
	}
	entries, truncated, err := s.fetchPointsHistory(ctx, apiKey)
	if err != nil {
		return nil, err
	}
	summary, rollups := summarizePoeUsage(entries)
	if recentLimit <= 0 {
		recentLimit = poeUsageDefaultRecentLimit
	}
	recent := entries
	if len(recent) > recentLimit {
		recent = recent[:recentLimit]
	}
	return &PoeUsageOverview{
		Configured:          true,
		CurrentPointBalance: &balance,
		Summary:             summary,
		ModelSummaries:      rollups,
		Entries:             recent,
		Truncated:           truncated,
	}, nil
}

func (s *PoeUsageService) fetchCurrentBalance(ctx context.Context, apiKey string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, poeCurrentBalanceURL(), nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	resp, err := s.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return 0, fmt.Errorf("poe current balance api status %d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload poeCurrentBalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, err
	}
	return payload.CurrentPointBalance, nil
}

func (s *PoeUsageService) fetchPointsHistory(ctx context.Context, apiKey string) ([]PoeUsageEntry, bool, error) {
	out := make([]PoeUsageEntry, 0, poeUsagePageLimit)
	startingAfter := ""
	cutoff := time.Now().UTC().Add(-30 * 24 * time.Hour)
	truncated := false

	for page := 0; page < poeUsageMaxPages; page++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, poePointsHistoryURL(startingAfter), nil)
		if err != nil {
			return nil, false, err
		}
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
		resp, err := s.http.Do(req)
		if err != nil {
			return nil, false, err
		}
		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			resp.Body.Close()
			return nil, false, fmt.Errorf("poe points history api status %d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		var payload poePointsHistoryResponse
		err = json.NewDecoder(resp.Body).Decode(&payload)
		resp.Body.Close()
		if err != nil {
			return nil, false, err
		}
		if len(payload.Data) == 0 {
			return out, false, nil
		}

		oldestAt := time.Now().UTC()
		for _, row := range payload.Data {
			entry := normalizePoeUsageEntry(row)
			out = append(out, entry)
			oldestAt = entry.CreatedAt
		}
		if !payload.HasMore {
			return out, false, nil
		}
		if oldestAt.Before(cutoff) {
			return out, false, nil
		}
		startingAfter = strings.TrimSpace(payload.Data[len(payload.Data)-1].QueryID)
		if startingAfter == "" {
			return out, false, nil
		}
	}

	truncated = true
	return out, truncated, nil
}

func normalizePoeUsageEntry(row poePointsHistoryRow) PoeUsageEntry {
	costUSD, _ := strconv.ParseFloat(strings.TrimSpace(row.CostUSD), 64)
	return PoeUsageEntry{
		QueryID:    strings.TrimSpace(row.QueryID),
		BotName:    strings.TrimSpace(row.BotName),
		CreatedAt:  normalizePoeUsageTimestamp(row.Creation),
		CostUSD:    costUSD,
		RawCostUSD: strings.TrimSpace(row.CostUSD),
		CostPoints: row.CostPoints,
		Breakdown:  row.Breakdown,
		UsageType:  strings.TrimSpace(row.UsageType),
		ChatName:   strings.TrimSpace(row.ChatName),
	}
}

func normalizePoeUsageTimestamp(v int64) time.Time {
	switch {
	case v >= 1_000_000_000_000_000:
		return time.UnixMicro(v).UTC()
	case v >= 1_000_000_000_000:
		return time.UnixMilli(v).UTC()
	default:
		return time.Unix(v, 0).UTC()
	}
}

func summarizePoeUsage(entries []PoeUsageEntry) (PoeUsageSummary, []PoeUsageModelRollup) {
	summary := PoeUsageSummary{}
	byModel := make(map[string]*PoeUsageModelRollup)
	for _, entry := range entries {
		summary.EntryCount++
		summary.TotalCostPoints += entry.CostPoints
		summary.TotalCostUSD += entry.CostUSD
		if strings.EqualFold(entry.UsageType, "chat") {
			summary.ChatEntryCount++
		} else {
			summary.APIEntryCount++
		}
		if summary.LatestEntryAt == nil || entry.CreatedAt.After(*summary.LatestEntryAt) {
			t := entry.CreatedAt
			summary.LatestEntryAt = &t
		}

		key := entry.BotName
		if key == "" {
			key = "Unknown"
		}
		row, ok := byModel[key]
		if !ok {
			row = &PoeUsageModelRollup{BotName: key}
			byModel[key] = row
		}
		row.EntryCount++
		row.TotalCostPoints += entry.CostPoints
		row.TotalCostUSD += entry.CostUSD
		if row.LatestEntryAt == nil || entry.CreatedAt.After(*row.LatestEntryAt) {
			t := entry.CreatedAt
			row.LatestEntryAt = &t
		}
	}

	rollups := make([]PoeUsageModelRollup, 0, len(byModel))
	for _, row := range byModel {
		rollups = append(rollups, *row)
	}
	sort.Slice(rollups, func(i, j int) bool {
		if rollups[i].TotalCostPoints == rollups[j].TotalCostPoints {
			if rollups[i].TotalCostUSD == rollups[j].TotalCostUSD {
				return rollups[i].BotName < rollups[j].BotName
			}
			return rollups[i].TotalCostUSD > rollups[j].TotalCostUSD
		}
		return rollups[i].TotalCostPoints > rollups[j].TotalCostPoints
	})
	return summary, rollups
}

func poeCurrentBalanceURL() string {
	return "https://api.poe.com/usage/current_balance"
}

func poePointsHistoryURL(startingAfter string) string {
	u, _ := url.Parse("https://api.poe.com/usage/points_history")
	q := u.Query()
	q.Set("limit", strconv.Itoa(poeUsagePageLimit))
	if strings.TrimSpace(startingAfter) != "" {
		q.Set("starting_after", strings.TrimSpace(startingAfter))
	}
	u.RawQuery = q.Encode()
	return u.String()
}
