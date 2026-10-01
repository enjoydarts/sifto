package service

import "testing"

func TestD1ShadowEventKeepsCandidateSnapshotAndDeduplicatesReplays(t *testing.T) {
	data := D1ShadowEventData{ItemID: "item", UserID: "user", TriggerID: "run-1", Kind: "facts", Attempt: 1, Title: "title", Content: "source", Facts: []string{"candidate"}}
	first := NewD1ShadowEvent(data)
	replay := NewD1ShadowEvent(data)
	if first.Name != "item/d1-shadow.requested" || first.ID == nil || *first.ID != *replay.ID || first.Data.Content != "source" || first.Data.Attempt != 1 {
		t.Fatalf("event = %#v, replay = %#v", first, replay)
	}
	data.Facts[0] = "next candidate"
	if first.Data.Facts[0] != "candidate" || *NewD1ShadowEvent(data).ID == *first.ID {
		t.Fatal("candidate changed or different candidate deduplicated")
	}
	data = first.Data
	data.TriggerID = "run-2"
	if *NewD1ShadowEvent(data).ID == *first.ID {
		t.Fatal("new processing run deduplicated")
	}
}
