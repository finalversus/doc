package draw

import (
	"fmt"

	"github.com/codefinio/doc/pdf/internal/transform"
)

type Point struct {
	X float64
	Y float64
}

func NewPoint(x, y float64) Point {
	return Point{X: x, Y: y}
}

func (p Point) Add(dx, dy float64) Point {
	p.X += dx
	p.Y += dy
	return p
}

func (p Point) AddVector(v Vector) Point {
	p.X += v.Dx
	p.Y += v.Dy
	return p
}

func (p Point) Rotate(theta float64) Point {
	r := transform.NewPoint(p.X, p.Y).Rotate(theta)
	return NewPoint(r.X, r.Y)
}

func (p Point) String() string {
	return fmt.Sprintf("(%.1f,%.1f)", p.X, p.Y)
}
