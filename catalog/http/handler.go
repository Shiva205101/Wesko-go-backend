package http

import (
	"errors"
	"log/slog"
	stdhttp "net/http"
	"strconv"

	"vesko/catalog"
	catalogservice "vesko/catalog/service"
	applogger "vesko/logger"
	"vesko/requestctx"
	"vesko/validation"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *catalogservice.Service
	logger  *slog.Logger
}

type errorResponse struct {
	Error     string            `json:"error"`
	RequestID string            `json:"request_id"`
	Details   map[string]string `json:"details,omitempty"`
}

func New(service *catalogservice.Service, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{
		service: service,
		logger:  logger.With("component", "catalog_http"),
	}
}

func (h *Handler) RegisterRoutes(router gin.IRouter) {
	router.POST("/catalog/categories", h.handleCreateCategory)
	router.GET("/catalog/categories", h.handleListCategories)
	router.GET("/catalog/categories/:id", h.handleGetCategory)
	router.PUT("/catalog/categories/:id", h.handleUpdateCategory)
	router.DELETE("/catalog/categories/:id", h.handleDeleteCategory)

	router.POST("/catalog/products", h.handleCreateProduct)
	router.GET("/catalog/products", h.handleListProducts)
	router.GET("/catalog/products/:id", h.handleGetProduct)
	router.PUT("/catalog/products/:id", h.handleUpdateProduct)
	router.DELETE("/catalog/products/:id", h.handleDeleteProduct)
}

func (h *Handler) handleCreateCategory(c *gin.Context) {
	var req catalog.CreateCategoryRequest
	if err := h.decodeAndValidateJSON(c, &req); err != nil {
		return
	}

	item, err := h.service.CreateCategory(c.Request.Context(), req)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	c.JSON(stdhttp.StatusCreated, toCategoryResponse(item))
}

func (h *Handler) handleListCategories(c *gin.Context) {
	var req catalog.ListCategoriesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(stdhttp.StatusBadRequest, errorResponse{Error: "invalid query params", RequestID: requestIDFromContext(c)})
		return
	}

	items, total, err := h.service.ListCategories(c.Request.Context(), req)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	out := make([]catalog.CategoryResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toCategoryResponse(item))
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"items":       out,
		"page":        req.Page,
		"page_size":   req.PageSize,
		"total_count": total,
	})
}

func (h *Handler) handleGetCategory(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	item, err := h.service.GetCategory(c.Request.Context(), id)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	c.JSON(stdhttp.StatusOK, toCategoryResponse(item))
}

func (h *Handler) handleUpdateCategory(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	var req catalog.UpdateCategoryRequest
	if err := h.decodeAndValidateJSON(c, &req); err != nil {
		return
	}

	item, err := h.service.UpdateCategory(c.Request.Context(), id, req)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	c.JSON(stdhttp.StatusOK, toCategoryResponse(item))
}

func (h *Handler) handleDeleteCategory(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	if err := h.service.DeleteCategory(c.Request.Context(), id); err != nil {
		h.writeServiceError(c, err)
		return
	}

	c.Status(stdhttp.StatusNoContent)
}

func (h *Handler) handleCreateProduct(c *gin.Context) {
	var req catalog.CreateProductRequest
	if err := h.decodeAndValidateJSON(c, &req); err != nil {
		return
	}

	item, err := h.service.CreateProduct(c.Request.Context(), req)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	c.JSON(stdhttp.StatusCreated, toProductResponse(item))
}

func (h *Handler) handleListProducts(c *gin.Context) {
	var req catalog.ListProductsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(stdhttp.StatusBadRequest, errorResponse{Error: "invalid query params", RequestID: requestIDFromContext(c)})
		return
	}

	items, total, err := h.service.ListProducts(c.Request.Context(), req)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	out := make([]catalog.ProductResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toProductResponse(item))
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	c.JSON(stdhttp.StatusOK, catalog.ProductListResponse{
		Items:      out,
		Page:       page,
		PageSize:   pageSize,
		TotalCount: total,
	})
}

func (h *Handler) handleGetProduct(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	item, err := h.service.GetProduct(c.Request.Context(), id)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	c.JSON(stdhttp.StatusOK, toProductResponse(item))
}

func (h *Handler) handleUpdateProduct(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	var req catalog.UpdateProductRequest
	if err := h.decodeAndValidateJSON(c, &req); err != nil {
		return
	}

	item, err := h.service.UpdateProduct(c.Request.Context(), id, req)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	c.JSON(stdhttp.StatusOK, toProductResponse(item))
}

func (h *Handler) handleDeleteProduct(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	if err := h.service.DeleteProduct(c.Request.Context(), id); err != nil {
		h.writeServiceError(c, err)
		return
	}

	c.Status(stdhttp.StatusNoContent)
}

func (h *Handler) decodeAndValidateJSON(c *gin.Context, dst any) error {
	l := applogger.FromContext(c.Request.Context())
	if err := c.ShouldBindJSON(dst); err != nil {
		l.Warn("invalid request body", "path", c.FullPath(), "error", err.Error())
		c.JSON(stdhttp.StatusBadRequest, errorResponse{Error: "invalid request body", RequestID: requestIDFromContext(c)})
		return err
	}
	if err := validation.Validate(dst); err != nil {
		l.Warn("request validation failed", "path", c.FullPath(), "error", err.Error())
		var validationErrs validation.Errors
		if errors.As(err, &validationErrs) {
			c.JSON(stdhttp.StatusBadRequest, errorResponse{
				Error:     validation.ErrValidationFailed.Error(),
				RequestID: requestIDFromContext(c),
				Details:   validationErrs.Messages(),
			})
			return err
		}
		c.JSON(stdhttp.StatusBadRequest, errorResponse{Error: validation.ErrValidationFailed.Error(), RequestID: requestIDFromContext(c)})
		return err
	}

	return nil
}

