package code

import "context"

type Rule interface {
	GetPrefix() string
	GetPattern() string
	GetDigit() int
}

type Process interface {
	GenerateCode(ctx context.Context, rule Rule) string
}
