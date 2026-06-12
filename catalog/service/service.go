package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"vesko/catalog"
)

type Service struct {
	repo   catalog.Repository
	logger *slog.Logger
}

func New(repo catalog.Repository, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}

	return &Service{
		repo:   repo,
		logger: logger.With("component", "catalog_service"),
	}
}

func (s *Service) CreateCategory(ctx context.Context, req catalog.CreateCategoryRequest) (catalog.Category, error) {
	name := strings.TrimSpace(req.Name)
	slug := normalizeSlug(req.Slug)
	if name == "" {
		return catalog.Category{}, catalog.ErrInvalidCategoryName
	}
	if slug == "" {
		return catalog.Category{}, catalog.ErrInvalidCategorySlug
	}

	category := catalog.Category{
		Name:        name,
		Slug:        slug,
		Description: strings.TrimSpace(req.Description),
		ParentID:    req.ParentID,
		IsActive:    true,
		SortOrder:   0,
	}
	if req.IsActive != nil {
		category.IsActive = *req.IsActive
	}
	if req.SortOrder != nil {
		category.SortOrder = *req.SortOrder
	}

	return s.repo.CreateCategory(ctx, category)
}

func (s *Service) GetCategory(ctx context.Context, id uint) (catalog.Category, error) {
	if id == 0 {
		return catalog.Category{}, catalog.ErrCategoryNotFound
	}

	return s.repo.GetCategoryByID(ctx, id)
}

func (s *Service) UpdateCategory(ctx context.Context, id uint, req catalog.UpdateCategoryRequest) (catalog.Category, error) {
	if id == 0 {
		return catalog.Category{}, catalog.ErrCategoryNotFound
	}

	current, err := s.repo.GetCategoryByID(ctx, id)
	if err != nil {
		return catalog.Category{}, err
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return catalog.Category{}, catalog.ErrInvalidCategoryName
		}
		current.Name = name
	}

	if req.Slug != nil {
		slug := normalizeSlug(*req.Slug)
		if slug == "" {
			return catalog.Category{}, catalog.ErrInvalidCategorySlug
		}
		current.Slug = slug
	}

	if req.Description != nil {
		current.Description = strings.TrimSpace(*req.Description)
	}
	if req.ParentID != nil {
		current.ParentID = req.ParentID
	}
	if req.IsActive != nil {
		current.IsActive = *req.IsActive
	}
	if req.SortOrder != nil {
		current.SortOrder = *req.SortOrder
	}

	return s.repo.UpdateCategory(ctx, current)
}

func (s *Service) DeleteCategory(ctx context.Context, id uint) error {
	if id == 0 {
		return catalog.ErrCategoryNotFound
	}
	return s.repo.DeleteCategory(ctx, id)
}

func (s *Service) ListCategories(ctx context.Context, req catalog.ListCategoriesRequest) ([]catalog.Category, int64, error) {
	filter := catalog.CategoryFilter{
		ParentID: req.ParentID,
		IsActive: req.IsActive,
		Search:   strings.TrimSpace(req.Search),
		Page:     req.Page,
		PageSize: req.PageSize,
	}

	return s.repo.ListCategories(ctx, filter)
}

