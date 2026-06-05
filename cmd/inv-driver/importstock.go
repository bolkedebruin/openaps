package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/bolkedebruin/openaps/internal/importstock"
	"github.com/bolkedebruin/openaps/internal/store"
)

const defaultStockDB = "/home/database.db"

// runImportStock seeds inv-driver's inverter inventory from the stock
// APsystems SQLite database so a mid-day brownfield migration can poll the
// already-paired fleet immediately, instead of waiting for the next dawn
// re-announce.
func runImportStock(args []string) error {
	fs := flag.NewFlagSet("import-stock", flag.ContinueOnError)
	stockDB := fs.String("stock-db", defaultStockDB, "stock APsystems SQLite database to read")
	dbPath := fs.String("db", defaultDB, "inv-driver SQLite state-store path")
	dryRun := fs.Bool("dry-run", false, "report what would be imported without writing")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx := context.Background()

	stock, err := importstock.OpenStockDB(ctx, *stockDB)
	if err != nil {
		return err
	}
	defer stock.Close()

	rows, err := importstock.ReadStockRows(ctx, stock)
	if err != nil {
		return err
	}

	if *dryRun {
		res, err := importstock.Import(ctx, nil, rows, time.Now().UnixMilli(), true)
		if err != nil {
			return err
		}
		fmt.Printf("dry-run: %d would be imported (%d unbound, no short_addr yet; %d with out-of-range short_address imported as unbound), %d skipped (empty serial), %d skipped (invalid serial)\n",
			res.Imported, res.Unbound, res.Invalid, res.Skipped, res.InvalidSerial)
		return nil
	}

	st, err := store.Open(ctx, *dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	res, err := importstock.Import(ctx, st, rows, time.Now().UnixMilli(), false)
	if err != nil {
		return err
	}
	fmt.Printf("imported %d inverter(s) (%d new, %d updated, %d unbound with no short_addr; %d with out-of-range short_address imported as unbound), %d skipped (empty serial), %d skipped (invalid serial)\n",
		res.Imported, res.Inserted, res.Updated, res.Unbound, res.Invalid, res.Skipped, res.InvalidSerial)
	return nil
}
