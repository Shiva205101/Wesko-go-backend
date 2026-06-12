package catalog

import "errors"

var (
	ErrInvalidCategoryName  = errors.New("category name is required")
	ErrInvalidCategorySlug  = errors.New("category slug is required")
	ErrInvalidProductName   = errors.New("product name is required")
	ErrInvalidProductSlug   = errors.New("product slug is required")
	ErrInvalidProductStatus = errors.New("invalid product status")
	ErrInvalidSKUCode       = errors.New("sku code is required")
	ErrInvalidMoneyAmount   = errors.New("invalid money amount")
	ErrInvalidCurrency      = errors.New("invalid currency")

	ErrCategoryNotFound = errors.New("category not found")
	ErrProductNotFound  = errors.New("product not found")
	ErrSKUNotFound      = errors.New("sku not found")
	ErrJobNotFound      = errors.New("bulk upload job not found")

	ErrCategorySlugAlreadyExists = errors.New("category slug already exists")
	ErrProductSlugAlreadyExists  = errors.New("product slug already exists")
	ErrSKUCodeAlreadyExists      = errors.New("sku code already exists")
)
