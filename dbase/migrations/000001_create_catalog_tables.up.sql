CREATE TABLE IF NOT EXISTS catalog_categories (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    parent_id BIGINT NULL REFERENCES catalog_categories(id) ON DELETE SET NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_catalog_categories_parent_id ON catalog_categories(parent_id);
CREATE INDEX IF NOT EXISTS idx_catalog_categories_is_active ON catalog_categories(is_active);
CREATE INDEX IF NOT EXISTS idx_catalog_categories_deleted_at ON catalog_categories(deleted_at);

CREATE TABLE IF NOT EXISTS catalog_products (
    id BIGSERIAL PRIMARY KEY,
    category_id BIGINT NOT NULL REFERENCES catalog_categories(id),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    fabric_summary TEXT NOT NULL DEFAULT '',
    wash_care TEXT NOT NULL DEFAULT '',
    durability_notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL,
    CONSTRAINT chk_catalog_products_status
        CHECK (status IN ('draft', 'active', 'inactive', 'archived'))
);

CREATE INDEX IF NOT EXISTS idx_catalog_products_category_id ON catalog_products(category_id);
CREATE INDEX IF NOT EXISTS idx_catalog_products_status ON catalog_products(status);
CREATE INDEX IF NOT EXISTS idx_catalog_products_created_at ON catalog_products(created_at);
CREATE INDEX IF NOT EXISTS idx_catalog_products_deleted_at ON catalog_products(deleted_at);

CREATE TABLE IF NOT EXISTS catalog_product_images (
    id BIGSERIAL PRIMARY KEY,
    product_id BIGINT NOT NULL REFERENCES catalog_products(id) ON DELETE CASCADE,
    url VARCHAR(1024) NOT NULL,
    alt_text VARCHAR(255) NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_catalog_product_images_product_id ON catalog_product_images(product_id);
CREATE INDEX IF NOT EXISTS idx_catalog_product_images_is_primary ON catalog_product_images(is_primary);

CREATE TABLE IF NOT EXISTS catalog_skus (
    id BIGSERIAL PRIMARY KEY,
    product_id BIGINT NOT NULL REFERENCES catalog_products(id) ON DELETE CASCADE,
    sku_code VARCHAR(100) NOT NULL UNIQUE,
    size VARCHAR(50) NOT NULL,
    color VARCHAR(50) NOT NULL,
    mrp_amount BIGINT NOT NULL,
    selling_amount BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'INR',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_catalog_skus_amounts_positive CHECK (mrp_amount >= 0 AND selling_amount >= 0),
    CONSTRAINT chk_catalog_skus_selling_lte_mrp CHECK (selling_amount <= mrp_amount)
);

CREATE INDEX IF NOT EXISTS idx_catalog_skus_product_id ON catalog_skus(product_id);
CREATE INDEX IF NOT EXISTS idx_catalog_skus_size ON catalog_skus(size);
CREATE INDEX IF NOT EXISTS idx_catalog_skus_color ON catalog_skus(color);
CREATE INDEX IF NOT EXISTS idx_catalog_skus_is_active ON catalog_skus(is_active);

CREATE TABLE IF NOT EXISTS catalog_product_attributes (
    id BIGSERIAL PRIMARY KEY,
    product_id BIGINT NOT NULL REFERENCES catalog_products(id) ON DELETE CASCADE,
    type VARCHAR(30) NOT NULL,
    name VARCHAR(100) NOT NULL,
    value TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_catalog_product_attributes_type
        CHECK (type IN ('fabric', 'performance', 'fit', 'care')),
    CONSTRAINT uq_catalog_product_attributes UNIQUE (product_id, type, name)
);

CREATE INDEX IF NOT EXISTS idx_catalog_product_attributes_product_id ON catalog_product_attributes(product_id);
CREATE INDEX IF NOT EXISTS idx_catalog_product_attributes_type ON catalog_product_attributes(type);

CREATE TABLE IF NOT EXISTS catalog_bulk_upload_jobs (
    id BIGSERIAL PRIMARY KEY,
    created_by BIGINT NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    total_rows INTEGER NOT NULL DEFAULT 0,
    success_rows INTEGER NOT NULL DEFAULT 0,
    failed_rows INTEGER NOT NULL DEFAULT 0,
    error_report_url VARCHAR(1024) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_catalog_bulk_upload_jobs_status
        CHECK (status IN ('pending', 'processing', 'completed', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_catalog_bulk_upload_jobs_created_by ON catalog_bulk_upload_jobs(created_by);
CREATE INDEX IF NOT EXISTS idx_catalog_bulk_upload_jobs_status ON catalog_bulk_upload_jobs(status);
