// Package clientkey is the crypto-shredding mechanism ADR-0027 asks for
// (docs/adr/0027-erasure-redacts-in-place-and-shreds-the-key.md): one
// random 256-bit key per Client, used to seal the personal data that
// would otherwise sit in plaintext inside `activity`, an append-only
// table erasure is forbidden to UPDATE or DELETE.
//
// Erasure destroys the key rather than the rows. What survives is the
// shape of the history -- that something happened, when, and who did it,
// all of which live in plaintext columns; what does not survive is the
// diff, which is the only part that ever held her name, address or date
// of birth.
//
// The package knows nothing about what it seals. It takes bytes and
// returns bytes, so `activity` stays ignorant of Clients and `client`
// stays ignorant of ciphers.
package clientkey

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

// keyLength is AES-256's key size, restated here because
// client_data_keys carries the same number as a CHECK constraint and the
// two must not drift.
const keyLength = 32

// sealedVersion is the "v" every Sealed carries. It exists so a later
// change of cipher can be told apart from this one on read, without
// having to guess from the ciphertext's shape.
const sealedVersion = 1

// ErrNoKey is what Open reports when the Client's key row is gone --
// the ordinary post-erasure state, not a failure. A reader that gets
// this renders the entry as unreadable rather than failing the request:
// the row is still a true record that something happened.
var ErrNoKey = errors.New("clientkey: no key for that client")

// Sealed is the on-disk shape of an encrypted diff: still valid jsonb,
// so activity.diff stays one jsonb column with no schema change, and
// self-describing enough that a person reading the table by hand can see
// what they are looking at.
type Sealed struct {
	Version int    `json:"v"`
	Enc     string `json:"enc"`
}

// IsSealed reports whether raw is a Sealed envelope rather than a
// plaintext diff. The read path needs this because rows written before
// #394 are plaintext and stay that way -- activity is append-only, so
// there is no migration that could have converted them.
func IsSealed(raw json.RawMessage) bool {
	var s Sealed
	if err := json.Unmarshal(raw, &s); err != nil {
		return false
	}
	return s.Version == sealedVersion && s.Enc != ""
}

// Ensure makes clientID's key if she has none, and is a no-op if she
// already has one. Called in the same transaction that inserts the
// Client, so a Client and her key either both exist or neither does --
// and again at every write site that seals, so a Client who predates
// #394 gets one the first time something is written about her.
//
// It will not remake a key for an erased Client. That is the one case
// where "she has no key" is a decision rather than an omission, and
// quietly undoing it would make everything written after the erasure
// readable again -- so the INSERT selects through clients and finds
// nothing when erased_at is set. A caller that seals afterwards gets
// ErrNoKey, which is the correct refusal.
func Ensure(ctx context.Context, tx *sql.Tx, practiceID, clientID string) error {
	key := make([]byte, keyLength)
	if _, err := rand.Read(key); err != nil {
		// coverage:ignore reason: crypto/rand.Read never fails on a supported platform, not exercised by unit tests
		return fmt.Errorf("clientkey: generate key: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO client_data_keys (client_id, practice_id, key)
		 SELECT $1, $2, $3 FROM clients WHERE id = $1 AND erased_at IS NULL
		 ON CONFLICT (client_id) DO NOTHING`,
		clientID, practiceID, key,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("clientkey: ensure key: %w", err)
	}
	return nil
}

// Destroy deletes clientID's key. This is the erasure -- after it, every
// diff sealed under that key is permanently unreadable, and not one row
// of activity was touched to make it so. Reports no error when there is
// no key to destroy, so a retry is a no-op rather than a failure.
func Destroy(ctx context.Context, tx *sql.Tx, clientID string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM client_data_keys WHERE client_id = $1`, clientID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("clientkey: destroy key: %w", err)
	}
	return nil
}

// Seal encrypts plaintext under clientID's key and returns the jsonb
// envelope to store. Reports ErrNoKey if the Client has no key -- which
// after erasure is the correct answer: nothing further should be written
// about a Client whose key is gone, and the caller refuses rather than
// falling back to plaintext.
func Seal(ctx context.Context, tx *sql.Tx, clientID string, plaintext []byte) (json.RawMessage, error) {
	key, err := fetchKey(ctx, tx, clientID)
	if err != nil {
		return nil, err
	}
	gcm, err := newGCM(key)
	if err != nil {
		// coverage:ignore reason: a 32-byte key always yields a valid AES-GCM, not exercised by unit tests
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		// coverage:ignore reason: crypto/rand.Read never fails on a supported platform, not exercised by unit tests
		return nil, fmt.Errorf("clientkey: generate nonce: %w", err)
	}
	// The nonce is prepended to the ciphertext rather than stored beside
	// it: one opaque string is one column, and Open knows where to cut.
	sealed := gcm.Seal(nonce, nonce, plaintext, nil)
	envelope, err := json.Marshal(Sealed{Version: sealedVersion, Enc: base64.StdEncoding.EncodeToString(sealed)})
	if err != nil {
		// coverage:ignore reason: a struct of int and string always marshals cleanly, not exercised by unit tests
		return nil, fmt.Errorf("clientkey: marshal sealed: %w", err)
	}
	return envelope, nil
}

// Open decrypts an envelope Seal produced, back to the plaintext diff.
// Reports ErrNoKey once the key has been destroyed -- the whole point of
// the design, and the case every reader has to render rather than treat
// as an error.
func Open(ctx context.Context, tx *sql.Tx, clientID string, envelope json.RawMessage) ([]byte, error) {
	var s Sealed
	if err := json.Unmarshal(envelope, &s); err != nil || s.Version != sealedVersion {
		return nil, fmt.Errorf("clientkey: unrecognized sealed envelope")
	}
	raw, err := base64.StdEncoding.DecodeString(s.Enc)
	if err != nil {
		return nil, fmt.Errorf("clientkey: decode sealed envelope: %w", err)
	}
	key, err := fetchKey(ctx, tx, clientID)
	if err != nil {
		return nil, err
	}
	gcm, err := newGCM(key)
	if err != nil {
		// coverage:ignore reason: a 32-byte key always yields a valid AES-GCM, not exercised by unit tests
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, fmt.Errorf("clientkey: sealed envelope too short")
	}
	plaintext, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return nil, fmt.Errorf("clientkey: open sealed envelope: %w", err)
	}
	return plaintext, nil
}

// fetchKey reads clientID's key, reporting ErrNoKey when the row is
// absent. RLS confines it to the caller's own Practice, so a key is
// never readable across a tenancy boundary.
func fetchKey(ctx context.Context, tx *sql.Tx, clientID string) ([]byte, error) {
	var key []byte
	err := tx.QueryRowContext(ctx, `SELECT key FROM client_data_keys WHERE client_id = $1`, clientID).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoKey
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("clientkey: fetch key: %w", err)
	}
	return key, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		// coverage:ignore reason: a 32-byte key is always a valid AES key, not exercised by unit tests
		return nil, fmt.Errorf("clientkey: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		// coverage:ignore reason: AES always has a GCM-compatible block size, not exercised by unit tests
		return nil, fmt.Errorf("clientkey: new gcm: %w", err)
	}
	return gcm, nil
}
