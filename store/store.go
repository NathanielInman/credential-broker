package store

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"go.etcd.io/bbolt"
)

// Bucket names used throughout the application.
const (
	BucketUsers     = "users"
	BucketScopes    = "scopes"
	BucketSessions  = "sessions"
	BucketTwoFactor = "twofactor"
	BucketConfig    = "config"
)

// allBuckets is the list of buckets created on initialization.
var allBuckets = []string{
	BucketUsers,
	BucketScopes,
	BucketSessions,
	BucketTwoFactor,
	BucketConfig,
}

// entry is the internal storage format for each value.
type entry struct {
	Value  string `json:"value"`  // base64-encoded value
	Expiry int64  `json:"expiry"` // unix ms timestamp, 0 means no expiry
}

// Store wraps bbolt with TTL support.
type Store struct {
	db *bbolt.DB
}

// New opens (or creates) a bbolt database at the given path and ensures all
// required buckets exist.
func New(path string) (*Store, error) {
	db, err := bbolt.Open(path, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		for _, name := range allBuckets {
			if _, err := tx.CreateBucketIfNotExists([]byte(name)); err != nil {
				return fmt.Errorf("create bucket %q: %w", name, err)
			}
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize buckets: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// Set stores a value in a bucket. A ttl of 0 means no expiry.
func (s *Store) Set(bucket, key string, value []byte, ttl time.Duration) error {
	e := entry{
		Value: base64.StdEncoding.EncodeToString(value),
	}
	if ttl > 0 {
		e.Expiry = time.Now().Add(ttl).UnixMilli()
	}

	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal entry: %w", err)
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket %q not found", bucket)
		}
		return b.Put([]byte(key), data)
	})
}

// Get retrieves a value. Returns nil, nil if the key is not found or has expired.
func (s *Store) Get(bucket, key string) ([]byte, error) {
	var result []byte

	err := s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket %q not found", bucket)
		}

		raw := b.Get([]byte(key))
		if raw == nil {
			return nil
		}

		var e entry
		if err := json.Unmarshal(raw, &e); err != nil {
			return fmt.Errorf("unmarshal entry: %w", err)
		}

		if e.Expiry != 0 && time.Now().UnixMilli() > e.Expiry {
			return b.Delete([]byte(key))
		}

		decoded, err := base64.StdEncoding.DecodeString(e.Value)
		if err != nil {
			return fmt.Errorf("decode value: %w", err)
		}
		result = decoded
		return nil
	})

	return result, err
}

// Delete removes a key from a bucket.
func (s *Store) Delete(bucket, key string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket %q not found", bucket)
		}
		return b.Delete([]byte(key))
	})
}

// GetAll returns all non-expired key-value pairs in a bucket.
func (s *Store) GetAll(bucket string) (map[string][]byte, error) {
	result := make(map[string][]byte)
	var expired []string

	err := s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket %q not found", bucket)
		}

		now := time.Now().UnixMilli()

		if err := b.ForEach(func(k, v []byte) error {
			var e entry
			if err := json.Unmarshal(v, &e); err != nil {
				return fmt.Errorf("unmarshal entry for key %q: %w", string(k), err)
			}

			if e.Expiry != 0 && now > e.Expiry {
				expired = append(expired, string(k))
				return nil
			}

			decoded, err := base64.StdEncoding.DecodeString(e.Value)
			if err != nil {
				return fmt.Errorf("decode value for key %q: %w", string(k), err)
			}
			result[string(k)] = decoded
			return nil
		}); err != nil {
			return err
		}

		for _, key := range expired {
			if err := b.Delete([]byte(key)); err != nil {
				return fmt.Errorf("delete expired key %q: %w", key, err)
			}
		}

		return nil
	})

	return result, err
}

// Keys returns all non-expired keys in a bucket.
func (s *Store) Keys(bucket string) ([]string, error) {
	var keys []string
	var expired []string

	err := s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket %q not found", bucket)
		}

		now := time.Now().UnixMilli()

		if err := b.ForEach(func(k, v []byte) error {
			var e entry
			if err := json.Unmarshal(v, &e); err != nil {
				return fmt.Errorf("unmarshal entry for key %q: %w", string(k), err)
			}

			if e.Expiry != 0 && now > e.Expiry {
				expired = append(expired, string(k))
				return nil
			}

			keys = append(keys, string(k))
			return nil
		}); err != nil {
			return err
		}

		for _, key := range expired {
			if err := b.Delete([]byte(key)); err != nil {
				return fmt.Errorf("delete expired key %q: %w", key, err)
			}
		}

		return nil
	})

	return keys, err
}

// Cleanup removes all expired entries across all buckets.
func (s *Store) Cleanup() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		now := time.Now().UnixMilli()

		for _, name := range allBuckets {
			b := tx.Bucket([]byte(name))
			if b == nil {
				continue
			}

			var expired [][]byte

			if err := b.ForEach(func(k, v []byte) error {
				var e entry
				if err := json.Unmarshal(v, &e); err != nil {
					return nil // skip malformed entries
				}
				if e.Expiry != 0 && now > e.Expiry {
					expired = append(expired, append([]byte{}, k...))
				}
				return nil
			}); err != nil {
				return fmt.Errorf("iterate bucket %q: %w", name, err)
			}

			for _, k := range expired {
				if err := b.Delete(k); err != nil {
					return fmt.Errorf("delete expired key in bucket %q: %w", name, err)
				}
			}
		}

		return nil
	})
}

// StartCleanup runs Cleanup every interval in a background goroutine.
// It stops when the database is closed.
func (s *Store) StartCleanup(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			if err := s.Cleanup(); err != nil {
				return
			}
		}
	}()
}
