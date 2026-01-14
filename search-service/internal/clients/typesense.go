package clients

import (
	"context"
	"fmt"
	"time"

	"github.com/tesseract-hub/search-service/internal/config"
	"github.com/typesense/typesense-go/v2/typesense"
	"github.com/typesense/typesense-go/v2/typesense/api"
)

// TypesenseClient wraps the Typesense client with convenience methods
type TypesenseClient struct {
	client *typesense.Client
	config *config.Config
}

// NewTypesenseClient creates a new Typesense client
func NewTypesenseClient(cfg *config.Config) (*TypesenseClient, error) {
	client := typesense.NewClient(
		typesense.WithServer(fmt.Sprintf("%s://%s:%d", cfg.TypesenseProtocol, cfg.TypesenseHost, cfg.TypesensePort)),
		typesense.WithAPIKey(cfg.TypesenseAPIKey),
		typesense.WithConnectionTimeout(time.Duration(cfg.SearchTimeout)*time.Second),
	)

	return &TypesenseClient{
		client: client,
		config: cfg,
	}, nil
}

// GetClient returns the underlying Typesense client
func (tc *TypesenseClient) GetClient() *typesense.Client {
	return tc.client
}

// Health checks if Typesense is healthy
func (tc *TypesenseClient) Health(ctx context.Context) error {
	_, err := tc.client.Health(ctx, time.Duration(tc.config.SearchTimeout)*time.Second)
	return err
}

// Search performs a search on a collection with tenant filtering
func (tc *TypesenseClient) Search(ctx context.Context, collection string, params *api.SearchCollectionParams) (*api.SearchResult, error) {
	return tc.client.Collection(collection).Documents().Search(ctx, params)
}

// MultiSearch performs multiple searches in parallel
func (tc *TypesenseClient) MultiSearch(ctx context.Context, params *api.MultiSearchParams, searches api.MultiSearchSearchesParameter) (*api.MultiSearchResult, error) {
	return tc.client.MultiSearch.Perform(ctx, params, searches)
}

// IndexDocument indexes a single document
func (tc *TypesenseClient) IndexDocument(ctx context.Context, collection string, document interface{}) (map[string]interface{}, error) {
	return tc.client.Collection(collection).Documents().Create(ctx, document)
}

// UpsertDocument upserts a document (create or update)
func (tc *TypesenseClient) UpsertDocument(ctx context.Context, collection string, document interface{}) (map[string]interface{}, error) {
	return tc.client.Collection(collection).Documents().Upsert(ctx, document)
}

// UpdateDocument updates an existing document
func (tc *TypesenseClient) UpdateDocument(ctx context.Context, collection string, id string, document interface{}) (map[string]interface{}, error) {
	return tc.client.Collection(collection).Document(id).Update(ctx, document)
}

// DeleteDocument deletes a document by ID
func (tc *TypesenseClient) DeleteDocument(ctx context.Context, collection string, id string) (map[string]interface{}, error) {
	return tc.client.Collection(collection).Document(id).Delete(ctx)
}

// DeleteByQuery deletes documents matching a filter
func (tc *TypesenseClient) DeleteByQuery(ctx context.Context, collection string, filter string) (int, error) {
	params := &api.DeleteDocumentsParams{
		FilterBy: &filter,
	}
	result, err := tc.client.Collection(collection).Documents().Delete(ctx, params)
	if err != nil {
		return 0, err
	}
	return result, nil
}

// ImportDocuments imports multiple documents
func (tc *TypesenseClient) ImportDocuments(ctx context.Context, collection string, documents []interface{}, action string) ([]*api.ImportDocumentResponse, error) {
	params := &api.ImportDocumentsParams{
		Action: &action,
	}
	return tc.client.Collection(collection).Documents().Import(ctx, documents, params)
}

