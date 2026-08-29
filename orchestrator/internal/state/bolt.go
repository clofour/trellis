package state

import (
	"context"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

var bucketName = []byte("trellis")

type BoltStore struct {
	db *bolt.DB
}

func NewBoltStore(path string) (*BoltStore, error) {
	db, err := bolt.Open(path, 0o600, nil)
	if err != nil {
		return nil, fmt.Errorf("open bolt database: %w", err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketName)
		return err
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create bucket: %w", err)
	}
	return &BoltStore{db: db}, nil
}

func (b *BoltStore) Get(_ context.Context, key string) ([]byte, error) {
	var result []byte
	err := b.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bucketName).Get([]byte(key))
		if v != nil {
			result = make([]byte, len(v))
			copy(result, v)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", key, err)
	}
	return result, nil
}

func (b *BoltStore) List(_ context.Context, prefix string) (map[string][]byte, error) {
	result := make(map[string][]byte)
	err := b.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(bucketName).Cursor()
		p := []byte(prefix)
		for k, v := c.Seek(p); k != nil && len(k) >= len(p) && string(k[:len(p)]) == prefix; k, v = c.Next() {
			cp := make([]byte, len(v))
			copy(cp, v)
			result[string(k)] = cp
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", prefix, err)
	}
	return result, nil
}

func (b *BoltStore) Put(_ context.Context, key string, value []byte) error {
	err := b.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketName).Put([]byte(key), value)
	})
	if err != nil {
		return fmt.Errorf("put %s: %w", key, err)
	}
	return nil
}

func (b *BoltStore) Delete(_ context.Context, key string) error {
	err := b.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketName).Delete([]byte(key))
	})
	if err != nil {
		return fmt.Errorf("delete %s: %w", key, err)
	}
	return nil
}

func (b *BoltStore) Close() error {
	return b.db.Close()
}
