package catalog

type CreateCategoryRequest struct {
	Name        string `json:"name" validate:"required,min=2,max=255"`
	Slug        string `json:"slug" validate:"required,min=2,max=255"`
	Description string `json:"description"`
	ParentID    *uint  `json:"parent_id"`
	IsActive    *bool  `json:"is_active"`
	SortOrder   *int   `json:"sort_order"`
}

type UpdateCategoryRequest struct {
	Name        *string `json:"name"`
	Slug        *string `json:"slug"`
	Description *string `json:"description"`
	ParentID    *uint   `json:"parent_id"`
	IsActive    *bool   `json:"is_active"`
	SortOrder   *int    `json:"sort_order"`
}

type ListCategoriesRequest struct {
	ParentID *uint  `form:"parent_id"`
	IsActive *bool  `form:"is_active"`
	Search   string `form:"search"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

type CreateProductRequest struct {
	CategoryID      uint                            `json:"category_id" validate:"required"`
	Name            string                          `json:"name" validate:"required,min=2,max=255"`
	Slug            string                          `json:"slug" validate:"required,min=2,max=255"`
	Description     string                          `json:"description"`
	Status          ProductStatus                   `json:"status" validate:"omitempty,oneof=draft active inactive archived"`
	FabricSummary   string                          `json:"fabric_summary"`
	WashCare        string                          `json:"wash_care"`
	DurabilityNotes string                          `json:"durability_notes"`
	Images          []CreateProductImageRequest     `json:"images"`
	SKUs            []CreateProductSKURequest       `json:"skus"`
	Attributes      []CreateProductAttributeRequest `json:"attributes"`
}

type UpdateProductRequest struct {
	CategoryID      *uint                            `json:"category_id"`
	Name            *string                          `json:"name"`
	Slug            *string                          `json:"slug"`
	Description     *string                          `json:"description"`
	Status          *ProductStatus                   `json:"status" validate:"omitempty,oneof=draft active inactive archived"`
	FabricSummary   *string                          `json:"fabric_summary"`
	WashCare        *string                          `json:"wash_care"`
	DurabilityNotes *string                          `json:"durability_notes"`
	Images          *[]CreateProductImageRequest     `json:"images"`
	SKUs            *[]CreateProductSKURequest       `json:"skus"`
	Attributes      *[]CreateProductAttributeRequest `json:"attributes"`
}

type ListProductsRequest struct {
	CategoryID *uint  `form:"category_id"`
	Size       string `form:"size"`
	Color      string `form:"color"`
	MinPrice   *int64 `form:"min_price"`
	MaxPrice   *int64 `form:"max_price"`
	Search     string `form:"search"`
	Sort       string `form:"sort"`
	Status     string `form:"status"`
	Page       int    `form:"page"`
	PageSize   int    `form:"page_size"`
}

type CreateProductImageRequest struct {
	URL       string `json:"url"`
	AltText   string `json:"alt_text"`
	SortOrder *int   `json:"sort_order"`
	IsPrimary *bool  `json:"is_primary"`
}

type CreateProductSKURequest struct {
	SKUCode       string `json:"sku_code" validate:"required,min=2,max=100"`
	Size          string `json:"size" validate:"required,min=1,max=50"`
	Color         string `json:"color" validate:"required,min=1,max=50"`
	MRPAmount     int64  `json:"mrp_amount"`
	SellingAmount int64  `json:"selling_amount"`
	Currency      string `json:"currency" validate:"omitempty,len=3"`
	IsActive      *bool  `json:"is_active"`
}

type UpdateProductSKURequest struct {
	Size          *string `json:"size"`
	Color         *string `json:"color"`
	MRPAmount     *int64  `json:"mrp_amount"`
	SellingAmount *int64  `json:"selling_amount"`
	Currency      *string `json:"currency"`
	IsActive      *bool   `json:"is_active"`
}

type CreateProductAttributeRequest struct {
	Type  AttributeType `json:"type" validate:"required,oneof=fabric performance fit care"`
	Name  string        `json:"name" validate:"required,min=1,max=100"`
	Value string        `json:"value" validate:"required"`
}

type CreateBulkUploadJobRequest struct {
	CreatedBy uint   `json:"created_by"`
	FileName  string `json:"file_name"`
}