func (s *Service) CreateProduct(ctx context.Context, req catalog.CreateProductRequest) (catalog.Product, error) {
	name := strings.TrimSpace(req.Name)
	slug := normalizeSlug(req.Slug)
	if name == "" {
		return catalog.Product{}, catalog.ErrInvalidProductName
	}
	if slug == "" {
		return catalog.Product{}, catalog.ErrInvalidProductSlug
	}
	if req.CategoryID == 0 {
		return catalog.Product{}, catalog.ErrCategoryNotFound
	}

	status := req.Status
	if status == "" {
		status = catalog.ProductStatusDraft
	}
	if !isValidStatus(status) {
		return catalog.Product{}, catalog.ErrInvalidProductStatus
	}

	if _, err := s.repo.GetCategoryByID(ctx, req.CategoryID); err != nil {
		return catalog.Product{}, err
	}

	product := catalog.Product{
		CategoryID:      req.CategoryID,
		Name:            name,
		Slug:            slug,
		Description:     strings.TrimSpace(req.Description),
		Status:          status,
		FabricSummary:   strings.TrimSpace(req.FabricSummary),
		WashCare:        strings.TrimSpace(req.WashCare),
		DurabilityNotes: strings.TrimSpace(req.DurabilityNotes),
		Images:          mapImageRequests(req.Images, 0),
		SKUs:            mapSKURequests(req.SKUs, 0),
		Attributes:      mapAttributeRequests(req.Attributes, 0),
	}

	if err := validateSKUs(product.SKUs); err != nil {
		return catalog.Product{}, err
	}

	return s.repo.CreateProduct(ctx, product)
}

func (s *Service) GetProduct(ctx context.Context, id uint) (catalog.Product, error) {
	if id == 0 {
		return catalog.Product{}, catalog.ErrProductNotFound
	}
	return s.repo.GetProductByID(ctx, id)
}

func (s *Service) UpdateProduct(ctx context.Context, id uint, req catalog.UpdateProductRequest) (catalog.Product, error) {
	if id == 0 {
		return catalog.Product{}, catalog.ErrProductNotFound
	}

	current, err := s.repo.GetProductByID(ctx, id)
	if err != nil {
		return catalog.Product{}, err
	}

	if req.CategoryID != nil {
		if *req.CategoryID == 0 {
			return catalog.Product{}, catalog.ErrCategoryNotFound
		}
		if _, err := s.repo.GetCategoryByID(ctx, *req.CategoryID); err != nil {
			return catalog.Product{}, err
		}
		current.CategoryID = *req.CategoryID
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return catalog.Product{}, catalog.ErrInvalidProductName
		}
		current.Name = name
	}
	if req.Slug != nil {
		slug := normalizeSlug(*req.Slug)
		if slug == "" {
			return catalog.Product{}, catalog.ErrInvalidProductSlug
		}
		current.Slug = slug
	}
	if req.Description != nil {
		current.Description = strings.TrimSpace(*req.Description)
	}
	if req.Status != nil {
		if !isValidStatus(*req.Status) {
			return catalog.Product{}, catalog.ErrInvalidProductStatus
		}
		current.Status = *req.Status
	}
	if req.FabricSummary != nil {
		current.FabricSummary = strings.TrimSpace(*req.FabricSummary)
	}
	if req.WashCare != nil {
		current.WashCare = strings.TrimSpace(*req.WashCare)
	}
	if req.DurabilityNotes != nil {
		current.DurabilityNotes = strings.TrimSpace(*req.DurabilityNotes)
	}
	if req.Images != nil {
		current.Images = mapImageRequests(*req.Images, current.ID)
	}
	if req.SKUs != nil {
		current.SKUs = mapSKURequests(*req.SKUs, current.ID)
	}
	if req.Attributes != nil {
		current.Attributes = mapAttributeRequests(*req.Attributes, current.ID)
	}

	if err := validateSKUs(current.SKUs); err != nil {
		return catalog.Product{}, err
	}

	return s.repo.UpdateProduct(ctx, current)
}

func (s *Service) DeleteProduct(ctx context.Context, id uint) error {
	if id == 0 {
		return catalog.ErrProductNotFound
	}
	return s.repo.DeleteProduct(ctx, id)
}

