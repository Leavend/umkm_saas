package pgconn

type CommandTag struct{}

func (c CommandTag) RowsAffected() int64 { return 0 }
