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

	"github.com/enjoydarts/sifto/api/internal/repository"
	"github.com/enjoydarts/sifto/api/internal/timeutil"
)

const (
	poeUsagePageLimit          = 100
	poeUsageMaxPages           = 20
	poeUsageDefaultRecentLimit = 100
	poeUsageDefaultModelLimit  = 20
)

type PoeUsageService struct {
	http *http.Client
	repo *repository.PoeUsageRepo
}

func NewPoeUsageService(repo *repository.PoeUsageRepo) *PoeUsageService {
	return &PoeUsageService{
		http: &http.Client{Timeout: 20 * time.Second},
		repo: repo,
	}
}

type PoeUsageRange string

const (
	PoeUsageRangeToday       PoeUsageRange = "today"
	PoeUsageRangeYesterday   PoeUsageRange = "yesterday"
	PoeUsageRangeLast7Days   PoeUsageRange = "7d"
	PoeUsageRangeLast14Days  PoeUsageRange = "14d"
	PoeUsageRangeLast30Days  PoeUsageRange = "30d"
	PoeUsageRangeMonthToDate PoeUsageRange = "mtd"
	PoeUsageRangePrevMonth   PoeUsageRange = "prev_month"
)

type PoeUsageOverview struct {
	Configured          bool                        `json:"configured"`
	SelectedRange       string                      `json:"selected_range"`
	RangeStartedAt      *time.Time                  `json:"range_started_at,omitempty"`
	RangeEndedAt        *time.Time                  `json:"range_ended_at,omitempty"`
	CurrentPointBalance *int                        `json:"current_point_balance,omitempty"`
	Summary             PoeUsageSummary             `json:"summary"`
	ModelSummaries      []PoeUsageModelRollup       `json:"model_summaries"`
	Entries             []PoeUsageEntry             `json:"entries"`
	EntryLimit          int                         `json:"entry_limit"`
	AvailableRanges     []PoeUsageRangeOption       `json:"available_ranges"`
	LastSyncRun         *repository.PoeUsageSyncRun `json:"last_sync_run,omitempty"`
}

type PoeUsageRangeOption struct {
	Key string `json:"key"`
}

type PoeUsageSummary struct {
	EntryCount        int        `json:"entry_count"`
	APIEntryCount     int        `json:"api_entry_count"`
	ChatEntryCount    int        `json:"chat_entry_count"`
	TotalCostPoints   int        `json:"total_cost_points"`
	TotalCostUSD      float64    `json:"total_cost_usd"`
	AverageCostPoints float64    `json:"average_cost_points"`
	AverageCostUSD    float64    `json:"average_cost_usd"`
	LatestEntryAt     *time.Time `json:"latest_entry_at,omitempty"`
}

