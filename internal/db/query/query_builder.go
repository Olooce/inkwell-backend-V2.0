package query

import (
	"fmt"
	"strings"
)

type QueryBuilder struct {
	query      strings.Builder
	table      string
	conditions []string
	columns    []string
	values     []interface{}
}

func NewQueryBuilder() *QueryBuilder {
	return &QueryBuilder{}
}

func (qb *QueryBuilder) Select(columns ...string) *QueryBuilder {
	qb.columns = append(qb.columns, columns...)
	return qb
}

func (qb *QueryBuilder) From(table string) *QueryBuilder {
	qb.table = table
	return qb
}

func (qb *QueryBuilder) Where(condition string, args ...interface{}) *QueryBuilder {
	qb.conditions = append(qb.conditions, condition)
	qb.values = append(qb.values, args...)
	return qb
}

func (qb *QueryBuilder) InsertInto(table string, columns ...string) *QueryBuilder {
	qb.table = table
	qb.columns = columns
	return qb
}

func (qb *QueryBuilder) Values(vals ...interface{}) *QueryBuilder {
	qb.values = vals
	return qb
}

func (qb *QueryBuilder) Update(table string) *QueryBuilder {
	qb.table = table
	qb.query.WriteString(fmt.Sprintf("UPDATE %s SET ", table))
	return qb
}

func (qb *QueryBuilder) Set(assignments map[string]interface{}) *QueryBuilder {
	sets := []string{}
	for col, val := range assignments {
		sets = append(sets, fmt.Sprintf("%s = ?", col))
		qb.values = append(qb.values, val)
	}
	qb.query.WriteString(strings.Join(sets, ", "))
	return qb
}

func (qb *QueryBuilder) DeleteFrom(table string) *QueryBuilder {
	qb.table = table
	qb.query.WriteString(fmt.Sprintf("DELETE FROM %s ", table))
	return qb
}

func (qb *QueryBuilder) Build() (string, []interface{}) {
	if qb.query.Len() > 0 {
		if len(qb.conditions) > 0 {
			qb.query.WriteString(" WHERE " + strings.Join(qb.conditions, " AND "))
		}
		return qb.query.String(), qb.values
	}

	if len(qb.columns) > 0 {
		qb.query.WriteString(fmt.Sprintf("SELECT %s FROM %s", strings.Join(qb.columns, ", "), qb.table))
	} else {
		qb.query.WriteString(fmt.Sprintf("SELECT * FROM %s", qb.table))
	}

	if len(qb.conditions) > 0 {
		qb.query.WriteString(" WHERE " + strings.Join(qb.conditions, " AND "))
	}

	return qb.query.String(), qb.values
}
