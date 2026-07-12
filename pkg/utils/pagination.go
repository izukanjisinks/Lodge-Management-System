package utils

import (
	"net/http"
	"strconv"
)

type Pagination struct {
	Page     int
	PageSize int
	Offset   int
}

type PaginatedResponse struct {
	Data     interface{} `json:"data"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Total    int         `json:"total"`
}

func ParsePagination(r *http.Request) Pagination {
	page := 1
	pageSize := 10

	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil && v > 0 && v <= 1000 {
			pageSize = v
		}
	}

	return Pagination{
		Page:     page,
		PageSize: pageSize,
		Offset:   (page - 1) * pageSize,
	}
}
