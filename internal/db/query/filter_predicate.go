package query

import "fmt"

type FilterPredicate struct {
	predicate string
}

func NewFilterPredicate() *FilterPredicate {
	return &FilterPredicate{}
}

func (fp *FilterPredicate) Open() *FilterPredicate {
	fp.predicate += "("
	return fp
}

func (fp *FilterPredicate) Close() *FilterPredicate {
	fp.predicate += ")"
	return fp
}

func (fp *FilterPredicate) And() *FilterPredicate {
	fp.predicate += " AND "
	return fp
}

func (fp *FilterPredicate) Or() *FilterPredicate {
	fp.predicate += " OR "
	return fp
}

func (fp *FilterPredicate) Not() *FilterPredicate {
	fp.predicate += " NOT "
	return fp
}

func (fp *FilterPredicate) Equal(column string, value interface{}) *FilterPredicate {
	fp.predicate += fmt.Sprintf("%s = '%v'", column, value)
	return fp
}

func (fp *FilterPredicate) NotEqual(column string, value interface{}) *FilterPredicate {
	fp.predicate += fmt.Sprintf("%s <> '%v'", column, value)
	return fp
}

func (fp *FilterPredicate) GreaterThan(column string, value interface{}) *FilterPredicate {
	fp.predicate += fmt.Sprintf("%s > '%v'", column, value)
	return fp
}

func (fp *FilterPredicate) LessThan(column string, value interface{}) *FilterPredicate {
	fp.predicate += fmt.Sprintf("%s < '%v'", column, value)
	return fp
}

func (fp *FilterPredicate) Between(column string, v1, v2 interface{}) *FilterPredicate {
	fp.predicate += fmt.Sprintf("%s BETWEEN '%v' AND '%v'", column, v1, v2)
	return fp
}

func (fp *FilterPredicate) In(column string, values ...interface{}) *FilterPredicate {
	fp.predicate += fmt.Sprintf("%s IN (%v)", column, values)
	return fp
}

func (fp *FilterPredicate) Like(column, pattern string) *FilterPredicate {
	fp.predicate += fmt.Sprintf("%s LIKE '%%%s%%'", column, pattern)
	return fp
}

func (fp *FilterPredicate) Build() string {
	return fp.predicate
}
