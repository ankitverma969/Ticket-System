package repository

import (
	"errors"
)

var (
	// ErrNotFound is returned when a requested record is not found in the database.
	ErrNotFound = errors.New("record not found")

	// ErrDuplicate is returned when an operation violates a unique constraint (e.g. duplicate email).
	ErrDuplicate = errors.New("record already exists")

	// ErrInvalidID is returned when an ID string or hex is not a valid ObjectID.
	ErrInvalidID = errors.New("invalid identifier")
)
