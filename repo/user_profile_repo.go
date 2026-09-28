package repo

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

// userProfileKeyPrefix is where the Auth service syncs user profiles to,
// asynchronously — this repo only ever reads that cache, never calls Auth
// directly (no synchronous cross-service call in the request path).
const userProfileKeyPrefix = "user:profile:"

// UserProfile mirrors the JSON the Auth service writes to Redis at
// user:profile:<user_id>.
type UserProfile struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

// UserProfileRepository reads cached user profiles from Redis.
type UserProfileRepository interface {
	// GetProfiles returns whatever profiles are cached for userIDs, keyed
	// by user id. A missing/unparsable profile is simply left out of the
	// map — callers decide the fallback, this layer doesn't invent one.
	GetProfiles(ctx context.Context, userIDs []string) (map[string]UserProfile, error)
}

type userProfileRepository struct {
	rdb *redis.Client
}

// NewUserProfileRepository wires a UserProfileRepository backed by rdb.
func NewUserProfileRepository(rdb *redis.Client) UserProfileRepository {
	return &userProfileRepository{rdb: rdb}
}

// GetProfiles batches one GET per user id into a single Redis pipeline
// round-trip (keys differ per user, so this is the pipelined equivalent of
// an MGET across non-uniform keys).
func (r *userProfileRepository) GetProfiles(ctx context.Context, userIDs []string) (map[string]UserProfile, error) {
	profiles := make(map[string]UserProfile, len(userIDs))
	if len(userIDs) == 0 {
		return profiles, nil
	}

	pipe := r.rdb.Pipeline()
	cmds := make(map[string]*redis.StringCmd, len(userIDs))
	for _, id := range userIDs {
		cmds[id] = pipe.Get(ctx, userProfileKeyPrefix+id)
	}

	// Exec's error reflects the first failing command, which is expected
	// to be redis.Nil whenever any profile simply isn't cached yet — that's
	// not a repo-level failure, so only a non-Nil error aborts the call.
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, err
	}

	for id, cmd := range cmds {
		raw, err := cmd.Result()
		if err != nil {
			continue
		}

		var profile UserProfile
		if err := json.Unmarshal([]byte(raw), &profile); err != nil {
			continue
		}
		profiles[id] = profile
	}

	return profiles, nil
}
