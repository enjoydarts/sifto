package service

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"net"
	"strconv"
	"testing"
	"time"
)

type cacheTestRedis struct {
	data     map[string]string
	commands []string
}

func (h *cacheTestRedis) DialHook(next redis.DialHook) redis.DialHook {
	return func(context.Context, string, string) (net.Conn, error) {
		return nil, fmt.Errorf("network should not be used")
	}
}
func (h *cacheTestRedis) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
func (h *cacheTestRedis) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		h.commands = append(h.commands, cmd.Name())
		key := fmt.Sprint(cmd.Args()[1])
		switch cmd.Name() {
		case "get":
			value, ok := h.data[key]
			if !ok {
				cmd.SetErr(redis.Nil)
				return redis.Nil
			}
			cmd.(*redis.StringCmd).SetVal(value)
		case "set":
			h.data[key] = string(cmd.Args()[2].([]byte))
			cmd.(*redis.StatusCmd).SetVal("OK")
		case "incr":
			value, _ := strconv.ParseInt(h.data[key], 10, 64)
			value++
			h.data[key] = strconv.FormatInt(value, 10)
			cmd.(*redis.IntCmd).SetVal(value)
		default:
			return fmt.Errorf("unexpected command %s", cmd.Name())
		}
		return nil
	}
}

func TestUserCacheInvalidationNeverScansAndIsolatesUsers(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "unused:6379"})
	defer client.Close()
	hook := &cacheTestRedis{data: make(map[string]string)}
	client.AddHook(hook)
	cache := &RedisJSONCache{client: client, prefix: "test"}
	ctx := context.Background()
	for _, user := range []string{"a", "b"} {
		if err := cache.SetJSON(ctx, "v1:items:reading-plan:"+user+":size=10", map[string]string{"user": user}, time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := cache.DeleteByPrefix(ctx, "v1:items:reading-plan:a:", 5000); err != nil {
		t.Fatal(err)
	}
	var value map[string]string
	if found, err := cache.GetJSON(ctx, "v1:items:reading-plan:a:size=10", &value); err != nil || found {
		t.Fatalf("invalidated data found=%v err=%v", found, err)
	}
	if found, err := cache.GetJSON(ctx, "v1:items:reading-plan:b:size=10", &value); err != nil || !found || value["user"] != "b" {
		t.Fatalf("other user affected found=%v err=%v", found, err)
	}
	for _, command := range hook.commands {
		if command == "scan" {
			t.Fatal("invalidation scanned Redis")
		}
	}
}