func (h *Handler) writeServiceError(c *gin.Context, err error) {
	l := applogger.FromContext(c.Request.Context())
	status := stdhttp.StatusInternalServerError

	switch {
	case errors.Is(err, catalog.ErrInvalidCategoryName),
		errors.Is(err, catalog.ErrInvalidCategorySlug),
		errors.Is(err, catalog.ErrInvalidProductName),
		errors.Is(err, catalog.ErrInvalidProductSlug),
		errors.Is(err, catalog.ErrInvalidProductStatus),
		errors.Is(err, catalog.ErrInvalidSKUCode),
		errors.Is(err, catalog.ErrInvalidMoneyAmount),
		errors.Is(err, catalog.ErrInvalidCurrency):
		status = stdhttp.StatusBadRequest
	case errors.Is(err, catalog.ErrCategoryNotFound),
		errors.Is(err, catalog.ErrProductNotFound),
		errors.Is(err, catalog.ErrSKUNotFound),
		errors.Is(err, catalog.ErrJobNotFound):
		status = stdhttp.StatusNotFound
	case errors.Is(err, catalog.ErrCategorySlugAlreadyExists),
		errors.Is(err, catalog.ErrProductSlugAlreadyExists),
		errors.Is(err, catalog.ErrSKUCodeAlreadyExists):
		status = stdhttp.StatusConflict
	}

	if status >= stdhttp.StatusInternalServerError {
		l.Error("catalog request failed", "method", c.Request.Method, "path", c.FullPath(), "status", status, "error", err.Error())
		c.JSON(status, errorResponse{Error: "internal server error", RequestID: requestIDFromContext(c)})
		return
	}

	l.Warn("catalog request rejected", "method", c.Request.Method, "path", c.FullPath(), "status", status, "error", err.Error())
	c.JSON(status, errorResponse{Error: err.Error(), RequestID: requestIDFromContext(c)})
}

func parseUintParam(c *gin.Context, name string) (uint, bool) {
	raw := c.Param(name)
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		c.JSON(stdhttp.StatusBadRequest, errorResponse{
			Error:     "invalid path parameter: " + name,
			RequestID: requestIDFromContext(c),
		})
		return 0, false
	}
	return uint(value), true
}

func toCategoryResponse(item catalog.Category) catalog.CategoryResponse {
	return catalog.CategoryResponse{
		ID:          item.ID,
		Name:        item.Name,
		Slug:        item.Slug,
		Description: item.Description,
		ParentID:    item.ParentID,
		IsActive:    item.IsActive,
		SortOrder:   item.SortOrder,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
		DeletedAt:   item.DeletedAt,
	}
}

func toProductResponse(item catalog.Product) catalog.ProductResponse {
	images := make([]catalog.ProductImageResponse, 0, len(item.Images))
	for _, image := range item.Images {
		images = append(images, catalog.ProductImageResponse{
			ID:        image.ID,
			URL:       image.URL,
			AltText:   image.AltText,
			SortOrder: image.SortOrder,
			IsPrimary: image.IsPrimary,
			CreatedAt: image.CreatedAt,
			UpdatedAt: image.UpdatedAt,
		})
	}

	skus := make([]catalog.ProductSKUResponse, 0, len(item.SKUs))
	for _, sku := range item.SKUs {
		skus = append(skus, catalog.ProductSKUResponse{
			ID:            sku.ID,
			SKUCode:       sku.SKUCode,
			Size:          sku.Size,
			Color:         sku.Color,
			MRPAmount:     sku.MRPAmount,
			SellingAmount: sku.SellingAmount,
			Currency:      sku.Currency,
			IsActive:      sku.IsActive,
			CreatedAt:     sku.CreatedAt,
			UpdatedAt:     sku.UpdatedAt,
		})
	}

	attributes := make([]catalog.ProductAttributeResponse, 0, len(item.Attributes))
	for _, attribute := range item.Attributes {
		attributes = append(attributes, catalog.ProductAttributeResponse{
			ID:        attribute.ID,
			Type:      attribute.Type,
			Name:      attribute.Name,
			Value:     attribute.Value,
			CreatedAt: attribute.CreatedAt,
			UpdatedAt: attribute.UpdatedAt,
		})
	}

	return catalog.ProductResponse{
		ID:              item.ID,
		CategoryID:      item.CategoryID,
		Name:            item.Name,
		Slug:            item.Slug,
		Description:     item.Description,
		Status:          item.Status,
		FabricSummary:   item.FabricSummary,
		WashCare:        item.WashCare,
		DurabilityNotes: item.DurabilityNotes,
		Images:          images,
		SKUs:            skus,
		Attributes:      attributes,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
		DeletedAt:       item.DeletedAt,
	}
}

func requestIDFromContext(c *gin.Context) string {
	requestID := requestctx.RequestID(c.Request.Context())
	if requestID != "" {
		return requestID
	}
	return c.GetHeader("X-Request-ID")
}
