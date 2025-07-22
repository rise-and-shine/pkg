package pagination_test

import (
	"fmt"

	"github.com/rise-and-shine/pkg/pagination"
)

// Example demonstrates how to use pagination with embedding for APIs

// UserListRequest shows how to embed pagination.Params in a request struct
type UserListRequest struct {
	pagination.Params        // Embedded pagination parameters
	Name              string `query:"name" json:"name,omitempty"`
	Active            *bool  `query:"active" json:"active,omitempty"`
	DepartmentID      int64  `query:"department_id" json:"department_id,omitempty"`
}

// User represents a user entity
type User struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Active       bool   `json:"active"`
	DepartmentID int64  `json:"department_id"`
}

// UserListResponse shows how to embed pagination.Response in a response struct
type UserListResponse struct {
	pagination.Response        // Embedded pagination metadata
	Users               []User `json:"users"`
}

// Example_usage demonstrates typical usage patterns
func Example_usage() {
	// 1. Parse request with embedded pagination
	req := UserListRequest{
		Params: pagination.Params{
			Page: 2,
			Size: 10,
		},
		Name:   "john",
		Active: boolPtr(true),
	}

	// 2. Normalize pagination parameters
	req.Params.Normalize(pagination.DefaultConfig())

	// 3. Use pagination for database query
	limit, offset := req.ToLimitOffset()
	fmt.Printf("Query with LIMIT %d OFFSET %d\n", limit, offset)

	// 4. Simulate fetching data
	users := []User{
		{ID: 11, Name: "John Doe", Email: "john@example.com", Active: true, DepartmentID: 1},
		{ID: 12, Name: "John Smith", Email: "smith@example.com", Active: true, DepartmentID: 2},
	}
	totalCount := int64(25) // Total matching records

	// 5. Create response with embedded pagination
	response := UserListResponse{
		Response: req.NewResponse(totalCount),
		Users:    users,
	}

	// 6. Output information
	fmt.Printf("Pagination info: %s\n", response.Response.String())

	// Output:
	// Query with LIMIT 10 OFFSET 10
	// Pagination info: page 2 of 3 (total: 25, size: 10)
}

// Example_differentApproaches shows different ways to use pagination
func Example_differentApproaches() {
	cfg := pagination.DefaultConfig()

	// Approach 1: Using page/size
	fmt.Println("=== Page/Size Approach ===")
	pageParams := pagination.Params{Page: 3, Size: 15}
	pageParams.Normalize(cfg)
	fmt.Printf("Input: %s\n", pageParams.String())
	limit, offset := pageParams.ToLimitOffset()
	fmt.Printf("For SQL: LIMIT %d OFFSET %d\n", limit, offset)

	// Approach 2: Using limit/offset
	fmt.Println("\n=== Limit/Offset Approach ===")
	limitParams := pagination.Params{Limit: 20, Offset: 40}
	limitParams.Normalize(cfg)
	fmt.Printf("Input: %s\n", limitParams.String())
	page, size := limitParams.ToPageSize()
	fmt.Printf("Equivalent page/size: page %d, size %d\n", page, size)

	// Approach 3: Using defaults
	fmt.Println("\n=== Using Defaults ===")
	defaultParams := pagination.Params{}
	defaultParams.Normalize(cfg)
	fmt.Printf("Normalized: %s\n", defaultParams.String())

	// Output:
	// === Page/Size Approach ===
	// Input: page=3 size=15
	// For SQL: LIMIT 15 OFFSET 30
	//
	// === Limit/Offset Approach ===
	// Input: limit=20 offset=40
	// Equivalent page/size: page 3, size 20
	//
	// === Using Defaults ===
	// Normalized: page=1 size=20
}

// Example_customConfig shows how to use custom pagination configuration
func Example_customConfig() {
	// Create custom configuration
	customCfg := pagination.Config{
		DefaultLimit: 50,
		MaxLimit:     200,
		DefaultSize:  50,
		MaxSize:      200,
	}

	params := pagination.Params{Size: 300} // Exceeds max
	params.Normalize(customCfg)

	fmt.Printf("Custom config applied: %s\n", params.String())
	fmt.Printf("Size was capped at: %d\n", params.Size)

	// Output:
	// Custom config applied: page=1 size=200
	// Size was capped at: 200
}

// Example_withDatabase shows a more realistic database integration example
func Example_withDatabase() {
	type ProductListRequest struct {
		pagination.Params
		Category string  `query:"category"`
		MinPrice float64 `query:"min_price"`
		MaxPrice float64 `query:"max_price"`
	}

	type Product struct {
		ID       int64   `json:"id"`
		Name     string  `json:"name"`
		Category string  `json:"category"`
		Price    float64 `json:"price"`
	}

	type ProductListResponse struct {
		pagination.Response
		Products []Product `json:"products"`
	}

	// Simulate incoming request
	req := ProductListRequest{
		Params:   pagination.Params{Page: 1, Size: 5},
		Category: "electronics",
		MinPrice: 100.0,
		MaxPrice: 1000.0,
	}

	req.Params.Normalize(pagination.DefaultConfig())

	// This is where you'd typically build your database query
	limit, offset := req.ToLimitOffset()
	fmt.Printf("Database query would use: LIMIT %d OFFSET %d\n", limit, offset)

	// Simulate database results
	products := []Product{
		{ID: 1, Name: "Laptop", Category: "electronics", Price: 899.99},
		{ID: 2, Name: "Phone", Category: "electronics", Price: 599.99},
		{ID: 3, Name: "Tablet", Category: "electronics", Price: 399.99},
	}
	totalCount := int64(15) // Total matching products

	// Create paginated response
	response := ProductListResponse{
		Response: req.NewResponse(totalCount),
		Products: products,
	}

	fmt.Printf("Response summary: %s\n", response.Response.String())
	fmt.Printf("Has next page: %t\n", response.HasNext)

	// Output:
	// Database query would use: LIMIT 5 OFFSET 0
	// Response summary: page 1 of 3 (total: 15, size: 5)
	// Has next page: true
}

// Helper function
func boolPtr(b bool) *bool {
	return &b
}
