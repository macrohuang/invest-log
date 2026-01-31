package investlog

import (
	"database/sql"
	"fmt"
	"strings"
)

// GetAssetTypes returns all asset types.
func (c *Core) GetAssetTypes() ([]AssetType, error) {
	rows, err := c.db.Query("SELECT id, code, label, created_at FROM asset_types ORDER BY code")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var types []AssetType
	for rows.Next() {
		var at AssetType
		var createdAt sql.NullString
		if err := rows.Scan(&at.ID, &at.Code, &at.Label, &createdAt); err != nil {
			return nil, err
		}
		if createdAt.Valid {
			at.CreatedAt = createdAt.String
		}
		types = append(types, at)
	}
	return types, rows.Err()
}

// GetAssetTypeLabels returns a code->label map.
func (c *Core) GetAssetTypeLabels() (map[string]string, error) {
	types, err := c.GetAssetTypes()
	if err != nil {
		return nil, err
	}
	labels := make(map[string]string, len(types))
	for _, t := range types {
		labels[t.Code] = t.Label
	}
	return labels, nil
}

// AddAssetType adds a new asset type.
func (c *Core) AddAssetType(code, label string) (bool, error) {
	code = strings.ToLower(strings.TrimSpace(code))
	label = strings.TrimSpace(label)
	if code == "" || label == "" {
		return false, fmt.Errorf("code and label required")
	}
	_, err := c.db.Exec("INSERT INTO asset_types (code, label) VALUES (?, ?)", code, label)
	if err != nil {
		return false, err
	}
	return true, nil
}

// CheckAssetTypeInUse returns true if any symbols use the asset type.
func (c *Core) CheckAssetTypeInUse(code string) (bool, error) {
	code = strings.ToLower(strings.TrimSpace(code))
	var count int
	if err := c.db.QueryRow("SELECT COUNT(*) FROM symbols WHERE asset_type = ?", code).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// CanDeleteAssetType checks whether an asset type can be deleted.
func (c *Core) CanDeleteAssetType(code string) (bool, string, error) {
	inUse, err := c.CheckAssetTypeInUse(code)
	if err != nil {
		return false, "", err
	}
	if inUse {
		return false, "Cannot delete: symbols use this asset type", nil
	}
	return true, "Can be deleted", nil
}

// DeleteAssetType deletes an asset type if unused.
func (c *Core) DeleteAssetType(code string) (bool, string, error) {
	canDelete, message, err := c.CanDeleteAssetType(code)
	if err != nil {
		return false, "", err
	}
	if !canDelete {
		return false, message, nil
	}
	code = strings.ToLower(strings.TrimSpace(code))
	result, err := c.db.Exec("DELETE FROM asset_types WHERE code = ?", code)
	if err != nil {
		return false, "", err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, "", err
	}
	if rows > 0 {
		return true, "Asset type deleted", nil
	}
	return false, "Asset type not found", nil
}
