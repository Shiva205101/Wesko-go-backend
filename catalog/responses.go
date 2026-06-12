package catalog

import "time"

type CategoryResponse struct {
	ID          uint       `json:"id"`
	Name        string     `json:"name"`
	Slug        string     `json:"slug"`
	Description string     `json:"description"`
	ParentID    *uint      `json:"parent_id,omitempty"`
	IsActive    bool       `json:"is_active"`
	SortOrder   int        `json:"sort_order"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}

type ProductImageResponse struct {
	ID        uint      `json:"id"`
	URL       string    `json:"url"`
	AltText   string    `json:"alt_text"`
	SortOrder int       `json:"sort_order"`
	IsPrimary bool      `json:"is_primary"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ProductSKUResponse struct {
	ID            uint      `json:"id"`
	SKUCode       string    `json:"sku_code"`
	Size          string    `json:"size"`
	Color         string    `json:"color"`
	MRPAmount     int64     `json:"mrp_amount"`
	SellingAmount int64     `json:"selling_amount"`
	Currency      string    `json:"currency"`
	IsActive      bool      `json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ProductAttributeResponse struct {
	ID        uint          `json:"id"`
	Type      AttributeType `json:"type"`
	Name      string        `json:"name"`
	Value     string        `json:"value"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

type ProductResponse struct {
	ID              uint                       `json:"id"`
	CategoryID      uint                       `json:"category_id"`
	Name            string                     `json:"name"`
	Slug            string                     `json:"slug"`
	Description     string                     `json:"description"`
	Status          ProductStatus              `json:"status"`
	FabricSummary   string                     `json:"fabric_summary"`
	WashCare        string                     `json:"wash_care"`
	DurabilityNotes string                     `json:"durability_notes"`
	Images          []ProductImageResponse     `json:"images"`
	SKUs            []ProductSKUResponse       `json:"skus"`
	Attributes      []ProductAttributeResponse `json:"attributes"`
	CreatedAt       time.Time                  `json:"created_at"`
	UpdatedAt       time.Time                  `json:"updated_at"`
	DeletedAt       *time.Time                 `json:"deleted_at,omitempty"`
}

type ProductListResponse struct {
	Items      []ProductResponse `json:"items"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
	TotalCount int64             `json:"total_count"`
}

type BulkUploadJobResponse struct {
	ID             uint             `json:"id"`
	CreatedBy      uint             `json:"created_by"`
	FileName       string           `json:"file_name"`
	Status         BulkUploadStatus `json:"status"`
	TotalRows      int              `json:"total_rows"`
	SuccessRows    int              `json:"success_rows"`
	FailedRows     int              `json:"failed_rows"`
	ErrorReportURL string           `json:"error_report_url"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

type ProductAvailabilityResponse struct {
	ProductID      uint `json:"product_id"`
	InStock        bool `json:"in_stock"`
	AvailableSKUs  int  `json:"available_skus"`
	TotalActiveSKU int  `json:"total_active_skus"`
}
