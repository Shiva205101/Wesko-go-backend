package productdao

import (
	"context"
	"errors"
	"strings"
	"time"

	"vesko/catalog"

	"gorm.io/gorm"
)

type CategoryModel struct {
	ID          uint           `gorm:"primaryKey;autoIncrement"`
	Name        string         `gorm:"size:255;not null"`
	Slug        string         `gorm:"size:255;not null;uniqueIndex"`
	Description string         `gorm:"type:text"`
	ParentID    *uint          `gorm:"index"`
	IsActive    bool           `gorm:"not null;default:true;index"`
	SortOrder   int            `gorm:"not null;default:0"`
	CreatedAt   time.Time      `gorm:"not null"`
	UpdatedAt   time.Time      `gorm:"not null"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func (CategoryModel) TableName() string {
	return "catalog_categories"
}

type ProductModel struct {
	ID              uint                    `gorm:"primaryKey;autoIncrement"`
	CategoryID      uint                    `gorm:"not null;index"`
	Name            string                  `gorm:"size:255;not null"`
	Slug            string                  `gorm:"size:255;not null;uniqueIndex"`
	Description     string                  `gorm:"type:text"`
	Status          string                  `gorm:"size:20;not null;default:'draft';index"`
	FabricSummary   string                  `gorm:"type:text"`
	WashCare        string                  `gorm:"type:text"`
	DurabilityNotes string                  `gorm:"type:text"`
	CreatedAt       time.Time               `gorm:"not null"`
	UpdatedAt       time.Time               `gorm:"not null"`
	DeletedAt       gorm.DeletedAt          `gorm:"index"`
	Images          []ProductImageModel     `gorm:"foreignKey:ProductID;references:ID"`
	SKUs            []ProductSKUModel       `gorm:"foreignKey:ProductID;references:ID"`
	Attributes      []ProductAttributeModel `gorm:"foreignKey:ProductID;references:ID"`
}

func (ProductModel) TableName() string {
	return "catalog_products"
}

type ProductImageModel struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	ProductID uint      `gorm:"not null;index"`
	URL       string    `gorm:"size:1024;not null"`
	AltText   string    `gorm:"size:255"`
	SortOrder int       `gorm:"not null;default:0"`
	IsPrimary bool      `gorm:"not null;default:false;index"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (ProductImageModel) TableName() string {
	return "catalog_product_images"
}

type ProductSKUModel struct {
	ID            uint      `gorm:"primaryKey;autoIncrement"`
	ProductID     uint      `gorm:"not null;index"`
	SKUCode       string    `gorm:"size:100;not null;uniqueIndex"`
	Size          string    `gorm:"size:50;not null;index"`
	Color         string    `gorm:"size:50;not null;index"`
	MRPAmount     int64     `gorm:"not null"`
	SellingAmount int64     `gorm:"not null"`
	Currency      string    `gorm:"size:3;not null;default:'INR'"`
	IsActive      bool      `gorm:"not null;default:true;index"`
	CreatedAt     time.Time `gorm:"not null"`
	UpdatedAt     time.Time `gorm:"not null"`
}

func (ProductSKUModel) TableName() string {
	return "catalog_skus"
}

type ProductAttributeModel struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	ProductID uint      `gorm:"not null;index"`
	Type      string    `gorm:"size:30;not null;index"`
	Name      string    `gorm:"size:100;not null"`
	Value     string    `gorm:"type:text;not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (ProductAttributeModel) TableName() string {
	return "catalog_product_attributes"
}

type BulkUploadJobModel struct {
	ID             uint      `gorm:"primaryKey;autoIncrement"`
	CreatedBy      uint      `gorm:"not null;index"`
	FileName       string    `gorm:"size:255;not null"`
	Status         string    `gorm:"size:20;not null;default:'pending';index"`
	TotalRows      int       `gorm:"not null;default:0"`
	SuccessRows    int       `gorm:"not null;default:0"`
	FailedRows     int       `gorm:"not null;default:0"`
	ErrorReportURL string    `gorm:"size:1024"`
	CreatedAt      time.Time `gorm:"not null"`
	UpdatedAt      time.Time `gorm:"not null"`
}

func (BulkUploadJobModel) TableName() string {
	return "catalog_bulk_upload_jobs"
}

type PostgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) AutoMigrate(ctx context.Context) error {
	return r.db.WithContext(ctx).AutoMigrate(
		&CategoryModel{},
		&ProductModel{},
		&ProductImageModel{},
		&ProductSKUModel{},
		&ProductAttributeModel{},
		&BulkUploadJobModel{},
	)
}

func (r *PostgresRepository) CreateCategory(ctx context.Context, category catalog.Category) (catalog.Category, error) {
	model := toCategoryModel(category)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return catalog.Category{}, mapDBError(err)
	}

	return toCategoryDomain(model), nil
}

func (r *PostgresRepository) GetCategoryByID(ctx context.Context, id uint) (catalog.Category, error) {
	var model CategoryModel
	if err := r.db.WithContext(ctx).First(&model, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return catalog.Category{}, catalog.ErrCategoryNotFound
		}
		return catalog.Category{}, err
	}

	return toCategoryDomain(model), nil
}

func (r *PostgresRepository) UpdateCategory(ctx context.Context, category catalog.Category) (catalog.Category, error) {
	model := toCategoryModel(category)
	result := r.db.WithContext(ctx).
		Model(&CategoryModel{}).
		Where("id = ?", category.ID).
		Updates(map[string]any{
			"name":        model.Name,
			"slug":        model.Slug,
			"description": model.Description,
			"parent_id":   model.ParentID,
			"is_active":   model.IsActive,
			"sort_order":  model.SortOrder,
			"updated_at":  time.Now().UTC(),
		})
	if result.Error != nil {
		return catalog.Category{}, mapDBError(result.Error)
	}
	if result.RowsAffected == 0 {
		return catalog.Category{}, catalog.ErrCategoryNotFound
	}

	return r.GetCategoryByID(ctx, category.ID)
}

func (r *PostgresRepository) DeleteCategory(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&CategoryModel{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return catalog.ErrCategoryNotFound
	}

	return nil
}

func (r *PostgresRepository) ListCategories(ctx context.Context, filter catalog.CategoryFilter) ([]catalog.Category, int64, error) {
	page, pageSize := normalizePagination(filter.Page, filter.PageSize)
	query := r.db.WithContext(ctx).Model(&CategoryModel{})

	if filter.ParentID != nil {
		query = query.Where("parent_id = ?", *filter.ParentID)
	}
	if filter.IsActive != nil {
		query = query.Where("is_active = ?", *filter.IsActive)
	}
	if filter.Search != "" {
		pattern := "%" + filter.Search + "%"
		query = query.Where("name ILIKE ? OR slug ILIKE ?", pattern, pattern)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var models []CategoryModel
	if err := query.Order("sort_order ASC, id ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&models).Error; err != nil {
		return nil, 0, err
	}

	items := make([]catalog.Category, 0, len(models))
	for _, model := range models {
		items = append(items, toCategoryDomain(model))
	}

	return items, total, nil
}

func (r *PostgresRepository) CreateProduct(ctx context.Context, product catalog.Product) (catalog.Product, error) {
	model := toProductModel(product)

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model).Error; err != nil {
			return mapDBError(err)
		}

		if len(product.Images) > 0 {
			images := make([]ProductImageModel, 0, len(product.Images))
			for _, image := range product.Images {
				image.ProductID = model.ID
				images = append(images, toProductImageModel(image))
			}
			if err := tx.Create(&images).Error; err != nil {
				return mapDBError(err)
			}
		}

		if len(product.SKUs) > 0 {
			skus := make([]ProductSKUModel, 0, len(product.SKUs))
			for _, sku := range product.SKUs {
				sku.ProductID = model.ID
				skus = append(skus, toProductSKUModel(sku))
			}
			if err := tx.Create(&skus).Error; err != nil {
				return mapDBError(err)
			}
		}

		if len(product.Attributes) > 0 {
			attributes := make([]ProductAttributeModel, 0, len(product.Attributes))
			for _, attribute := range product.Attributes {
				attribute.ProductID = model.ID
				attributes = append(attributes, toProductAttributeModel(attribute))
			}
			if err := tx.Create(&attributes).Error; err != nil {
				return mapDBError(err)
			}
		}

		return nil
	})
	if err != nil {
		return catalog.Product{}, err
	}

	return r.GetProductByID(ctx, model.ID)
}

func (r *PostgresRepository) GetProductByID(ctx context.Context, id uint) (catalog.Product, error) {
	var model ProductModel
	if err := r.db.WithContext(ctx).
		Preload("Images", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC, id ASC")
		}).
		Preload("SKUs", func(db *gorm.DB) *gorm.DB {
			return db.Order("id ASC")
		}).
		Preload("Attributes", func(db *gorm.DB) *gorm.DB {
			return db.Order("id ASC")
		}).
		First(&model, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return catalog.Product{}, catalog.ErrProductNotFound
		}
		return catalog.Product{}, err
	}

	return toProductDomain(model), nil
}

func (r *PostgresRepository) UpdateProduct(ctx context.Context, product catalog.Product) (catalog.Product, error) {
	model := toProductModel(product)

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&ProductModel{}).Where("id = ?", product.ID).Updates(map[string]any{
			"category_id":      model.CategoryID,
			"name":             model.Name,
			"slug":             model.Slug,
			"description":      model.Description,
			"status":           model.Status,
			"fabric_summary":   model.FabricSummary,
			"wash_care":        model.WashCare,
			"durability_notes": model.DurabilityNotes,
			"updated_at":       time.Now().UTC(),
		})
		if result.Error != nil {
			return mapDBError(result.Error)
		}
		if result.RowsAffected == 0 {
			return catalog.ErrProductNotFound
		}

		if err := tx.Where("product_id = ?", product.ID).Delete(&ProductImageModel{}).Error; err != nil {
			return err
		}
		if len(product.Images) > 0 {
			images := make([]ProductImageModel, 0, len(product.Images))
			for _, image := range product.Images {
				image.ProductID = product.ID
				images = append(images, toProductImageModel(image))
			}
			if err := tx.Create(&images).Error; err != nil {
				return mapDBError(err)
			}
		}

		if err := tx.Where("product_id = ?", product.ID).Delete(&ProductSKUModel{}).Error; err != nil {
			return err
		}
		if len(product.SKUs) > 0 {
			skus := make([]ProductSKUModel, 0, len(product.SKUs))
			for _, sku := range product.SKUs {
				sku.ProductID = product.ID
				skus = append(skus, toProductSKUModel(sku))
			}
			if err := tx.Create(&skus).Error; err != nil {
				return mapDBError(err)
			}
		}

		if err := tx.Where("product_id = ?", product.ID).Delete(&ProductAttributeModel{}).Error; err != nil {
			return err
		}
		if len(product.Attributes) > 0 {
			attributes := make([]ProductAttributeModel, 0, len(product.Attributes))
			for _, attribute := range product.Attributes {
				attribute.ProductID = product.ID
				attributes = append(attributes, toProductAttributeModel(attribute))
			}
			if err := tx.Create(&attributes).Error; err != nil {
				return mapDBError(err)
			}
		}

		return nil
	})
	if err != nil {
		return catalog.Product{}, err
	}

	return r.GetProductByID(ctx, product.ID)
}

func (r *PostgresRepository) DeleteProduct(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&ProductModel{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return catalog.ErrProductNotFound
	}

	return nil
}

func (r *PostgresRepository) ListProducts(ctx context.Context, filter catalog.ProductFilter) ([]catalog.Product, int64, error) {
	page, pageSize := normalizePagination(filter.Page, filter.PageSize)
	query := r.db.WithContext(ctx).Model(&ProductModel{})

	joinSKU := filter.Size != "" || filter.Color != "" || filter.MinPrice != nil || filter.MaxPrice != nil || filter.Sort == "price_asc" || filter.Sort == "price_desc"
	if joinSKU {
		query = query.Joins("LEFT JOIN catalog_skus ON catalog_skus.product_id = catalog_products.id")
	}

	if filter.CategoryID != nil {
		query = query.Where("catalog_products.category_id = ?", *filter.CategoryID)
	}
	if filter.Status != nil {
		query = query.Where("catalog_products.status = ?", string(*filter.Status))
	}
	if filter.Search != "" {
		pattern := "%" + filter.Search + "%"
		query = query.Where("catalog_products.name ILIKE ? OR catalog_products.slug ILIKE ?", pattern, pattern)
	}
	if filter.Size != "" {
		query = query.Where("catalog_skus.size = ?", filter.Size)
	}
	if filter.Color != "" {
		query = query.Where("catalog_skus.color = ?", filter.Color)
	}
	if filter.MinPrice != nil {
		query = query.Where("catalog_skus.selling_amount >= ?", *filter.MinPrice)
	}
	if filter.MaxPrice != nil {
		query = query.Where("catalog_skus.selling_amount <= ?", *filter.MaxPrice)
	}

	var total int64
	if err := query.Distinct("catalog_products.id").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	orderBy := "catalog_products.created_at DESC"
	switch filter.Sort {
	case "oldest":
		orderBy = "catalog_products.created_at ASC"
	case "price_asc":
		orderBy = "catalog_skus.selling_amount ASC NULLS LAST, catalog_products.created_at DESC"
	case "price_desc":
		orderBy = "catalog_skus.selling_amount DESC NULLS LAST, catalog_products.created_at DESC"
	}

	var models []ProductModel
	if err := query.
		Distinct("catalog_products.id").
		Preload("Images", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC, id ASC")
		}).
		Preload("SKUs", func(db *gorm.DB) *gorm.DB {
			return db.Order("id ASC")
		}).
		Preload("Attributes", func(db *gorm.DB) *gorm.DB {
			return db.Order("id ASC")
		}).
		Order(orderBy).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&models).Error; err != nil {
		return nil, 0, err
	}

	items := make([]catalog.Product, 0, len(models))
	for _, model := range models {
		items = append(items, toProductDomain(model))
	}

	return items, total, nil
}

func (r *PostgresRepository) CreateSKU(ctx context.Context, sku catalog.ProductSKU) (catalog.ProductSKU, error) {
	model := toProductSKUModel(sku)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return catalog.ProductSKU{}, mapDBError(err)
	}

	return toProductSKUDomain(model), nil
}

func (r *PostgresRepository) UpdateSKU(ctx context.Context, sku catalog.ProductSKU) (catalog.ProductSKU, error) {
	model := toProductSKUModel(sku)
	result := r.db.WithContext(ctx).Model(&ProductSKUModel{}).Where("id = ?", sku.ID).Updates(map[string]any{
		"size":           model.Size,
		"color":          model.Color,
		"mrp_amount":     model.MRPAmount,
		"selling_amount": model.SellingAmount,
		"currency":       model.Currency,
		"is_active":      model.IsActive,
		"updated_at":     time.Now().UTC(),
	})
	if result.Error != nil {
		return catalog.ProductSKU{}, mapDBError(result.Error)
	}
	if result.RowsAffected == 0 {
		return catalog.ProductSKU{}, catalog.ErrSKUNotFound
	}

	var out ProductSKUModel
	if err := r.db.WithContext(ctx).First(&out, sku.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return catalog.ProductSKU{}, catalog.ErrSKUNotFound
		}
		return catalog.ProductSKU{}, err
	}

	return toProductSKUDomain(out), nil
}

func (r *PostgresRepository) DeleteSKU(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&ProductSKUModel{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return catalog.ErrSKUNotFound
	}

	return nil
}

func (r *PostgresRepository) ListProductSKUs(ctx context.Context, productID uint) ([]catalog.ProductSKU, error) {
	var models []ProductSKUModel
	if err := r.db.WithContext(ctx).Where("product_id = ?", productID).Order("id ASC").Find(&models).Error; err != nil {
		return nil, err
	}

	items := make([]catalog.ProductSKU, 0, len(models))
	for _, model := range models {
		items = append(items, toProductSKUDomain(model))
	}

	return items, nil
}

func (r *PostgresRepository) ReplaceProductImages(ctx context.Context, productID uint, images []catalog.ProductImage) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("product_id = ?", productID).Delete(&ProductImageModel{}).Error; err != nil {
			return err
		}
		if len(images) == 0 {
			return nil
		}

		models := make([]ProductImageModel, 0, len(images))
		for _, image := range images {
			image.ProductID = productID
			models = append(models, toProductImageModel(image))
		}
		return tx.Create(&models).Error
	})
}

func (r *PostgresRepository) ReplaceProductAttributes(ctx context.Context, productID uint, attributes []catalog.ProductAttribute) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("product_id = ?", productID).Delete(&ProductAttributeModel{}).Error; err != nil {
			return err
		}
		if len(attributes) == 0 {
			return nil
		}

		models := make([]ProductAttributeModel, 0, len(attributes))
		for _, attribute := range attributes {
			attribute.ProductID = productID
			models = append(models, toProductAttributeModel(attribute))
		}
		return tx.Create(&models).Error
	})
}

func (r *PostgresRepository) CreateBulkUploadJob(ctx context.Context, job catalog.BulkUploadJob) (catalog.BulkUploadJob, error) {
	model := toBulkUploadJobModel(job)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return catalog.BulkUploadJob{}, err
	}

	return toBulkUploadJobDomain(model), nil
}

func (r *PostgresRepository) UpdateBulkUploadJob(ctx context.Context, job catalog.BulkUploadJob) (catalog.BulkUploadJob, error) {
	model := toBulkUploadJobModel(job)
	result := r.db.WithContext(ctx).Model(&BulkUploadJobModel{}).Where("id = ?", job.ID).Updates(map[string]any{
		"status":           model.Status,
		"total_rows":       model.TotalRows,
		"success_rows":     model.SuccessRows,
		"failed_rows":      model.FailedRows,
		"error_report_url": model.ErrorReportURL,
		"updated_at":       time.Now().UTC(),
	})
	if result.Error != nil {
		return catalog.BulkUploadJob{}, result.Error
	}
	if result.RowsAffected == 0 {
		return catalog.BulkUploadJob{}, catalog.ErrJobNotFound
	}

	return r.GetBulkUploadJobByID(ctx, job.ID)
}

func (r *PostgresRepository) GetBulkUploadJobByID(ctx context.Context, id uint) (catalog.BulkUploadJob, error) {
	var model BulkUploadJobModel
	if err := r.db.WithContext(ctx).First(&model, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return catalog.BulkUploadJob{}, catalog.ErrJobNotFound
		}
		return catalog.BulkUploadJob{}, err
	}
	return toBulkUploadJobDomain(model), nil
}

func (r *PostgresRepository) GetProductAvailability(ctx context.Context, productID uint) (catalog.ProductAvailability, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&ProductSKUModel{}).
		Where("product_id = ? AND is_active = ?", productID, true).
		Count(&total).Error; err != nil {
		return catalog.ProductAvailability{}, err
	}

	available := int(total)
	return catalog.ProductAvailability{
		ProductID:      productID,
		InStock:        available > 0,
		AvailableSKUs:  available,
		TotalActiveSKU: available,
	}, nil
}

func normalizePagination(page int, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	return page, pageSize
}

func mapDBError(err error) error {
	switch {
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return mapDuplicateError(err)
	default:
		return err
	}
}

func mapDuplicateError(err error) error {
	msg := err.Error()
	switch {
	case contains(msg, "catalog_categories"), contains(msg, "slug"):
		return catalog.ErrCategorySlugAlreadyExists
	case contains(msg, "catalog_products"), contains(msg, "slug"):
		return catalog.ErrProductSlugAlreadyExists
	case contains(msg, "catalog_skus"), contains(msg, "sku_code"):
		return catalog.ErrSKUCodeAlreadyExists
	default:
		return err
	}
}

func contains(source string, needle string) bool {
	return strings.Contains(strings.ToLower(source), strings.ToLower(needle))
}

func toCategoryModel(category catalog.Category) CategoryModel {
	return CategoryModel{
		ID:          category.ID,
		Name:        category.Name,
		Slug:        category.Slug,
		Description: category.Description,
		ParentID:    category.ParentID,
		IsActive:    category.IsActive,
		SortOrder:   category.SortOrder,
		CreatedAt:   category.CreatedAt,
		UpdatedAt:   category.UpdatedAt,
	}
}

func toCategoryDomain(model CategoryModel) catalog.Category {
	var deletedAt *time.Time
	if model.DeletedAt.Valid {
		t := model.DeletedAt.Time
		deletedAt = &t
	}

	return catalog.Category{
		ID:          model.ID,
		Name:        model.Name,
		Slug:        model.Slug,
		Description: model.Description,
		ParentID:    model.ParentID,
		IsActive:    model.IsActive,
		SortOrder:   model.SortOrder,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
		DeletedAt:   deletedAt,
	}
}

func toProductModel(product catalog.Product) ProductModel {
	return ProductModel{
		ID:              product.ID,
		CategoryID:      product.CategoryID,
		Name:            product.Name,
		Slug:            product.Slug,
		Description:     product.Description,
		Status:          string(product.Status),
		FabricSummary:   product.FabricSummary,
		WashCare:        product.WashCare,
		DurabilityNotes: product.DurabilityNotes,
		CreatedAt:       product.CreatedAt,
		UpdatedAt:       product.UpdatedAt,
	}
}

func toProductImageModel(image catalog.ProductImage) ProductImageModel {
	return ProductImageModel{
		ID:        image.ID,
		ProductID: image.ProductID,
		URL:       image.URL,
		AltText:   image.AltText,
		SortOrder: image.SortOrder,
		IsPrimary: image.IsPrimary,
		CreatedAt: image.CreatedAt,
		UpdatedAt: image.UpdatedAt,
	}
}

func toProductSKUModel(sku catalog.ProductSKU) ProductSKUModel {
	return ProductSKUModel{
		ID:            sku.ID,
		ProductID:     sku.ProductID,
		SKUCode:       sku.SKUCode,
		Size:          sku.Size,
		Color:         sku.Color,
		MRPAmount:     sku.MRPAmount,
		SellingAmount: sku.SellingAmount,
		Currency:      sku.Currency,
		IsActive:      sku.IsActive,
		CreatedAt:     sku.CreatedAt,
		UpdatedAt:     sku.UpdatedAt,
	}
}

func toProductAttributeModel(attribute catalog.ProductAttribute) ProductAttributeModel {
	return ProductAttributeModel{
		ID:        attribute.ID,
		ProductID: attribute.ProductID,
		Type:      string(attribute.Type),
		Name:      attribute.Name,
		Value:     attribute.Value,
		CreatedAt: attribute.CreatedAt,
		UpdatedAt: attribute.UpdatedAt,
	}
}

func toBulkUploadJobModel(job catalog.BulkUploadJob) BulkUploadJobModel {
	return BulkUploadJobModel{
		ID:             job.ID,
		CreatedBy:      job.CreatedBy,
		FileName:       job.FileName,
		Status:         string(job.Status),
		TotalRows:      job.TotalRows,
		SuccessRows:    job.SuccessRows,
		FailedRows:     job.FailedRows,
		ErrorReportURL: job.ErrorReportURL,
		CreatedAt:      job.CreatedAt,
		UpdatedAt:      job.UpdatedAt,
	}
}

func toProductDomain(model ProductModel) catalog.Product {
	var deletedAt *time.Time
	if model.DeletedAt.Valid {
		t := model.DeletedAt.Time
		deletedAt = &t
	}

	images := make([]catalog.ProductImage, 0, len(model.Images))
	for _, image := range model.Images {
		images = append(images, toProductImageDomain(image))
	}

	skus := make([]catalog.ProductSKU, 0, len(model.SKUs))
	for _, sku := range model.SKUs {
		skus = append(skus, toProductSKUDomain(sku))
	}

	attributes := make([]catalog.ProductAttribute, 0, len(model.Attributes))
	for _, attribute := range model.Attributes {
		attributes = append(attributes, toProductAttributeDomain(attribute))
	}

	return catalog.Product{
		ID:              model.ID,
		CategoryID:      model.CategoryID,
		Name:            model.Name,
		Slug:            model.Slug,
		Description:     model.Description,
		Status:          catalog.ProductStatus(model.Status),
		FabricSummary:   model.FabricSummary,
		WashCare:        model.WashCare,
		DurabilityNotes: model.DurabilityNotes,
		CreatedAt:       model.CreatedAt,
		UpdatedAt:       model.UpdatedAt,
		DeletedAt:       deletedAt,
		Images:          images,
		SKUs:            skus,
		Attributes:      attributes,
	}
}

func toProductImageDomain(model ProductImageModel) catalog.ProductImage {
	return catalog.ProductImage{
		ID:        model.ID,
		ProductID: model.ProductID,
		URL:       model.URL,
		AltText:   model.AltText,
		SortOrder: model.SortOrder,
		IsPrimary: model.IsPrimary,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}
}

func toProductSKUDomain(model ProductSKUModel) catalog.ProductSKU {
	return catalog.ProductSKU{
		ID:            model.ID,
		ProductID:     model.ProductID,
		SKUCode:       model.SKUCode,
		Size:          model.Size,
		Color:         model.Color,
		MRPAmount:     model.MRPAmount,
		SellingAmount: model.SellingAmount,
		Currency:      model.Currency,
		IsActive:      model.IsActive,
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
	}
}

func toProductAttributeDomain(model ProductAttributeModel) catalog.ProductAttribute {
	return catalog.ProductAttribute{
		ID:        model.ID,
		ProductID: model.ProductID,
		Type:      catalog.AttributeType(model.Type),
		Name:      model.Name,
		Value:     model.Value,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}
}

func toBulkUploadJobDomain(model BulkUploadJobModel) catalog.BulkUploadJob {
	return catalog.BulkUploadJob{
		ID:             model.ID,
		CreatedBy:      model.CreatedBy,
		FileName:       model.FileName,
		Status:         catalog.BulkUploadStatus(model.Status),
		TotalRows:      model.TotalRows,
		SuccessRows:    model.SuccessRows,
		FailedRows:     model.FailedRows,
		ErrorReportURL: model.ErrorReportURL,
		CreatedAt:      model.CreatedAt,
		UpdatedAt:      model.UpdatedAt,
	}
}
