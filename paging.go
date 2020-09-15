package turtleware

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

// Paging is a simple holder for applying offsets and limits.
type Paging struct {
	Offset uint32
	Limit  uint16
}

var (
	// ErrInvalidOffset indicates that the query contained an invalid
	// offset parameter (e.g. non numeric).
	ErrInvalidOffset = errors.New("invalid offset parameter")

	// ErrInvalidLimit indicates that the query contained an invalid
	// limit parameter (e.g. non numeric).
	ErrInvalidLimit = errors.New("invalid limit parameter")
)

// ParsePagingFromRequest parses paging information from a given
// request.
func ParsePagingFromRequest(r *http.Request) (Paging, error) {
	query := r.URL.Query()

	var (
		offset uint32
		limit  uint16
	)

	offsetString := query.Get("offset")
	if offsetString != "" {
		val, err := strconv.ParseUint(offsetString, 10, 32)
		if err != nil {
			return Paging{}, ErrInvalidOffset
		}

		offset = uint32(val)
	} else {
		offset = 0
	}

	limitString := query.Get("limit")
	if limitString != "" {
		val, err := strconv.ParseUint(limitString, 10, 16)
		if err != nil {
			return Paging{}, ErrInvalidLimit
		}

		limit = uint16(val)

		if limit > 500 {
			limit = 500
		}
	} else {
		limit = 100
	}

	return Paging{
		Offset: offset,
		Limit:  limit,
	}, nil
}

// String provides a simple way of stringifying paging information for
// requests.
func (paging Paging) String() string {
	if paging.Offset > 0 {
		return fmt.Sprintf("offset=%d&limit=%d", paging.Offset, paging.Limit)
	}

	return fmt.Sprintf("limit=%d", paging.Limit)
}
