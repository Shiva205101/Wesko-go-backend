package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"vesko/catalog"
	catalogservice "vesko/catalog/service"

	"github.com/gin-gonic/gin"
)

type fakeRepo struct {
	nextCategoryID uint
	nextProductID  uint
	categories     map[uint]catalog.Category
	products       map[uint]catalog.Product
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		nextCategoryID: 1,
		nextProductID:  1,
		categories:     map[uint]catalog.Category{},
		products:       map[uint]catalog.Product{},
	}
}

func (r *fakeRepo) AutoMigrate(context.Context) error { return nil }

func (r *fakeRepo) CreateCategory(_ context.Context, category catalog.Category) (catalog.Category, error) {
	for _, item := range r.categories {
		if item.Slug == category.Slug {
			return catalog.Category{}, catalog.ErrCategorySlugAlreadyExists
		}
	}
	category.ID = r.nextCategoryID
	r.nextCategoryID++
	r.categories[category.ID] = category
	return category, nil
}

func (r *fakeRepo) GetCategoryByID(_ context.Context, id uint) (catalog.Category, error) {
	item, ok := r.categories[id]
	if !ok {
		return catalog.Category{}, catalog.ErrCategoryNotFound
	}
	return item, nil
}

func (r *fakeRepo) UpdateCategory(_ context.Context, category catalog.Category) (catalog.Category, error) {
	if _, ok := r.categories[category.ID]; !ok {
		return catalog.Category{}, catalog.ErrCategoryNotFound
	}
	r.categories[category.ID] = category
	return category, nil
}

func (r *fakeRepo) DeleteCategory(_ context.Context, id uint) error {
	if _, ok := r.categories[id]; !ok {
		return catalog.ErrCategoryNotFound
	}
	delete(r.categories, id)
	return nil
}

func (r *fakeRepo) ListCategories(_ context.Context, _ catalog.CategoryFilter) ([]catalog.Category, int64, error) {
	items := make([]catalog.Category, 0, len(r.categories))
	for _, item := range r.categories {
		items = append(items, item)
	}
	return items, int64(len(items)), nil
}

func (r *fakeRepo) CreateProduct(_ context.Context, product catalog.Product) (catalog.Product, error) {
	for _, item := range r.products {
		if item.Slug == product.Slug {
			return catalog.Product{}, catalog.ErrProductSlugAlreadyExists
		}
	}
	product.ID = r.nextProductID
	r.nextProductID++
	r.products[product.ID] = product
	return product, nil
}

func (r *fakeRepo) GetProductByID(_ context.Context, id uint) (catalog.Product, error) {
	item, ok := r.products[id]
	if !ok {
		return catalog.Product{}, catalog.ErrProductNotFound
	}
	return item, nil
}

func (r *fakeRepo) UpdateProduct(_ context.Context, product catalog.Product) (catalog.Product, error) {
	if _, ok := r.products[product.ID]; !ok {
		return catalog.Product{}, catalog.ErrProductNotFound
	}
	r.products[product.ID] = product
	return product, nil
}

func (r *fakeRepo) DeleteProduct(_ context.Context, id uint) error {
	if _, ok := r.products[id]; !ok {
		return catalog.ErrProductNotFound
	}
	delete(r.products, id)
	return nil
}

func (r *fakeRepo) ListProducts(_ context.Context, _ catalog.ProductFilter) ([]catalog.Product, int64, error) {
	items := make([]catalog.Product, 0, len(r.products))
	for _, item := range r.products {
		items = append(items, item)
	}
	return items, int64(len(items)), nil
}

func (r *fakeRepo) CreateSKU(context.Context, catalog.ProductSKU) (catalog.ProductSKU, error) {
	return catalog.ProductSKU{}, nil
}
func (r *fakeRepo) UpdateSKU(context.Context, catalog.ProductSKU) (catalog.ProductSKU, error) {
	return catalog.ProductSKU{}, nil
}
func (r *fakeRepo) DeleteSKU(context.Context, uint) error { return nil }
func (r *fakeRepo) ListProductSKUs(context.Context, uint) ([]catalog.ProductSKU, error) {
	return nil, nil
}
func (r *fakeRepo) ReplaceProductImages(context.Context, uint, []catalog.ProductImage) error {
	return nil
}
func (r *fakeRepo) ReplaceProductAttributes(context.Context, uint, []catalog.ProductAttribute) error {
	return nil
}
func (r *fakeRepo) CreateBulkUploadJob(context.Context, catalog.BulkUploadJob) (catalog.BulkUploadJob, error) {
	return catalog.BulkUploadJob{}, nil
}
func (r *fakeRepo) UpdateBulkUploadJob(context.Context, catalog.BulkUploadJob) (catalog.BulkUploadJob, error) {
	return catalog.BulkUploadJob{}, nil
}
func (r *fakeRepo) GetBulkUploadJobByID(context.Context, uint) (catalog.BulkUploadJob, error) {
	return catalog.BulkUploadJob{}, catalog.ErrJobNotFound
}
func (r *fakeRepo) GetProductAvailability(context.Context, uint) (catalog.ProductAvailability, error) {
	return catalog.ProductAvailability{}, nil
}

func TestCreateCategory(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := newFakeRepo()
	service := catalogservice.New(repo, nil)
	handler := New(service, nil)

	router := gin.New()
	handler.RegisterRoutes(router)

	body := map[string]any{
		"name": "Casual",
		"slug": "casual",
	}
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/catalog/categories", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestCreateAndGetProduct(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := newFakeRepo()
	_, err := repo.CreateCategory(context.Background(), catalog.Category{Name: "Kids", Slug: "kids", IsActive: true})
	if err != nil {
		t.Fatalf("seed category: %v", err)
	}

	service := catalogservice.New(repo, nil)
	handler := New(service, nil)
	router := gin.New()
	handler.RegisterRoutes(router)

	createBody := map[string]any{
		"category_id": 1,
		"name":        "Trail Tee",
		"slug":        "trail-tee",
		"status":      "active",
		"skus": []map[string]any{
			{
				"sku_code":       "TRAIL-TEE-BLK-M",
				"size":           "M",
				"color":          "Black",
				"mrp_amount":     149900,
				"selling_amount": 119900,
				"currency":       "INR",
			},
		},
	}
	data, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest(http.MethodPost, "/catalog/products", bytes.NewReader(data))
	createReq.Header.Set("Content-Type", "application/json")
	createRR := httptest.NewRecorder()
	router.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", createRR.Code, createRR.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/catalog/products/1", nil)
	getRR := httptest.NewRecorder()
	router.ServeHTTP(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", getRR.Code, getRR.Body.String())
	}
}
