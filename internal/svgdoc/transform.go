// SPDX-License-Identifier: GPL-3.0-only

package svgdoc

import (
	"math"
	"strconv"
	"strings"
)

// matrix is an SVG affine transformation [a b c d e f]: x' = a x + c y + e,
// y' = b x + d y + f.
type matrix [6]float64

var identity = matrix{1, 0, 0, 1, 0, 0}

// mul returns m × n: n is applied first, then m (the order in which nested
// transformations compose, outer on the left).
func (m matrix) mul(n matrix) matrix {
	return matrix{
		m[0]*n[0] + m[2]*n[1],
		m[1]*n[0] + m[3]*n[1],
		m[0]*n[2] + m[2]*n[3],
		m[1]*n[2] + m[3]*n[3],
		m[0]*n[4] + m[2]*n[5] + m[4],
		m[1]*n[4] + m[3]*n[5] + m[5],
	}
}

func (m matrix) apply(x, y float64) (float64, float64) {
	return m[0]*x + m[2]*y + m[4], m[1]*x + m[3]*y + m[5]
}

// parseTransform reads a transform attribute: a list of matrix, translate,
// scale, rotate, skewX and skewY. The second result is false when the text
// is not a valid transform list, in which case SVG says the element is not
// drawn; the caller treats it as unknown.
func parseTransform(s string) (matrix, bool) {
	m := identity
	s = strings.TrimSpace(s)
	for s != "" {
		open := strings.IndexByte(s, '(')
		end := strings.IndexByte(s, ')')
		if open < 0 || end < open {
			return identity, false
		}
		name := strings.TrimSpace(strings.Trim(s[:open], ", \t\r\n"))
		nums, ok := parseNumbers(s[open+1 : end])
		if !ok {
			return identity, false
		}
		var t matrix
		switch {
		case name == "matrix" && len(nums) == 6:
			copy(t[:], nums)
		case name == "translate" && len(nums) == 1:
			t = matrix{1, 0, 0, 1, nums[0], 0}
		case name == "translate" && len(nums) == 2:
			t = matrix{1, 0, 0, 1, nums[0], nums[1]}
		case name == "scale" && len(nums) == 1:
			t = matrix{nums[0], 0, 0, nums[0], 0, 0}
		case name == "scale" && len(nums) == 2:
			t = matrix{nums[0], 0, 0, nums[1], 0, 0}
		case name == "rotate" && (len(nums) == 1 || len(nums) == 3):
			a := nums[0] * math.Pi / 180
			sin, cos := math.Sincos(a)
			t = matrix{cos, sin, -sin, cos, 0, 0}
			if len(nums) == 3 {
				cx, cy := nums[1], nums[2]
				t = matrix{1, 0, 0, 1, cx, cy}.mul(t).mul(matrix{1, 0, 0, 1, -cx, -cy})
			}
		case name == "skewX" && len(nums) == 1:
			t = matrix{1, 0, math.Tan(nums[0] * math.Pi / 180), 1, 0, 0}
		case name == "skewY" && len(nums) == 1:
			t = matrix{1, math.Tan(nums[0] * math.Pi / 180), 0, 1, 0, 0}
		default:
			return identity, false
		}
		m = m.mul(t)
		s = strings.TrimSpace(strings.TrimLeft(s[end+1:], ", \t\r\n"))
	}
	return m, true
}

// parseNumbers reads numbers separated by spaces and commas.
func parseNumbers(s string) ([]float64, bool) {
	fields := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r' })
	out := make([]float64, 0, len(fields))
	for _, f := range fields {
		v, err := strconv.ParseFloat(f, 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, false
		}
		out = append(out, v)
	}
	return out, true
}
