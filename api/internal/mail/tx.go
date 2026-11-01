package mail

import (
	"context"
	"database/sql"
)

type txKey struct{}

// WithTx carries the transaction an outbox worker already holds, so a
// Sender that reads the database reads through it rather than take a
// second pool connection (#2432).
func WithTx(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// TxFrom returns the transaction WithTx put on ctx.
func TxFrom(ctx context.Context) (*sql.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(*sql.Tx)
	return tx, ok
}
