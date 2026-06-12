package catalog

import "context"

type Repository interface {
	AutoMigrate(ctx context.Context) error

	CreateCategory(ctx context.Context, category Category) (Category, error)
	GetCategoryByID(ctx context.Context, id uint) (Category, error)
	UpdateCategory(ctx context.Context, category Category) (Category, error)
	DeleteCategory(ctx context.Context, id uint) error
	ListCategories(ctx context.Context, filter CategoryFilter) ([]Category, int64, error)

	CreateProduct(ctx context.Context, product Product) (Product, error)
	GetProductByID(ctx context.Context, id uint) (Product, error)
	UpdateProduct(ctx context.Context, product Product) (Product, error)
	DeleteProduct(ctx context.Context, id uint) error
	ListProducts(ctx context.Context, filter ProductFilter) ([]Product, int64, error)

	CreateSKU(ctx context.Context, sku ProductSKU) (ProductSKU, error)
	UpdateSKU(ctx context.Context, sku ProductSKU) (ProductSKU, error)
	DeleteSKU(ctx context.Context, id uint) error
	ListProductSKUs(ctx context.Context, productID uint) ([]ProductSKU, error)

	ReplaceProductImages(ctx context.Context, productID uint, images []ProductImage) error
	ReplaceProductAttributes(ctx context.Context, productID uint, attributes []ProductAttribute) error

	CreateBulkUploadJob(ctx context.Context, job BulkUploadJob) (BulkUploadJob, error)
	UpdateBulkUploadJob(ctx context.Context, job BulkUploadJob) (BulkUploadJob, error)
	GetBulkUploadJobByID(ctx context.Context, id uint) (BulkUploadJob, error)

	GetProductAvailability(ctx context.Context, productID uint) (ProductAvailability, error)
}
