package app

import (
	"errors"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Parsing never treats dots in a directory name as an extension. Years must
// be separate tokens, and a leading numeric movie title (1917, 2001) is kept.
var movieYearRE = regexp.MustCompile(`(?:19|20)[0-9]{2}`)
var movieTechRE = regexp.MustCompile(`(?i)(?:^|[ ._\[({-])(?:2160p|1080[pi]|720p|480p|4k|8k|uhd|atvp|amzn|dsnp|netflix|blu[ ._-]?ray|b[dr]rip|web[ ._-]?(?:dl|rip)|hdtv|dvdrip|remux|x26[45]|h[ .]?26[45]|hevc|avc|hdr10\+?|hdr|dovi|dolby[ ._-]?vision|aac|dts|truehd|ddp|atmos|10bit|8bit)(?:$|[^a-z0-9])`)
var movieBracketRE = regexp.MustCompile(`\[[^\]]*\]|【[^】]*】`)
var genericMovieRE = regexp.MustCompile(`(?i)^(?:movie|movies|film|films|video|videos|media|collection|collections|main|feature|index|正片|视频|电影|影片|正片视频|合集|电影合集|科幻|动作|恐怖|喜剧|剧情|战争|国产|欧美|未分类|[0-9]+|[a-f0-9]{16,}|(?:cd|disc|disk|part)[ ._-]*[0-9]+)$`)
var moviePartRE = regexp.MustCompile(`(?i)(?:^|[ ._-])(?:cd|disc|disk|part)[ ._-]*([0-9]+)(?:$|[ ._-])`)

type movieIdentity struct {
	Title, Year string
	ID          int64
}

func parseMovieName(name string, file bool) movieIdentity {
	if file {
		name = strings.TrimSuffix(name, path.Ext(name))
	}
	info := movieIdentity{}
	if m := tmdbIDRE.FindStringSubmatch(name); m != nil {
		info.ID, _ = strconv.ParseInt(m[1], 10, 64)
	}
	name = tmdbIDRE.ReplaceAllString(name, "")
	// Discard leading release-site/group tags only when a title follows them.
	for strings.HasPrefix(name, "[") || strings.HasPrefix(name, "【") {
		loc := movieBracketRE.FindStringIndex(name)
		if loc == nil || loc[0] != 0 || strings.Trim(name[loc[1]:], " ._-(){}") == "" {
			break
		}
		rest := name[loc[1]:]
		if tech := movieTechRE.FindStringIndex(rest); tech != nil {
			rest = rest[:tech[0]]
		}
		rest = movieYearRE.ReplaceAllString(rest, "")
		if strings.Trim(rest, " ._-()[]{}【】") == "" {
			break
		}
		name = strings.TrimSpace(name[loc[1]:])
	}
	if loc := movieTechRE.FindStringIndex(name); loc != nil {
		name = name[:loc[0]]
	}
	matches := movieYearRE.FindAllStringIndex(name, -1)
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]
		if m[0] > 0 && name[m[0]-1] >= '0' && name[m[0]-1] <= '9' || m[1] < len(name) && name[m[1]] >= '0' && name[m[1]] <= '9' {
			continue
		}
		before := strings.Trim(name[:m[0]], " ._-()[]{}【】")
		if before == "" {
			continue
		}
		info.Year = name[m[0]:m[1]]
		name = before
		break
	}
	name = moviePartRE.ReplaceAllString(name, " ")
	name = strings.NewReplacer(".", " ", "_", " ", "[", " ", "]", " ", "{", " ", "}", " ", "【", " ", "】", " ").Replace(name)
	info.Title = strings.Join(strings.Fields(strings.Trim(name, " ()-_")), " ")
	return info
}
func usefulMovieTitle(s string) bool {
	return s != "" && (!genericMovieRE.MatchString(s) || len(s) == 4 && movieYearRE.MatchString(s))
}
func movieTitleKey(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, s)
}
func identifyMovie(root, filename string) movieIdentity {
	info := parseMovieName(path.Base(filename), true)
	// A meaningful file is authoritative in a multi-movie or collection folder.
	// Only inherit a year/ID from a matching folder, or replace generic filenames.
	root = path.Clean(root)
	for dir := path.Dir(filename); dir != "." && dir != "/"; dir = path.Dir(dir) {
		if dir != root && !strings.HasPrefix(dir, strings.TrimRight(root, "/")+"/") {
			break
		}
		parent := parseMovieName(path.Base(dir), false)
		if usefulMovieTitle(parent.Title) && !genericMovieRE.MatchString(parent.Title) && (dir != root || parent.Year != "" || parent.ID > 0) {
			same := false
			for _, title := range movieSearchTitles(parent.Title) {
				if movieTitleKey(info.Title) == movieTitleKey(title) {
					same = true
				}
			}
			if info.Year != "" && parent.Year != "" && info.Year != parent.Year {
				same = false
			}
			if !usefulMovieTitle(info.Title) {
				info.Title = parent.Title
				if info.Year == "" {
					info.Year = parent.Year
				}
				if info.ID == 0 {
					info.ID = parent.ID
				}
				break
			}
			if same {
				if info.Year == "" {
					info.Year = parent.Year
				}
				if info.ID == 0 {
					info.ID = parent.ID
				}
				break
			}
		}
		if dir == root {
			break
		}
	}
	return info
}
func movieLabel(info movieIdentity) string {
	label := info.Title
	if info.Year != "" {
		label += " (" + info.Year + ")"
	}
	if info.ID > 0 {
		label += " [tmdbid-" + strconv.FormatInt(info.ID, 10) + "]"
	}
	return label
}
func movieExtra(rel string) bool {
	parts := strings.Split(strings.ToLower(rel), "/")
	for _, p := range parts[:len(parts)-1] {
		switch p {
		case "sample", "samples", "extras", "trailers", "featurettes", "behind the scenes", "deleted scenes", "花絮", "预告片":
			return true
		}
	}
	stem := strings.TrimSuffix(parts[len(parts)-1], path.Ext(strings.ToLower(rel)))
	return stem == "sample" || stem == "trailer" || strings.HasSuffix(stem, "-sample") || strings.HasSuffix(stem, ".sample") || strings.HasSuffix(stem, "-trailer") || strings.HasSuffix(stem, ".trailer")
}
func taskOutputPath(t Object, source string) (string, error) {
	legacy, err := outputPath(str(t, "path"), source, str(t, "renameRegex"))
	if err != nil || str(t, "libraryType") != "movie" || str(t, "movieNaming") == "original" || str(t, "renameRegex") != "" {
		return legacy, err
	}
	info := identifyMovie(str(t, "path"), source)
	if !usefulMovieTitle(info.Title) {
		return legacy, nil
	}
	label := safeName(movieLabel(info))
	return filepath.Join(label, label+".strm"), nil
}