type PoeUsageModelRollup struct {
	BotName           string     `json:"bot_name"`
	EntryCount        int        `json:"entry_count"`
	TotalCostPoints   int        `json:"total_cost_points"`
	TotalCostUSD      float64    `json:"total_cost_usd"`
	AverageCostPoints float64    `json:"average_cost_points"`
	AverageCostUSD    float64    `json:"average_cost_usd"`
	LatestEntryAt     *time.Time `json:"latest_entry_at,omitempty"`
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

func AvailablePoeUsageRanges() []PoeUsageRangeOption {
	return []PoeUsageRangeOption{
		{Key: string(PoeUsageRangeToday)},
		{Key: string(PoeUsageRangeYesterday)},
		{Key: string(PoeUsageRangeLast7Days)},
		{Key: string(PoeUsageRangeLast14Days)},
		{Key: string(PoeUsageRangeLast30Days)},
		{Key: string(PoeUsageRangeMonthToDate)},
		{Key: string(PoeUsageRangePrevMonth)},
	}
}

func NormalizePoeUsageRange(raw string) PoeUsageRange {
	switch PoeUsageRange(strings.TrimSpace(raw)) {
	case PoeUsageRangeToday, PoeUsageRangeYesterday, PoeUsageRangeLast7Days, PoeUsageRangeLast14Days, PoeUsageRangeLast30Days, PoeUsageRangeMonthToDate, PoeUsageRangePrevMonth:
		return PoeUsageRange(strings.TrimSpace(raw))
	default:
		return PoeUsageRangeLast30Days
	}
}

func (s *PoeUsageService) SyncHistory(ctx context.Context, userID, apiKey, syncSource string) (*repository.PoeUsageSyncRun, error) {
	syncRunID, err := s.repo.StartSyncRun(ctx, userID, syncSource)
	if err != nil {
		return nil, err
	}
	entries, _, err := s.fetchPointsHistory(ctx, apiKey)
	if err != nil {
		msg := err.Error()
		_ = s.repo.FinishSyncRun(context.Background(), syncRunID, 0, 0, 0, nil, nil, &msg)
		return nil, err
	}
	records := make([]repository.PoeUsageEntryRecord, 0, len(entries))
	var oldestAt *time.Time
	var latestAt *time.Time
	for _, entry := range entries {
		records = append(records, repository.PoeUsageEntryRecord{
			QueryID:    entry.QueryID,
			BotName:    entry.BotName,
			CreatedAt:  entry.CreatedAt,
			CostUSD:    entry.CostUSD,
			RawCostUSD: entry.RawCostUSD,
			CostPoints: entry.CostPoints,
			Breakdown:  entry.Breakdown,
			UsageType:  entry.UsageType,
			ChatName:   entry.ChatName,
		})
		if latestAt == nil || entry.CreatedAt.After(*latestAt) {
			t := entry.CreatedAt
			latestAt = &t
		}
		if oldestAt == nil || entry.CreatedAt.Before(*oldestAt) {
			t := entry.CreatedAt
			oldestAt = &t
		}
	}
	insertedCount, updatedCount, err := s.repo.UpsertEntries(ctx, userID, syncRunID, records)
	if err != nil {
		msg := err.Error()
		_ = s.repo.FinishSyncRun(context.Background(), syncRunID, len(records), 0, 0, oldestAt, latestAt, &msg)
		return nil, err
	}
	if err := s.repo.FinishSyncRun(ctx, syncRunID, len(records), insertedCount, updatedCount, oldestAt, latestAt, nil); err != nil {
		return nil, err
	}
	return s.repo.GetLatestSyncRun(ctx, userID)
}

func (s *PoeUsageService) GetOverview(ctx context.Context, userID, apiKey string, usageRange PoeUsageRange, recentLimit int) (*PoeUsageOverview, error) {
	balance, err := s.fetchCurrentBalance(ctx, apiKey)
	if err != nil {
		return nil, err
	}
	startedAt, endedAt := poeUsageRangeBounds(usageRange, timeutil.NowJST())
	summaryRow, err := s.repo.SummaryBetween(ctx, userID, startedAt, endedAt)
	if err != nil {
		return nil, err
	}
	modelRows, err := s.repo.ListModelRollupsBetween(ctx, userID, startedAt, endedAt, poeUsageDefaultModelLimit)
	if err != nil {
		return nil, err
	}
	if recentLimit <= 0 {
		recentLimit = poeUsageDefaultRecentLimit
	}
	entryRows, err := s.repo.ListEntriesBetween(ctx, userID, startedAt, endedAt, recentLimit)
	if err != nil {
		return nil, err
	}
	lastSyncRun, err := s.repo.GetLatestSyncRun(ctx, userID)
	if err != nil {
		return nil, err
	}

	summary := PoeUsageSummary{
		EntryCount:      summaryRow.EntryCount,
		APIEntryCount:   summaryRow.APIEntryCount,
		ChatEntryCount:  summaryRow.ChatEntryCount,
		TotalCostPoints: summaryRow.TotalCostPoints,
		TotalCostUSD:    summaryRow.TotalCostUSD,
		LatestEntryAt:   summaryRow.LatestEntryAt,
	}
	if summary.EntryCount > 0 {
		summary.AverageCostPoints = float64(summary.TotalCostPoints) / float64(summary.EntryCount)
		summary.AverageCostUSD = summary.TotalCostUSD / float64(summary.EntryCount)
	}
	models := make([]PoeUsageModelRollup, 0, len(modelRows))
	for _, row := range modelRows {
		model := PoeUsageModelRollup{
			BotName:         row.BotName,
			EntryCount:      row.EntryCount,
			TotalCostPoints: row.TotalCostPoints,
			TotalCostUSD:    row.TotalCostUSD,
			LatestEntryAt:   row.LatestEntryAt,
		}
		if row.EntryCount > 0 {
			model.AverageCostPoints = float64(row.TotalCostPoints) / float64(row.EntryCount)
			model.AverageCostUSD = row.TotalCostUSD / float64(row.EntryCount)
		}
		models = append(models, model)
	}
	entries := make([]PoeUsageEntry, 0, len(entryRows))
	for _, row := range entryRows {
		entries = append(entries, PoeUsageEntry{
			QueryID:    row.QueryID,
			BotName:    row.BotName,
			CreatedAt:  row.CreatedAt,
			CostUSD:    row.CostUSD,
			RawCostUSD: row.RawCostUSD,
			CostPoints: row.CostPoints,
			Breakdown:  row.Breakdown,
			UsageType:  row.UsageType,
			ChatName:   row.ChatName,
		})
	}

	startUTC := startedAt.UTC()
	endUTC := endedAt.UTC()
	return &PoeUsageOverview{
		Configured:          true,
		SelectedRange:       string(usageRange),
		RangeStartedAt:      &startUTC,
		RangeEndedAt:        &endUTC,
		CurrentPointBalance: &balance,
		Summary:             summary,
		ModelSummaries:      models,
		Entries:             entries,
		EntryLimit:          recentLimit,
		AvailableRanges:     AvailablePoeUsageRanges(),
		LastSyncRun:         lastSyncRun,
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
	if summary.EntryCount > 0 {
		summary.AverageCostPoints = float64(summary.TotalCostPoints) / float64(summary.EntryCount)
		summary.AverageCostUSD = summary.TotalCostUSD / float64(summary.EntryCount)
	}
	rollups := make([]PoeUsageModelRollup, 0, len(byModel))
	for _, row := range byModel {
		if row.EntryCount > 0 {
			row.AverageCostPoints = float64(row.TotalCostPoints) / float64(row.EntryCount)
			row.AverageCostUSD = row.TotalCostUSD / float64(row.EntryCount)
		}
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

func poeUsageRangeBounds(usageRange PoeUsageRange, now time.Time) (time.Time, time.Time) {
	today := timeutil.StartOfDayJST(now)
	switch usageRange {
	case PoeUsageRangeToday:
		return today, today.AddDate(0, 0, 1)
	case PoeUsageRangeYesterday:
		start := today.AddDate(0, 0, -1)
		return start, today
	case PoeUsageRangeLast7Days:
		return today.AddDate(0, 0, -6), today.AddDate(0, 0, 1)
	case PoeUsageRangeLast14Days:
		return today.AddDate(0, 0, -13), today.AddDate(0, 0, 1)
	case PoeUsageRangeMonthToDate:
		start := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, timeutil.JST)
		return start, today.AddDate(0, 0, 1)
	case PoeUsageRangePrevMonth:
		thisMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, timeutil.JST)
		prevMonth := thisMonth.AddDate(0, -1, 0)
		return prevMonth, thisMonth
	default:
		return today.AddDate(0, 0, -29), today.AddDate(0, 0, 1)
	}
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
