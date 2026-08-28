package client

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// insertClient writes rec as a new clients row under practiceID. rec.ID
// must already be set (the handler generates it in Go, not via
// RETURNING id, matching this repo's existing convention).
func insertClient(ctx context.Context, tx *sql.Tx, practiceID string, rec Record) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO clients (
			id, practice_id, given_name, family_name, preferred_name, email, phone,
			address_line1, address_line2, address_locality, address_region, address_postal_code,
			date_of_birth, field_values
		 ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NULLIF($13, '')::date, $14)`,
		rec.ID, practiceID, rec.GivenName, nullIfEmpty(rec.FamilyName), nullIfEmpty(rec.PreferredName),
		nullIfEmpty(rec.Email), nullIfEmpty(rec.Phone),
		nullIfEmpty(rec.AddressLine1), nullIfEmpty(rec.AddressLine2), nullIfEmpty(rec.AddressLocality),
		nullIfEmpty(rec.AddressRegion), nullIfEmpty(rec.AddressPostalCode),
		rec.DateOfBirth, []byte(rec.FieldValues),
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("client: insert client: %w", err)
	}
	return nil
}

// updateClient replaces every structural column and field_values on
// clientID's row -- a full-object PUT, the same "object is the whole
// state" convention contracts.PutContractHandler uses for Values.
func updateClient(ctx context.Context, tx *sql.Tx, clientID string, rec Record) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE clients SET
			given_name = $2, family_name = $3, preferred_name = $4, email = $5, phone = $6,
			address_line1 = $7, address_line2 = $8, address_locality = $9, address_region = $10,
			address_postal_code = $11, date_of_birth = NULLIF($12, '')::date, field_values = $13
		 WHERE id = $1`,
		clientID, rec.GivenName, nullIfEmpty(rec.FamilyName), nullIfEmpty(rec.PreferredName),
		nullIfEmpty(rec.Email), nullIfEmpty(rec.Phone),
		nullIfEmpty(rec.AddressLine1), nullIfEmpty(rec.AddressLine2), nullIfEmpty(rec.AddressLocality),
		nullIfEmpty(rec.AddressRegion), nullIfEmpty(rec.AddressPostalCode),
		rec.DateOfBirth, []byte(rec.FieldValues),
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("client: update client: %w", err)
	}
	return nil
}

// fetchRecord reads clientID's full Record, scoped to practiceID (RLS
// already confines it, but this is the app-layer's own filter, the same
// belt-and-suspenders engagement.listClientEngagements uses). Reports
// sql.ErrNoRows, wrapped, if no such row exists at this Practice.
func fetchRecord(ctx context.Context, tx *sql.Tx, practiceID, clientID string) (Record, error) {
	rec, err := scanRecord(tx.QueryRowContext(ctx,
		`SELECT `+recordColumns+` FROM clients WHERE id = $1 AND practice_id = $2`,
		clientID, practiceID,
	).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, fmt.Errorf("client: fetch record: %w", sql.ErrNoRows)
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return Record{}, fmt.Errorf("client: fetch record: %w", err)
	}
	return rec, nil
}
