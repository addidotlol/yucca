package version

import (
	"strconv"
	"strings"
	"unicode"
)

func Normalize(tag string) string {
	t := strings.TrimSpace(tag)
	t = strings.TrimPrefix(strings.ToLower(t), "v")
	return t
}

func Compare(a, b string) int {
	pa := parseInts(Normalize(a))
	pb := parseInts(Normalize(b))
	max := len(pa)
	if len(pb) > max {
		max = len(pb)
	}
	for i := 0; i < max; i++ {
		ai := 0
		bi := 0
		if i < len(pa) {
			ai = pa[i]
		}
		if i < len(pb) {
			bi = pb[i]
		}
		if ai < bi {
			return -1
		}
		if ai > bi {
			return 1
		}
	}

	an := Normalize(a)
	bn := Normalize(b)
	if an < bn {
		return -1
	}
	if an > bn {
		return 1
	}
	return 0
}

func parseInts(s string) []int {
	parts := make([]int, 0, 4)
	b := strings.Builder{}
	flush := func() {
		if b.Len() == 0 {
			return
		}
		n, err := strconv.Atoi(b.String())
		if err == nil {
			parts = append(parts, n)
		}
		b.Reset()
	}

	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return parts
}
