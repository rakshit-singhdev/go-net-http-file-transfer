package middleware

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "net"
    "net/http"
    "strconv"
    "time"

    "github.com/redis/go-redis/v9"
)

var rateLimitScript = redis.NewScript(`
    local current = redis.call("INCR", KEYS[1])

    if current == 1 then
        redis.call("PEXPIRE", KEYS[1], ARGV[1])
    end

    local ttl = redis.call("PTTL", KEYS[1])

    return {current, ttl}
`)

func RateLimitMiddleware(
    client *redis.Client,
    limit int64,
    window time.Duration,
) func(http.Handler) http.Handler {

    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

            ip, _, err := net.SplitHostPort(r.RemoteAddr)
            if err != nil {
                ip = r.RemoteAddr
            }

            // Hash the IP before using it in the Redis key.
            ipHash := sha256.Sum256([]byte(ip))

            key := fmt.Sprintf(
                "rate-limit:upload:%s",
                hex.EncodeToString(ipHash[:]),
            )

            result, err := rateLimitScript.Run(
                r.Context(),
                client,
                []string{key},
                window.Milliseconds(),
            ).Result()

            if err != nil {
                // Fail open: Redis failure should not block uploads.
                next.ServeHTTP(w, r)
                return
            }

            values, ok := result.([]interface{})
            if !ok || len(values) != 2 {
                next.ServeHTTP(w, r)
                return
            }

            count, err := strconv.ParseInt(
                fmt.Sprint(values[0]),
                10,
                64,
            )
            if err != nil {
                next.ServeHTTP(w, r)
                return
            }

            ttl, err := strconv.ParseInt(
                fmt.Sprint(values[1]),
                10,
                64,
            )
            if err != nil {
                next.ServeHTTP(w, r)
                return
            }

            remaining := limit - count
            if remaining < 0 {
                remaining = 0
            }

            w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(limit, 10))
            w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))

            if count > limit {
                retryAfter := (ttl + 999) / 1000
                if retryAfter < 1 {
                    retryAfter = 1
                }

                w.Header().Set("Retry-After", strconv.FormatInt(retryAfter, 10))

                http.Error(
                    w,
                    "Too many upload requests. Please try again later.",
                    http.StatusTooManyRequests,
                )
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}