package mail_test

import (
	"context"
	"database/sql"
	"testing"

	"doula-cloud/api/internal/mail"
)

func TestTxFrom_ReturnsTheTransactionWithTxCarried(t *testing.T) {
	if _, ok := mail.TxFrom(context.Background()); ok {
		t.Fatal("TxFrom found a transaction on a bare context")
	}
	tx := &sql.Tx{}
	got, ok := mail.TxFrom(mail.WithTx(context.Background(), tx))
	if !ok || got != tx {
		t.Fatalf("TxFrom = %p, %v; want %p, true", got, ok, tx)
	}
}
