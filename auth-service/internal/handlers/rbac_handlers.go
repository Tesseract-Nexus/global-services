package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"auth-service/internal/middleware"
	"auth-service/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type RBACHandlers struct {
	db *sql.DB
}

func NewRBACHandlers(db *sql.DB) *RBACHandlers {
	return &RBACHandlers{
		db: db,
	}
}

// Role Management

// ListRoles retrieves all roles for a tenant
func (h *RBACHandlers) ListRoles(c *gin.Context) {
	tenantID, err := middleware.GetTenantID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Tenant context not found",
		})
		return
	}

	query := `
		SELECT r.id, r.name, r.description, r.tenant_id, r.is_system, r.created_at, r.updated_at
		FROM roles r
		WHERE r.tenant_id = $1
		ORDER BY r.created_at DESC
	`

	rows, err := h.db.Query(query, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch roles",
			"details": err.Error(),
		})
		return
	}
	defer rows.Close()

	var roles []models.Role
	for rows.Next() {
		var role models.Role
		err := rows.Scan(&role.ID, &role.Name, &role.Description, &role.TenantID,
			&role.IsSystem, &role.CreatedAt, &role.UpdatedAt)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to scan role",
				"details": err.Error(),
			})
			return
		}

		// Get permissions for this role
		permissions, _ := h.getRolePermissions(role.ID)
		role.Permissions = permissions

		roles = append(roles, role)
	}

	c.JSON(http.StatusOK, gin.H{
		"roles": roles,
	})
}

// GetRole retrieves a specific role with its permissions
func (h *RBACHandlers) GetRole(c *gin.Context) {
	roleIDStr := c.Param("role_id")
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid role ID format",
		})
		return
	}

	query := `
		SELECT id, name, description, tenant_id, is_system, created_at, updated_at
		FROM roles WHERE id = $1
	`

	var role models.Role
	err = h.db.QueryRow(query, roleID).Scan(
		&role.ID, &role.Name, &role.Description, &role.TenantID,
		&role.IsSystem, &role.CreatedAt, &role.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Role not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch role",
			"details": err.Error(),
		})
		return
	}

	// Get permissions for this role
	permissions, err := h.getRolePermissions(roleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch role permissions",
			"details": err.Error(),
		})
		return
	}
	role.Permissions = permissions

	c.JSON(http.StatusOK, gin.H{
		"role": role,
	})
}

// CreateRole creates a new custom role
func (h *RBACHandlers) CreateRole(c *gin.Context) {
	var req struct {
		Name        string   `json:"name" binding:"required"`
		Description string   `json:"description"`
		Permissions []string `json:"permissions"` // Permission IDs
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	tenantID, err := middleware.GetTenantID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Tenant context not found",
		})
		return
	}

	// Check if role name already exists for this tenant
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM roles WHERE name = $1 AND tenant_id = $2)`
	err = h.db.QueryRow(checkQuery, req.Name, tenantID).Scan(&exists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to check role existence",
			"details": err.Error(),
		})
		return
	}

	if exists {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Role with this name already exists",
		})
		return
	}

	// Create role
	roleID := uuid.New()
	now := time.Now()

	insertQuery := `
		INSERT INTO roles (id, name, description, tenant_id, is_system, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err = h.db.Exec(insertQuery, roleID, req.Name, req.Description, tenantID, false, now, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create role",
			"details": err.Error(),
		})
		return
	}

	// Assign permissions if provided
	if len(req.Permissions) > 0 {
		for _, permIDStr := range req.Permissions {
			permID, err := uuid.Parse(permIDStr)
			if err != nil {
				continue // Skip invalid UUIDs
			}

			permQuery := `
				INSERT INTO role_permissions (id, role_id, permission_id, created_at)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (role_id, permission_id) DO NOTHING
			`
			h.db.Exec(permQuery, uuid.New(), roleID, permID, now)
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Role created successfully",
		"role_id": roleID,
	})
}

// UpdateRole updates an existing role
func (h *RBACHandlers) UpdateRole(c *gin.Context) {
	roleIDStr := c.Param("role_id")
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid role ID format",
		})
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Check if role is system role
	var isSystem bool
	checkQuery := `SELECT is_system FROM roles WHERE id = $1`
	err = h.db.QueryRow(checkQuery, roleID).Scan(&isSystem)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Role not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to check role",
			"details": err.Error(),
		})
		return
	}

	if isSystem {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Cannot modify system role",
		})
		return
	}

	// Update role
	updateQuery := `
		UPDATE roles
		SET name = $2, description = $3, updated_at = $4
		WHERE id = $1
	`

	_, err = h.db.Exec(updateQuery, roleID, req.Name, req.Description, time.Now())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update role",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Role updated successfully",
	})
}

// DeleteRole deletes a custom role
func (h *RBACHandlers) DeleteRole(c *gin.Context) {
	roleIDStr := c.Param("role_id")
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid role ID format",
		})
		return
	}

	// Check if role is system role
	var isSystem bool
	checkQuery := `SELECT is_system FROM roles WHERE id = $1`
	err = h.db.QueryRow(checkQuery, roleID).Scan(&isSystem)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Role not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to check role",
			"details": err.Error(),
		})
		return
	}

	if isSystem {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Cannot delete system role",
		})
		return
	}

	// Delete role (cascade will remove role_permissions and user_roles)
	deleteQuery := `DELETE FROM roles WHERE id = $1`
	_, err = h.db.Exec(deleteQuery, roleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete role",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Role deleted successfully",
	})
}