// A supplied bilingual name is searched as a whole first, then each language.
// Never drop the year just to obtain a hit for a different movie.
func movieSearchTitles(title string) []string {
	out := []string{title}
	han, latin := []rune{}, []rune{}
	for _, r := range title {
		if unicode.Is(unicode.Han, r) {
			han = append(han, r)
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			latin = append(latin, r)
		}
	}
	if len(han) > 0 && strings.TrimSpace(string(latin)) != "" {
		out = append(out, string(han), strings.Join(strings.Fields(string(latin)), " "))
	}
	return out
}
func selectMovieResult(results []any, title, year string) (int64, error) {
	best, score, tied := int64(0), -1, false
	queries := movieSearchTitles(title)
	for _, v := range results {
		m, _ := v.(map[string]any)
		if num(m, "id") <= 0 {
			continue
		}
		date := str(m, "release_date")
		if year != "" && len(date) >= 4 && date[:4] != year {
			continue
		}
		n := 0
		for _, query := range queries {
			key := movieTitleKey(query)
			if key != "" && (key == movieTitleKey(str(m, "title")) || key == movieTitleKey(str(m, "original_title"))) {
				n = 10
			}
		}
		if n > score {
			best, score, tied = num(m, "id"), n, false
		} else if n == score && num(m, "id") != best {
			tied = true
		}
	}
	if best == 0 {
		return 0, errors.New("TMDB 未匹配到对应年份的电影，请核对年份或指定 TMDB ID")
	}
	if tied || len(results) > 1 && score == 0 {
		return 0, errors.New("TMDB 电影匹配不明确，请在手动刮削中选择影片或指定 TMDB ID")
	}
	return best, nil
}
