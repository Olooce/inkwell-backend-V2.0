package db

import (
	"errors"
	"gorm.io/gorm"
)

// QueryExecutor handles database queries.
type QueryExecutor struct {
	DB *gorm.DB
}

// NewQueryExecutor creates a new instance of QueryExecutor.
func NewQueryExecutor(db *gorm.DB) *QueryExecutor {
	return &QueryExecutor{DB: db}
}

// IsFieldInTable checks if a field exists in a given table.
func (qe *QueryExecutor) IsFieldInTable(tableName, fieldName string) (bool, error) {
	var exists bool
	query := `SELECT COUNT(*) > 0 FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_NAME = ? AND COLUMN_NAME = ?`
	if err := qe.DB.Raw(query, tableName, fieldName).Scan(&exists).Error; err != nil {
		return false, err
	}
	return exists, nil
}

// Insert executes an insert query and returns the last inserted ID.
func (qe *QueryExecutor) Insert(table string, data map[string]interface{}) (uint, error) {
	result := qe.DB.Table(table).Create(&data)
	if result.Error != nil {
		return 0, result.Error
	}
	id, ok := data["id"].(uint)
	if !ok {
		return 0, errors.New("failed to retrieve last insert ID")
	}
	return id, nil
}

// Select executes a raw select query and returns the results.
func (qe *QueryExecutor) Select(query string, args ...interface{}) ([]map[string]interface{}, error) {
	rows, err := qe.DB.Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []map[string]interface{}{}
	cols, _ := rows.Columns()
	scanArgs := make([]interface{}, len(cols))
	for rows.Next() {
		rowData := make([]interface{}, len(cols))
		for i := range rowData {
			scanArgs[i] = &rowData[i]
		}
		if err := rows.Scan(scanArgs...); err != nil {
			return nil, err
		}
		record := make(map[string]interface{})
		for i, col := range cols {
			record[col] = rowData[i]
		}
		results = append(results, record)
	}
	return results, nil
}

// Update executes an update query with conditions.
func (qe *QueryExecutor) Update(table string, conditions map[string]interface{}, updates map[string]interface{}) error {
	result := qe.DB.Table(table).Where(conditions).Updates(updates)
	return result.Error
}

// Delete executes a delete query with conditions.
func (qe *QueryExecutor) Delete(table string, conditions map[string]interface{}) error {
	result := qe.DB.Table(table).Where(conditions).Delete(nil)
	return result.Error
}

// Count returns the number of rows that match the given conditions.
func (qe *QueryExecutor) Count(table string, conditions map[string]interface{}) (int64, error) {
	var count int64
	result := qe.DB.Table(table).Where(conditions).Count(&count)
	return count, result.Error
}

// Exists checks if a record matching the conditions exists.
func (qe *QueryExecutor) Exists(table string, conditions map[string]interface{}) (bool, error) {
	var exists bool
	query := `SELECT EXISTS (SELECT 1 FROM ` + table + ` WHERE ? LIMIT 1)`
	if err := qe.DB.Raw(query, conditions).Scan(&exists).Error; err != nil {
		return false, err
	}
	return exists, nil
}

// Transaction executes a set of operations within a database transaction.
func (qe *QueryExecutor) Transaction(txFunc func(tx *gorm.DB) error) error {
	return qe.DB.Transaction(txFunc)
}

// RawExec executes a raw SQL command.
func (qe *QueryExecutor) RawExec(query string, args ...interface{}) error {
	return qe.DB.Exec(query, args...).Error
}
