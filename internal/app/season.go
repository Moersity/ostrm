package app

import (
	"regexp"
	"strconv"
	"strings"
)

var seasonChinese = regexp.MustCompile(`第\s*([0-9零〇一二两三四五六七八九十百]+)\s*季`)
var seasonEnglish = regexp.MustCompile(`(?i)(?:^|[^a-z])season[ ._-]*(\d{1,3})(?:$|[^0-9])`)
var seasonShort = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])s(\d{1,3})(?:$|[^a-z0-9])`)
var seasonSpecial = regexp.MustCompile(`(?i)(?:^|[^a-z])specials?(?:$|[^a-z])|特别篇|特別篇`)

func seasonNumber(s string) int {
	if n, e := strconv.Atoi(s); e == nil {
		return n
	}
	digits := map[rune]int{'零': 0, '〇': 0, '一': 1, '二': 2, '两': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9}
	total, current := 0, 0
	for _, r := range s {
		if r == '百' {
			return -1
		}
		if r == '十' {
			if current == 0 {
				current = 1
			}
			total += current * 10
			current = 0
		} else if n, ok := digits[r]; ok {
			current = n
		} else {
			return -1
		}
	}
	return total + current
}

// Conflicting markers are not guessed. S04E01 is a file marker, not a season directory.
func parseSeason(s string) (int, bool) {
	values := []int{}
	for _, r := range []*regexp.Regexp{seasonChinese, seasonEnglish, seasonShort} {
		for _, m := range r.FindAllStringSubmatch(s, -1) {
			values = append(values, seasonNumber(m[1]))
		}
	}
	if seasonSpecial.MatchString(s) {
		values = append(values, 0)
	}
	if len(values) == 0 {
		return 0, false
	}
	n := values[0]
	for _, v := range values {
		if v != n || v < 0 || v > 99 {
			return 0, false
		}
	}
	return n, true
}
func isSeasonDirectory(s string) bool { _, ok := parseSeason(strings.TrimSpace(s)); return ok }
