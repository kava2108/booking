package repository

import "errors"

var (
	// ErrNotFound は対象の行が存在しないことを表す（pgx.ErrNoRows を変換したもの）。
	ErrNotFound = errors.New("repository: not found")
	// ErrEmailTaken は email の一意制約違反を表す。
	ErrEmailTaken = errors.New("repository: email already taken")
)
