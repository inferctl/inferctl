package envelope

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strconv"
	"time"

	"github.com/oklog/ulid/v2"
)

const ContractVersion = "0.1"

type Options struct {
	StartedAt      time.Time
	FinishedAt     time.Time
	Env            map[string]string
	SearchMode     *string
	FallbackTier   *string
	FallbackReason *string
	Warnings       []Warning
	Commands       []Command
	Errors         []Error
}

func NewMeta(data any, opts Options) (Meta, error) {
	env := opts.Env
	if env == nil {
		env = environ()
	}

	dataHash, err := DataHash(data)
	if err != nil {
		return Meta{}, err
	}

	if deterministic(env) {
		ts := "1970-01-01T00:00:00.000Z"
		if raw := env["SOURCE_DATE_EPOCH"]; raw != "" {
			sec, err := strconv.ParseInt(raw, 10, 64)
			if err == nil {
				ts = time.Unix(sec, 0).UTC().Format("2006-01-02T15:04:05.000Z")
			}
		}
		return Meta{
			RequestID:       "req_01TEST00000000000000000000",
			TSISO:           ts,
			ContractVersion: ContractVersion,
			ElapsedMS:       0,
			DataHash:        &dataHash,
			SearchMode:      opts.SearchMode,
			FallbackTier:    opts.FallbackTier,
			FallbackReason:  opts.FallbackReason,
		}, nil
	}

	started := opts.StartedAt
	if started.IsZero() {
		started = time.Now()
	}
	finished := opts.FinishedAt
	if finished.IsZero() {
		finished = time.Now()
	}

	return Meta{
		RequestID:       requestID(started),
		TSISO:           started.UTC().Format("2006-01-02T15:04:05.000Z"),
		ContractVersion: ContractVersion,
		ElapsedMS:       finished.Sub(started).Milliseconds(),
		DataHash:        &dataHash,
		SearchMode:      opts.SearchMode,
		FallbackTier:    opts.FallbackTier,
		FallbackReason:  opts.FallbackReason,
	}, nil
}

func DataHash(data any) (string, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func deterministic(env map[string]string) bool {
	return env["INFERCTL_TEST_DETERMINISTIC"] == "1"
}

func requestID(t time.Time) string {
	entropy := ulid.Monotonic(rand.Reader, 0)
	return "req_" + ulid.MustNew(ulid.Timestamp(t), entropy).String()
}

func environ() map[string]string {
	out := map[string]string{}
	for _, kv := range os.Environ() {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				out[kv[:i]] = kv[i+1:]
				break
			}
		}
	}
	return out
}
