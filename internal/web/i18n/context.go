package i18n

import (
	"context"

	"github.com/yazmeyaa/hosthalla/tokibundle"
)

type readerContextKey struct{}

func WithReader(ctx context.Context, reader tokibundle.Reader) context.Context {
	return context.WithValue(ctx, readerContextKey{}, reader)
}

func ReaderFromContext(ctx context.Context) tokibundle.Reader {
	if reader, ok := ctx.Value(readerContextKey{}).(tokibundle.Reader); ok {
		return reader
	}

	return tokibundle.Default()
}

func Reader(ctx context.Context) tokibundle.Reader {
	if reader, ok := ctx.Value(readerContextKey{}).(tokibundle.Reader); ok {
		return reader
	}

	return tokibundle.Default()
}
