package db

import "context"

// Placeholder helper transaksi jika dibutuhkan di masa depan.
type TxFunc func(ctx context.Context) error
