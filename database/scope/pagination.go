package scope

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func Paginate(page, pageSize int) func(*gorm.Statement) {
	return func(stmt *gorm.Statement) {
		if page < 1 {
			page = 1
		}

		if pageSize < 1 {
			pageSize = 10
		}

		if pageSize > 100 {
			pageSize = 100
		}

		offset := (page - 1) * pageSize

		stmt.AddClause(clause.Limit{
			Limit:  &pageSize,
			Offset: offset,
		})
	}
}
