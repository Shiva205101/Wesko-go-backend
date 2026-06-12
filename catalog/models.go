package catalog

import "time"

type ProductStatus string

const (
	ProductStatusDraft    ProductStatus = "draft"
	ProductStatusActive   ProductStatus = "active"
	ProductStatusInactive ProductStatus = "inactive"
	ProductStatusArchived ProductStatus = "archived"
)

type AttributeType string

const (
	AttributeTypeFabric      AttributeType = "fabric"
	AttributeTypePerformance AttributeType = "performance"
	AttributeTypeFit         AttributeType = "fit"
	AttributeTypeCare        AttributeType = "care"
)

type BulkUploadStatus string

const (
	BulkUploadStatusPending    BulkUploadStatus = "pending"
	BulkUploadStatusProcessing BulkUploadStatus = "processing"
	BulkUploadStatusCompleted  BulkUploadStatus = "completed"
	BulkUploadStatusFailed     BulkUploadStatus = "failed"
)

type Category struct {
	ID          uint
	Name        string
	Slug        string
	Description string
	ParentID    *uint
	IsActive    bool
	SortOrder   int
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

type Product struct {
	ID              uint
	CategoryID      uint
	Name            string
	Slug            string
	Description     string
	Status          ProductStatus
	FabricSummary   string
	WashCare        string
	DurabilityNotes string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
	Images          []ProductImage
	SKUs            []ProductSKU
	Attributes      []ProductAttribute
}

type ProductImage struct {
	ID        uint
	ProductID uint
	URL       string
	AltText   string
	SortOrder int
	IsPrimary bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ProductSKU struct {
	ID            uint
	ProductID     uint
	SKUCode       string
	Size          string
	Color         string
	MRPAmount     int64
	SellingAmount int64
	Currency      string
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type ProductAttribute struct {
	ID        uint
	ProductID uint
	Type      AttributeType
	Name      string
	Value     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type BulkUploadJob struct {
	ID             uint
	CreatedBy      uint
	FileName       string
	Status         BulkUploadStatus
	TotalRows      int
	SuccessRows    int
	FailedRows     int
	ErrorReportURL string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ProductAvailability struct {
	ProductID      uint
	InStock        bool
	AvailableSKUs  int
	TotalActiveSKU int
}

type CategoryFilter struct {
	ParentID *uint
	IsActive *bool
	Search   string
	Page     int
	PageSize int
}

type ProductFilter struct {
	CategoryID *uint
	Size       string
	Color      string
	MinPrice   *int64
	MaxPrice   *int64
	Search     string
	Sort       string
	Status     *ProductStatus
	Page       int
	PageSize   int
}
