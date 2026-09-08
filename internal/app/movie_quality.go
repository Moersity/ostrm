package app

import (
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var qualityResolutionRE = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(4320|2160|1440|1080|720|576|480)[pi](?:$|[^a-z0-9])`)
var qualityTokenRE = regexp.MustCompile(`[^a-z0-9]+`)
var movieEditionRE = regexp.MustCompile(`(?i)director[ ._'’-]*s?[ ._-]*cut|extended(?:[ ._-]*(?:cut|edition))?|unrated|uncut|theatrical(?:[ ._-]*cut)?|final[ ._-]*cut|ultimate[ ._-]*cut|加长版|导演剪辑版|未删减版|剧场版`)
var movieDiscRE = regexp.MustCompile(`(?i)(?:^|[ /._-])(?:cd|disc|disk|part)[ ._-]*([0-9]+)(?:$|[ /._-])`)

func movieQuality(f remoteFile) [5]int64 {
	text := strings.ToLower(f.Path)
	q := [5]int64{}
	for _, m := range qualityResolutionRE.FindAllStringSubmatch(text, -1) {
		n, _ := strconv.ParseInt(m[1], 10, 64)
		if n > q[0] {
			q[0] = n
		}
	}
	words := " " + qualityTokenRE.ReplaceAllString(text, " ") + " "
	has := func(token string) bool { return strings.Contains(words, " "+token+" ") }
	if has("8k") {
		q[0] = 4320
	} else if has("4k") || has("uhd") {
		if q[0] < 2160 {
			q[0] = 2160
		}
	}
	if has("dv") || has("dovi") || strings.Contains(words, " dolby vision ") {
		q[1] = 3
	} else if strings.Contains(text, "hdr10+") || has("hdr10plus") {
		q[1] = 2
	} else if has("hdr") || has("hdr10") || has("hlg") {
		q[1] = 1
	}
	switch {
	case has("remux"):
		q[2] = 5
	case has("bluray") || strings.Contains(words, " blu ray ") || has("bdrip"):
		q[2] = 4
	case strings.Contains(words, " web dl ") || has("webdl"):
		q[2] = 3
	case has("webrip"):
		q[2] = 2
	case has("hdtv") || has("dvdrip"):
		q[2] = 1
	}
	switch {
	case has("av1"):
		q[3] = 3
	case has("hevc") || has("h265") || has("x265") || strings.Contains(words, " h 265 "):
		q[3] = 2
	case has("avc") || has("h264") || has("x264") || strings.Contains(words, " h 264 "):
		q[3] = 1
	}
	// An explicit file resolution overrides broad library/folder labels.
	base := strings.ToLower(path.Base(f.Path))
	own := int64(0)
	for _, m := range qualityResolutionRE.FindAllStringSubmatch(base, -1) {
		n, _ := strconv.ParseInt(m[1], 10, 64)
		if n > own {
			own = n
		}
	}
	ownWords := " " + qualityTokenRE.ReplaceAllString(base, " ") + " "
	if strings.Contains(ownWords, " 8k ") {
		own = 4320
	} else if strings.Contains(ownWords, " 4k ") || strings.Contains(ownWords, " uhd ") {
		if own < 2160 {
			own = 2160
		}
	}
	if own > 0 {
		q[0] = own
	}
	if strings.Contains(ownWords, " sdr ") {
		q[1] = 0
	}
	q[4] = f.Size
	return q
}
func betterMovie(a, b remoteFile) bool {
	qa, qb := movieQuality(a), movieQuality(b)
	for i := range qa {
		if qa[i] != qb[i] {
			return qa[i] > qb[i]
		}
	}
	return a.Path < b.Path
}
func movieVersionGroup(t Object, f remoteFile) string {
	info := identifyMovie(str(t, "path"), f.Path)
	if !usefulMovieTitle(info.Title) && info.ID == 0 {
		return "file:" + f.Path
	}
	key := movieTitleKey(info.Title) + "|" + info.Year
	if info.ID > 0 {
		key = "tmdb:" + strconv.FormatInt(info.ID, 10)
	}
	// Without year or ID, only deduplicate within the same source directory.
	if info.Year == "" && info.ID == 0 {
		key += "|" + path.Dir(f.Path)
	}
	rel, _ := remoteRelative(str(t, "path"), f.Path)
	edition := movieEditionRE.FindString(rel)
	if edition != "" {
		key += "|edition:" + movieTitleKey(edition)
	}
	if m := movieDiscRE.FindStringSubmatch(rel); m != nil {
		key += "|part:" + m[1]
	}
	return key
}
func selectMovieVersions(t Object, files []remoteFile) ([]remoteFile, []Object) {
	if str(t, "libraryType") != "movie" || str(t, "movieVersions") == "all" {
		return files, nil
	}
	// Connect untranslated filenames to already identified bilingual filenames,
	// but only when the same title/year points to one unambiguous TMDB ID.
	known := map[string]map[int64]bool{}
	for _, f := range files {
		info := identifyMovie(str(t, "path"), f.Path)
		if info.ID == 0 || info.Year == "" {
			continue
		}
		for _, title := range movieSearchTitles(info.Title) {
			alias := movieTitleKey(title) + "|" + info.Year
			if known[alias] == nil {
				known[alias] = map[int64]bool{}
			}
			known[alias][info.ID] = true
		}
	}
	groups := map[string][]remoteFile{}
	for _, f := range files {
		key := movieVersionGroup(t, f)
		info := identifyMovie(str(t, "path"), f.Path)
		if info.ID == 0 && info.Year != "" {
			ids := map[int64]bool{}
			for _, title := range movieSearchTitles(info.Title) {
				for id := range known[movieTitleKey(title)+"|"+info.Year] {
					ids[id] = true
				}
			}
			if len(ids) == 1 {
				for id := range ids {
					key = strings.Replace(key, movieTitleKey(info.Title)+"|"+info.Year, "tmdb:"+strconv.FormatInt(id, 10), 1)
				}
			}
		}
		groups[key] = append(groups[key], f)
	}
	selected := []remoteFile{}
	decisions := []Object{}
	for _, group := range groups {
		sort.Slice(group, func(i, j int) bool { return betterMovie(group[i], group[j]) })
		selected = append(selected, group[0])
		for _, f := range group[1:] {
			decisions = append(decisions, Object{"kept": group[0].Path, "filtered": f.Path, "keptQuality": movieQuality(group[0]), "filteredQuality": movieQuality(f)})
		}
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].Path < selected[j].Path })
	sort.Slice(decisions, func(i, j int) bool { return str(decisions[i], "filtered") < str(decisions[j], "filtered") })
	return selected, decisions
}
func canReplaceMovieSource(t Object, oldSource, newSource string) bool {
	if str(t, "libraryType") != "movie" || str(t, "movieVersions") == "all" {
		return false
	}
	if _, e := remoteRelative(str(t, "path"), oldSource); e != nil {
		return false
	}
	old := identifyMovie(str(t, "path"), oldSource)
	next := identifyMovie(str(t, "path"), newSource)
	if old.ID > 0 && next.ID > 0 && old.ID != next.ID {
		return false
	}
	if movieTitleKey(movieEditionRE.FindString(oldSource)) != movieTitleKey(movieEditionRE.FindString(newSource)) {
		return false
	}
	part := func(p string) string {
		if m := movieDiscRE.FindStringSubmatch(p); m != nil {
			return m[1]
		}
		return ""
	}
	if part(oldSource) != part(newSource) {
		return false
	}
	if old.ID > 0 && old.ID == next.ID {
		return true
	}
	if old.Year != "" && old.Year == next.Year {
		for _, a := range movieSearchTitles(old.Title) {
			for _, b := range movieSearchTitles(next.Title) {
				if usefulMovieTitle(a) && movieTitleKey(a) == movieTitleKey(b) {
					return true
				}
			}
		}
	}
	return movieVersionGroup(t, remoteFile{Path: oldSource}) == movieVersionGroup(t, remoteFile{Path: newSource})
}

func movieQualityLabel(f remoteFile) string {
	q := movieQuality(f)
	parts := []string{}
	if q[0] > 0 {
		parts = append(parts, strconv.FormatInt(q[0], 10)+"p")
	}
	if q[1] > 0 {
		parts = append(parts, map[int64]string{1: "HDR", 2: "HDR10+", 3: "DV"}[q[1]])
	}
	if q[2] > 0 {
		parts = append(parts, map[int64]string{1: "HDTV", 2: "WEBRip", 3: "WEB-DL", 4: "BluRay", 5: "REMUX"}[q[2]])
	}
	if q[3] > 0 {
		parts = append(parts, map[int64]string{1: "H264", 2: "H265", 3: "AV1"}[q[3]])
	}
	if edition := movieEditionRE.FindString(f.Path); edition != "" {
		parts = append(parts, edition)
	}
	if m := movieDiscRE.FindStringSubmatch(f.Path); m != nil {
		parts = append(parts, "CD"+m[1])
	}
	return strings.Join(parts, " ")
}