// Permission Management

// ListPermissions retrieves all permissions
func (h *RBACHandlers) ListPermissions(c *gin.Context) {
	query := `
		SELECT id, name, resource, action, description, is_system, created_at, updated_at
		FROM permissions
		ORDER BY resource, action
	`

	rows, err := h.db.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch permissions",
			"details": err.Error(),
		})
		return
	}
	defer rows.Close()

	var permissions []models.Permission
	for rows.Next() {
		var perm models.Permission
		err := rows.Scan(&perm.ID, &perm.Name, &perm.Resource, &perm.Action,
			&perm.Description, &perm.IsSystem, &perm.CreatedAt, &perm.UpdatedAt)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to scan permission",
				"details": err.Error(),
			})
			return
		}
		permissions = append(permissions, perm)
	}

	c.JSON(http.StatusOK, gin.H{
		"permissions": permissions,
	})
}

// GetRolePermissions retrieves permissions for a specific role
func (h *RBACHandlers) GetRolePermissions(c *gin.Context) {
	roleIDStr := c.Param("role_id")
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid role ID format",
		})
		return
	}

	permissions, err := h.getRolePermissions(roleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch role permissions",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"permissions": permissions,
	})
}

// AssignPermissionToRole assigns a permission to a role
func (h *RBACHandlers) AssignPermissionToRole(c *gin.Context) {
	roleIDStr := c.Param("role_id")
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid role ID format",
		})
		return
	}

	var req struct {
		PermissionID string `json:"permission_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	permID, err := uuid.Parse(req.PermissionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid permission ID format",
		})
		return
	}

	// Check if role is system role
	var isSystem bool
	checkQuery := `SELECT is_system FROM roles WHERE id = $1`
	err = h.db.QueryRow(checkQuery, roleID).Scan(&isSystem)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Role not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to check role",
			"details": err.Error(),
		})
		return
	}

	if isSystem {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Cannot modify permissions for system role",
		})
		return
	}

	// Assign permission to role
	insertQuery := `
		INSERT INTO role_permissions (id, role_id, permission_id, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (role_id, permission_id) DO NOTHING
	`

	_, err = h.db.Exec(insertQuery, uuid.New(), roleID, permID, time.Now())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to assign permission",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Permission assigned successfully",
	})
}

// RemovePermissionFromRole removes a permission from a role
func (h *RBACHandlers) RemovePermissionFromRole(c *gin.Context) {
	roleIDStr := c.Param("role_id")
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid role ID format",
		})
		return
	}

	var req struct {
		PermissionID string `json:"permission_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	permID, err := uuid.Parse(req.PermissionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid permission ID format",
		})
		return
	}

	// Check if role is system role
	var isSystem bool
	checkQuery := `SELECT is_system FROM roles WHERE id = $1`
	err = h.db.QueryRow(checkQuery, roleID).Scan(&isSystem)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Role not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to check role",
			"details": err.Error(),
		})
		return
	}

	if isSystem {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Cannot modify permissions for system role",
		})
		return
	}

	// Remove permission from role
	deleteQuery := `DELETE FROM role_permissions WHERE role_id = $1 AND permission_id = $2`
	_, err = h.db.Exec(deleteQuery, roleID, permID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to remove permission",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Permission removed successfully",
	})
}

// Helper function to get role permissions
func (h *RBACHandlers) getRolePermissions(roleID uuid.UUID) ([]models.Permission, error) {
	query := `
		SELECT p.id, p.name, p.resource, p.action, p.description, p.is_system, p.created_at, p.updated_at
		FROM permissions p
		INNER JOIN role_permissions rp ON p.id = rp.permission_id
		WHERE rp.role_id = $1
		ORDER BY p.resource, p.action
	`

	rows, err := h.db.Query(query, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []models.Permission
	for rows.Next() {
		var perm models.Permission
		err := rows.Scan(&perm.ID, &perm.Name, &perm.Resource, &perm.Action,
			&perm.Description, &perm.IsSystem, &perm.CreatedAt, &perm.UpdatedAt)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, perm)
	}

	return permissions, nil
}

// GetUserRoles retrieves all roles for a specific user
func (h *RBACHandlers) GetUserRoles(c *gin.Context) {
	userIDStr := c.Param("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user ID format",
		})
		return
	}

	query := `
		SELECT r.id, r.name, r.description, r.tenant_id, r.is_system, r.created_at, r.updated_at
		FROM roles r
		INNER JOIN user_roles ur ON r.id = ur.role_id
		WHERE ur.user_id = $1
	`

	rows, err := h.db.Query(query, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch user roles",
			"details": err.Error(),
		})
		return
	}
	defer rows.Close()

	var roles []models.Role
	for rows.Next() {
		var role models.Role
		err := rows.Scan(&role.ID, &role.Name, &role.Description, &role.TenantID,
			&role.IsSystem, &role.CreatedAt, &role.UpdatedAt)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to scan role",
				"details": err.Error(),
			})
			return
		}
		roles = append(roles, role)
	}

	c.JSON(http.StatusOK, gin.H{
		"roles": roles,
	})
}