func (s *Service) ListProducts(ctx context.Context, req catalog.ListProductsRequest) ([]catalog.Product, int64, error) {
	filter := catalog.ProductFilter{
		CategoryID: req.CategoryID,
		Size:       strings.TrimSpace(req.Size),
		Color:      strings.TrimSpace(req.Color),
		MinPrice:   req.MinPrice,
		MaxPrice:   req.MaxPrice,
		Search:     strings.TrimSpace(req.Search),
		Sort:       strings.TrimSpace(req.Sort),
		Page:       req.Page,
		PageSize:   req.PageSize,
	}

	if req.Status != "" {
		status := catalog.ProductStatus(strings.TrimSpace(req.Status))
		if !isValidStatus(status) {
			return nil, 0, catalog.ErrInvalidProductStatus
		}
		filter.Status = &status
	}

	return s.repo.ListProducts(ctx, filter)
}

func normalizeSlug(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func mapImageRequests(requests []catalog.CreateProductImageRequest, productID uint) []catalog.ProductImage {
	out := make([]catalog.ProductImage, 0, len(requests))
	for idx, image := range requests {
		sortOrder := idx
		if image.SortOrder != nil {
			sortOrder = *image.SortOrder
		}
		isPrimary := idx == 0
		if image.IsPrimary != nil {
			isPrimary = *image.IsPrimary
		}

		out = append(out, catalog.ProductImage{
			ProductID: productID,
			URL:       strings.TrimSpace(image.URL),
			AltText:   strings.TrimSpace(image.AltText),
			SortOrder: sortOrder,
			IsPrimary: isPrimary,
		})
	}

	return out
}

func mapSKURequests(requests []catalog.CreateProductSKURequest, productID uint) []catalog.ProductSKU {
	out := make([]catalog.ProductSKU, 0, len(requests))
	for _, sku := range requests {
		currency := strings.ToUpper(strings.TrimSpace(sku.Currency))
		if currency == "" {
			currency = "INR"
		}
		isActive := true
		if sku.IsActive != nil {
			isActive = *sku.IsActive
		}

		out = append(out, catalog.ProductSKU{
			ProductID:     productID,
			SKUCode:       strings.TrimSpace(sku.SKUCode),
			Size:          strings.TrimSpace(sku.Size),
			Color:         strings.TrimSpace(sku.Color),
			MRPAmount:     sku.MRPAmount,
			SellingAmount: sku.SellingAmount,
			Currency:      currency,
			IsActive:      isActive,
		})
	}

	return out
}

func mapAttributeRequests(requests []catalog.CreateProductAttributeRequest, productID uint) []catalog.ProductAttribute {
	out := make([]catalog.ProductAttribute, 0, len(requests))
	for _, attr := range requests {
		out = append(out, catalog.ProductAttribute{
			ProductID: productID,
			Type:      attr.Type,
			Name:      strings.TrimSpace(attr.Name),
			Value:     strings.TrimSpace(attr.Value),
		})
	}

	return out
}

func isValidStatus(status catalog.ProductStatus) bool {
	switch status {
	case catalog.ProductStatusDraft, catalog.ProductStatusActive, catalog.ProductStatusInactive, catalog.ProductStatusArchived:
		return true
	default:
		return false
	}
}

func validateSKUs(skus []catalog.ProductSKU) error {
	seen := make(map[string]struct{}, len(skus))
	for _, sku := range skus {
		if strings.TrimSpace(sku.SKUCode) == "" {
			return catalog.ErrInvalidSKUCode
		}
		if sku.MRPAmount < 0 || sku.SellingAmount < 0 || sku.SellingAmount > sku.MRPAmount {
			return catalog.ErrInvalidMoneyAmount
		}
		if len(sku.Currency) != 3 {
			return catalog.ErrInvalidCurrency
		}

		key := strings.ToUpper(strings.TrimSpace(sku.SKUCode))
		if _, ok := seen[key]; ok {
			return catalog.ErrSKUCodeAlreadyExists
		}
		seen[key] = struct{}{}
	}

	return nil
}

func IsNotFoundError(err error) bool {
	return errors.Is(err, catalog.ErrCategoryNotFound) ||
		errors.Is(err, catalog.ErrProductNotFound) ||
		errors.Is(err, catalog.ErrSKUNotFound) ||
		errors.Is(err, catalog.ErrJobNotFound)
}
