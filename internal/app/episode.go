package app

import (
	"path"
	"regexp"
	"strconv"
	"strings"
)

var episodePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:^|[^a-z0-9])S([0-9]{1,2})[ ._-]*EP?([0-9]{1,3})(?:$|[^0-9])`),
	regexp.MustCompile(`(?i)(?:^|[^a-z0-9])([0-9]{1,2})x([0-9]{1,3})(?:$|[^0-9])`),
	regexp.MustCompile(`(?i)(?:^|[^a-z])Season[ ._-]*([0-9]{1,2})[ ._-]*Episode[ ._-]*([0-9]{1,3})(?:$|[^0-9])`),
}
var episodeOnly = regexp.MustCompile(`(?i)(?:^|[^a-z])(?:episode|ep|e)[ ._-]*([0-9]{1,3})(?:$|[^a-z0-9])`)
var episodeChinese = regexp.MustCompile(`第\s*([0-9零〇一二两三四五六七八九十百]+)\s*[集话話]`)
var episodeBare = regexp.MustCompile(`^([0-9]{1,3})(?:$|[ ._-])`)
var episodeAnime = regexp.MustCompile(`\s-\s([0-9]{1,3})(?:$|[ ._\[])`)

func mediaNumbers(p string) (int, int) {
	season, episode := 1, 0
	contextual := false
	for _, d := range strings.Split(path.Dir(p), "/") {
		if n, ok := parseSeason(d); ok {
			season, contextual = n, true
		}
	}
	name := strings.TrimSuffix(path.Base(p), path.Ext(p))
	for _, pattern := range episodePatterns {
		if m := pattern.FindStringSubmatch(name); m != nil {
			season, _ = strconv.Atoi(m[1])
			episode, _ = strconv.Atoi(m[2])
			return season, episode
		}
	}
	if n, ok := parseSeason(name); ok {
		season = n
	}
	if m := episodeOnly.FindStringSubmatch(name); m != nil {
		episode, _ = strconv.Atoi(m[1])
	} else if m := episodeChinese.FindStringSubmatch(name); m != nil {
		episode = seasonNumber(m[1])
	} else if contextual {
		if m := episodeBare.FindStringSubmatch(name); m != nil {
			episode, _ = strconv.Atoi(m[1])
		} else if m := episodeAnime.FindStringSubmatch(name); m != nil {
			episode, _ = strconv.Atoi(m[1])
		}
	}
	return season, max(0, episode)
}

// Season and episode markers are removed before release metadata. File titles
// take precedence; generic episode filenames inherit the nearest show directory.
func parseTVName(name string, file bool) movieIdentity {
	if file {
		name = strings.TrimSuffix(name, path.Ext(name))
	}
	cut := len(name)
	for _, r := range append(append([]*regexp.Regexp{}, episodePatterns...), episodeOnly, episodeChinese, episodeAnime, seasonChinese, seasonEnglish, seasonShort) {
		if loc := r.FindStringIndex(name); loc != nil && loc[0] < cut {
			cut = loc[0]
		}
	}
	info := parseMovieName(name[:cut], false)
	if m := tmdbIDRE.FindStringSubmatch(name); m != nil {
		info.ID, _ = strconv.ParseInt(m[1], 10, 64)
	}
	if info.Year == "" {
		info.Year = parseMovieName(name, false).Year
	}
	return info
}

func identifyTV(root, filename string) movieIdentity {
	info := parseTVName(path.Base(filename), true)
	root = path.Clean(root)
	for dir := path.Dir(filename); dir != "." && dir != "/"; dir = path.Dir(dir) {
		if dir != root && !strings.HasPrefix(dir, strings.TrimRight(root, "/")+"/") {
			break
		}
		parent := parseTVName(path.Base(dir), false)
		if usefulMovieTitle(parent.Title) && !isSeasonDirectory(parent.Title) && (dir != root || parent.Year != "" || parent.ID > 0) {
			if !usefulMovieTitle(info.Title) {
				return parent
			}
			for _, title := range movieSearchTitles(parent.Title) {
				if movieTitleKey(title) == movieTitleKey(info.Title) {
					if info.Year == "" {
						info.Year = parent.Year
					}
					if info.ID == 0 {
						info.ID = parent.ID
					}
					return info
				}
			}
		}
		if dir == root {
			break
		}
	}
	return info
}
