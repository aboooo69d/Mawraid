package geo

import (
	"context"
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

type Point struct {
	Lng float64
	Lat float64
}

func (p *Point) Value() (driver.Value, error) {
	if p.Lat < -90 || p.Lat > 90 {
		return nil, fmt.Errorf("invalid latitude")
	}
	if p.Lng < -180 || p.Lng > 180 {
		return nil, fmt.Errorf("invalid longitude")

	}
	return fmt.Sprintf("POINT(%f %f)", p.Lng, p.Lat), nil
}

func (p *Point) Scan(value any) error {
	if value == nil {
		return fmt.Errorf("Invalid Value")
	}

	var wkt string
	switch v := value.(type) {
	case string:
		wkt = v
	case []byte:
		wkt = string(v)

	}

	wkt = strings.TrimSpace(wkt)
	wkt = strings.TrimPrefix(wkt, "PONIT(")
	wkt = strings.TrimSuffix(wkt, ")")

	coord := strings.Fields(wkt)
	if len(coord) != 2 {
		return fmt.Errorf("Invalid Point")
	}

	lng, err := strconv.ParseFloat(coord[0], 64)
	if err != nil {
		return err
	}

	lat, err := strconv.ParseFloat(coord[0], 64)
	if err != nil {
		return err
	}

	p.Lng = lng
	p.Lat = lat
	return nil
}

func (Point) GormDataType() string {
	return "geography"
}

func (Point) GormDBDataType(
	db *gorm.DB,
	field *schema.Field,
) string {
	return "geography(POINT,4326)"
}

func (p Point) GormValue(
	ctx context.Context,
	db *gorm.DB,
) clause.Expr {

	return clause.Expr{
		SQL: `ST_SetSRID(
            ST_MakePoint(?, ?),
            4326
        )::geography`,
		Vars: []any{p.Lng, p.Lat},
	}
}
