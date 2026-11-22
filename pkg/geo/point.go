package geo

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

const (
	wkbSRIDFlag uint32 = 0x20000000
	wkbZFlag    uint32 = 0x80000000
	wkbMFlag    uint32 = 0x40000000
	wkbTypeMask uint32 = 0x0FFFFFFF

	wkbPoint   uint32 = 1
	wkbPointZ  uint32 = 1001
	wkbPointM  uint32 = 2001
	wkbPointZM uint32 = 3001
)

type Point struct {
	Lng float64
	Lat float64
}

var (
	_ driver.Valuer = Point{}
	_ sql.Scanner   = (*Point)(nil)
	_ gorm.Valuer   = Point{}
)

func validateCoords(lng, lat float64) error {
	if math.IsNaN(lat) || math.IsInf(lat, 0) || lat < -90 || lat > 90 {
		return fmt.Errorf("geo: invalid latitude %v", lat)
	}
	if math.IsNaN(lng) || math.IsInf(lng, 0) || lng < -180 || lng > 180 {
		return fmt.Errorf("geo: invalid longitude %v", lng)
	}
	return nil
}

func (p Point) Value() (driver.Value, error) {
	if err := validateCoords(p.Lng, p.Lat); err != nil {
		return nil, err
	}
	return fmt.Sprintf(
		"SRID=4326;POINT(%s %s)",
		strconv.FormatFloat(p.Lng, 'f', -1, 64),
		strconv.FormatFloat(p.Lat, 'f', -1, 64),
	), nil
}

func (p *Point) Scan(value any) error {
	if p == nil {
		return fmt.Errorf("geo: cannot scan into nil Point")
	}
	if value == nil {
		return fmt.Errorf("geo: cannot scan nil value into Point")
	}

	var text string
	switch v := value.(type) {
	case []byte:
		if len(v) > 0 && (v[0] == 0x00 || v[0] == 0x01) {
			return p.scanWKB(v)
		}
		text = string(v)
	case string:
		text = v
	default:
		return fmt.Errorf("geo: unsupported scan type %T", value)
	}

	s := strings.TrimSpace(text)
	if s == "" {
		return fmt.Errorf("geo: invalid empty point")
	}

	upper := strings.ToUpper(s)
	if strings.HasPrefix(upper, "SRID=") || strings.HasPrefix(upper, "POINT") {
		return p.scanWKT(s)
	}

	switch {
	case strings.HasPrefix(s, `\x`), strings.HasPrefix(s, `\X`):
		s = s[2:]
	case strings.HasPrefix(s, "0x"), strings.HasPrefix(s, "0X"):
		s = s[2:]
	}

	raw, err := hex.DecodeString(s)
	if err != nil {
		return fmt.Errorf("geo: invalid point hex: %w", err)
	}
	return p.scanWKB(raw)
}

func (p *Point) scanWKB(b []byte) error {
	if len(b) < 5 {
		return fmt.Errorf("geo: invalid WKB length %d", len(b))
	}

	var order binary.ByteOrder
	switch b[0] {
	case 0x00:
		order = binary.BigEndian
	case 0x01:
		order = binary.LittleEndian
	default:
		return fmt.Errorf("geo: invalid WKB byte order 0x%02x", b[0])
	}

	wkbType := order.Uint32(b[1:5])
	hasSRID := (wkbType & wkbSRIDFlag) != 0
	hasZ := (wkbType & wkbZFlag) != 0
	hasM := (wkbType & wkbMFlag) != 0

	switch wkbType & wkbTypeMask {
	case wkbPoint:
	case wkbPointZ:
		hasZ = true
	case wkbPointM:
		hasM = true
	case wkbPointZM:
		hasZ = true
		hasM = true
	default:
		return fmt.Errorf("geo: expected WKB Point geometry, got type %d", wkbType&wkbTypeMask)
	}

	offset := 5
	if hasSRID {
		offset += 4
	}

	coordBytes := 16
	if hasZ {
		coordBytes += 8
	}
	if hasM {
		coordBytes += 8
	}

	if len(b) != offset+coordBytes {
		return fmt.Errorf("geo: invalid WKB Point length %d, expected %d", len(b), offset+coordBytes)
	}

	lng := math.Float64frombits(order.Uint64(b[offset : offset+8]))
	lat := math.Float64frombits(order.Uint64(b[offset+8 : offset+16]))
	if err := validateCoords(lng, lat); err != nil {
		return err
	}

	p.Lng = lng
	p.Lat = lat
	return nil
}

func (p *Point) scanWKT(s string) error {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(strings.ToUpper(s), "SRID=") {
		sridPart, rest, ok := strings.Cut(s[5:], ";")
		if !ok {
			return fmt.Errorf("geo: invalid EWKT point: missing semicolon")
		}
		if _, err := strconv.Atoi(strings.TrimSpace(sridPart)); err != nil {
			return fmt.Errorf("geo: invalid EWKT SRID: %w", err)
		}
		s = strings.TrimSpace(rest)
	}

	if len(s) < 5 || !strings.EqualFold(s[:5], "POINT") {
		return fmt.Errorf("geo: invalid WKT point: expected POINT prefix")
	}

	s = strings.TrimSpace(s[5:])
	if !strings.HasPrefix(s, "(") || !strings.HasSuffix(s, ")") {
		return fmt.Errorf("geo: invalid WKT point: missing parentheses")
	}

	inner := s[1 : len(s)-1]
	coords := strings.Fields(inner)
	if len(coords) != 2 {
		return fmt.Errorf("geo: invalid WKT point: expected 2 coordinates, got %d", len(coords))
	}

	lng, err := strconv.ParseFloat(coords[0], 64)
	if err != nil {
		return fmt.Errorf("geo: invalid longitude: %w", err)
	}

	lat, err := strconv.ParseFloat(coords[1], 64)
	if err != nil {
		return fmt.Errorf("geo: invalid latitude: %w", err)
	}

	if err := validateCoords(lng, lat); err != nil {
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
