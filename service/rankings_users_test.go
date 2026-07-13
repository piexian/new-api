package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
)

func TestBuildRankedUsersUsesStableUserID(t *testing.T) {
	current := []model.RankingUserQuotaTotal{
		{UserID: 1, Username: "alice-new", TotalTokens: 200, TotalQuota: 20, RequestCount: 5},
		{UserID: 2, Username: "bob", TotalTokens: 100, TotalQuota: 10, RequestCount: 2},
		{Username: "invalid", TotalTokens: 999, TotalQuota: 9, RequestCount: 9},
	}
	previous := []model.RankingUserQuotaTotal{
		{UserID: 2, Username: "bob", TotalTokens: 50},
		{UserID: 1, Username: "alice-old", TotalTokens: 100},
	}

	rows := buildRankedUsers(current, previous, true)
	if len(rows) != 2 {
		t.Fatalf("expected 2 users, got %d", len(rows))
	}
	if rows[0].Username != "alice-new" || rows[0].Rank != 1 || rows[0].userID != 1 {
		t.Fatalf("unexpected top row: %+v", rows[0])
	}
	if rows[0].PreviousRank == nil || *rows[0].PreviousRank != 2 {
		t.Fatalf("user 1 previous rank want 2, got %+v", rows[0].PreviousRank)
	}
	if rows[0].GrowthPct != 100 {
		t.Fatalf("user 1 growth want 100, got %v", rows[0].GrowthPct)
	}
	if rows[0].Share < 0.66 || rows[0].Share > 0.67 {
		t.Fatalf("user 1 share unexpected: %v", rows[0].Share)
	}
	if rows[1].Username != "bob" {
		t.Fatalf("second row want bob, got %+v", rows[1])
	}

	payload, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "user_id") {
		t.Fatalf("user ID must not be serialized: %s", payload)
	}
}

func TestLimitRankedUsers(t *testing.T) {
	rows := make([]RankedUser, 5)
	for i := range rows {
		rows[i] = RankedUser{Rank: i + 1, Username: "u"}
	}
	got := limitRankedUsers(rows, 3)
	if len(got) != 3 {
		t.Fatalf("want 3, got %d", len(got))
	}
	got = limitRankedUsers(rows, 0)
	if len(got) != 5 {
		t.Fatalf("limit 0 should keep all, got %d", len(got))
	}
}

func TestPublicRankingsSnapshotHidesUsers(t *testing.T) {
	resp := &RankingsResponse{
		Users:    []RankedUser{{userID: 1, Rank: 1, Username: "alice", TotalTokens: 10}},
		allUsers: []RankedUser{{userID: 1, Rank: 1, Username: "alice", TotalTokens: 10}},
	}
	out := PublicRankingsSnapshot(resp)
	if out.Users != nil {
		t.Fatalf("guest should not get users, got %+v", out.Users)
	}
	if out.Me != nil {
		t.Fatalf("guest should not get Me, got %+v", out.Me)
	}
	if len(resp.Users) != 1 {
		t.Fatalf("cached response mutated: %+v", resp.Users)
	}
}

func TestAttachRankingsViewerUsesCachedSnapshot(t *testing.T) {
	resp := &RankingsResponse{
		Users: []RankedUser{{userID: 1, Rank: 1, Username: "alice", TotalTokens: 100}},
		allUsers: []RankedUser{
			{userID: 1, Rank: 1, Username: "alice", TotalTokens: 100},
			{userID: 2, Rank: 2, Username: "bob-current", TotalTokens: 50, Share: 0.3333, GrowthPct: 10},
		},
	}
	out := AttachRankingsViewer(resp, 2, "stale-session-name")
	if out.Me == nil {
		t.Fatal("expected Me for user 2")
	}
	if out.Me.Username != "bob-current" || out.Me.Rank != 2 || out.Me.InTopList {
		t.Fatalf("unexpected Me: %+v", out.Me)
	}
	if out.Me.TotalUsers != 2 {
		t.Fatalf("total users want 2, got %d", out.Me.TotalUsers)
	}
}