// ListCollections returns all collections
func (tc *TypesenseClient) ListCollections(ctx context.Context) ([]*api.CollectionResponse, error) {
	return tc.client.Collections().Retrieve(ctx)
}

// GetCollection returns a specific collection
func (tc *TypesenseClient) GetCollection(ctx context.Context, name string) (*api.CollectionResponse, error) {
	return tc.client.Collection(name).Retrieve(ctx)
}

// CreateCollection creates a new collection
func (tc *TypesenseClient) CreateCollection(ctx context.Context, schema *api.CollectionSchema) (*api.CollectionResponse, error) {
	return tc.client.Collections().Create(ctx, schema)
}

// DeleteCollection deletes a collection
func (tc *TypesenseClient) DeleteCollection(ctx context.Context, name string) (*api.CollectionResponse, error) {
	return tc.client.Collection(name).Delete(ctx)
}

// Collection schemas for initialization
var (
	ProductsSchema = &api.CollectionSchema{
		Name: "products",
		Fields: []api.Field{
			{Name: "tenant_id", Type: "string", Facet: pointer(true)},
			{Name: "name", Type: "string"},
			{Name: "description", Type: "string", Optional: pointer(true)},
			{Name: "sku", Type: "string"},
			{Name: "brand", Type: "string", Optional: pointer(true), Facet: pointer(true)},
			{Name: "price", Type: "float"},
			{Name: "sale_price", Type: "float", Optional: pointer(true)},
			{Name: "currency", Type: "string", Facet: pointer(true)},
			{Name: "category", Type: "string[]", Facet: pointer(true)},
			{Name: "tags", Type: "string[]", Optional: pointer(true), Facet: pointer(true)},
			{Name: "in_stock", Type: "bool", Facet: pointer(true)},
			{Name: "image_url", Type: "string", Optional: pointer(true)},
			{Name: "created_at", Type: "int64"},
			{Name: "updated_at", Type: "int64"},
		},
		DefaultSortingField: pointer("created_at"),
	}

	CustomersSchema = &api.CollectionSchema{
		Name: "customers",
		Fields: []api.Field{
			{Name: "tenant_id", Type: "string", Facet: pointer(true)},
			{Name: "name", Type: "string"},
			{Name: "email", Type: "string"},
			{Name: "phone", Type: "string", Optional: pointer(true)},
			{Name: "company", Type: "string", Optional: pointer(true)},
			{Name: "total_orders", Type: "int32", Optional: pointer(true)},
			{Name: "total_spent", Type: "float", Optional: pointer(true)},
			{Name: "status", Type: "string", Facet: pointer(true)},
			{Name: "created_at", Type: "int64"},
		},
		DefaultSortingField: pointer("created_at"),
	}

	OrdersSchema = &api.CollectionSchema{
		Name: "orders",
		Fields: []api.Field{
			{Name: "tenant_id", Type: "string", Facet: pointer(true)},
			{Name: "order_number", Type: "string"},
			{Name: "customer_name", Type: "string"},
			{Name: "customer_email", Type: "string"},
			{Name: "total", Type: "float"},
			{Name: "currency", Type: "string", Facet: pointer(true)},
			{Name: "status", Type: "string", Facet: pointer(true)},
			{Name: "items", Type: "string[]"},
			{Name: "created_at", Type: "int64"},
		},
		DefaultSortingField: pointer("created_at"),
	}

	CategoriesSchema = &api.CollectionSchema{
		Name: "categories",
		Fields: []api.Field{
			{Name: "tenant_id", Type: "string", Facet: pointer(true)},
			{Name: "name", Type: "string"},
			{Name: "slug", Type: "string"},
			{Name: "description", Type: "string", Optional: pointer(true)},
			{Name: "parent_id", Type: "string", Optional: pointer(true)},
			{Name: "level", Type: "int32"},
			{Name: "product_count", Type: "int32", Optional: pointer(true)},
		},
	}
)

func pointer[T any](v T) *T {
	return &v
}
