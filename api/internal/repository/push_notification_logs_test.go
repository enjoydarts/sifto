package repository

import "testing"

func TestNormalizePushNotificationLogInput(t *testing.T) {
	t.Parallel()

	invalid := string([]byte{0xe3, 0x81})
	itemID := "item-" + invalid
	oneSignalID := "onesignal-" + invalid

	got := normalizePushNotificationLogInput(PushNotificationLogInput{
		UserID:                  "user-1",
		Kind:                    "ai_navigator_brief",
		ItemID:                  &itemID,
		Title:                   "朝の通知 " + invalid,
		Message:                 "本文 " + invalid,
		OneSignalNotificationID: &oneSignalID,
		Recipients:              1,
	})

	if got.Title == "朝の通知 "+invalid {
		t.Fatalf("Title was not sanitized: %q", got.Title)
	}
	if got.Message == "本文 "+invalid {
		t.Fatalf("Message was not sanitized: %q", got.Message)
	}
	if got.ItemID == nil || *got.ItemID == "item-"+invalid {
		t.Fatalf("ItemID was not sanitized: %v", got.ItemID)
	}
	if got.OneSignalNotificationID == nil || *got.OneSignalNotificationID == "onesignal-"+invalid {
		t.Fatalf("OneSignalNotificationID was not sanitized: %v", got.OneSignalNotificationID)
	}
}
