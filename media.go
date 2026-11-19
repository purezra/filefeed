package main

import (
	"encoding/xml"
	"errors"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ImageFilter struct {
	Exts           []string `json:"exts"`
	Ratio          string   `json:"ratio"`
	Width          int      `json:"width"`
	Height         int      `json:"height"`
	ResolutionMode string   `json:"resolutionMode"`
	MinMB          float64  `json:"minMB"`
	MaxMB          float64  `json:"maxMB"`
}

type VideoFilter struct {
	Exts       []string `json:"exts"`
	MinMB      float64  `json:"minMB"`
	MaxMB      float64  `json:"maxMB"`
	MinSeconds float64  `json:"minSeconds"`
	MaxSeconds float64  `json:"maxSeconds"`
}

type MediaProperties struct {
	Width           int     `json:"width"`
	Height          int     `json:"height"`
	DurationSeconds float64 `json:"durationSeconds"`
	aspect          float64
}

type mediaFilters struct {
	image, video map[string]bool
	picture      ImageFilter
	movie        VideoFilter
	ratio        float64
}

func newMediaFilters(image ImageFilter, video VideoFilter) (mediaFilters, error) {
	f := mediaFilters{picture: image, movie: video}
	validRange := func(min, max float64) bool { return min >= 0 && max >= 0 && (max == 0 || max >= min) }
	if len(image.Exts) > 0 {
		if !validRange(image.MinMB, image.MaxMB) || image.Width < 0 || image.Height < 0 {
			return f, errors.New("请检查图片大小范围和分辨率")
		}
		f.picture.Ratio = strings.TrimSpace(image.Ratio)
		switch f.picture.Ratio {
		case "", "横图", "竖图":
		default:
			left, right, ok := strings.Cut(strings.ReplaceAll(f.picture.Ratio, "：", ":"), ":")
			w, wErr := strconv.Atoi(strings.TrimSpace(left))
			h, hErr := strconv.Atoi(strings.TrimSpace(right))
			if !ok || wErr != nil || hErr != nil || w <= 0 || h <= 0 {
				return f, errors.New("图片比例请填写两个正整数，例如 16:9")
			}
			f.ratio = float64(w) / float64(h)
		}
		f.image = extensionSet(image.Exts)
	}
	if len(video.Exts) > 0 {
		if !validRange(video.MinMB, video.MaxMB) || !validRange(video.MinSeconds, video.MaxSeconds) {
			return f, errors.New("请检查视频大小和时长范围")
		}
		f.video = extensionSet(video.Exts)
	}
	return f, nil
}

func extensionSet(exts []string) map[string]bool {
	set := make(map[string]bool, len(exts))
	for _, ext := range exts {
		set[strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))] = true
	}
	return set
}

func withinRange(value, min, max float64) bool {
	return value >= min && (max == 0 || value <= max)
}

func (f mediaFilters) match(ext, path string, st os.FileInfo) (matched, missing bool, media MediaProperties, err error) {
	size := float64(st.Size()) / (1024 * 1024)
	switch {
	case f.image[ext]:
		image := f.picture
		if !withinRange(size, image.MinMB, image.MaxMB) {
			return false, false, media, nil
		}
		if image.Ratio == "" && image.Width == 0 && image.Height == 0 {
			break
		}
		media, err = cachedMedia(path, st, true, ext)
		if err != nil {
			return false, false, media, err
		}
		if (image.Width > 0 && media.Width == 0) || (image.Height > 0 && media.Height == 0) ||
			(image.Ratio != "" && media.aspect == 0) {
			return false, true, media, nil
		}
		if !matchesResolution(media.Width, image.Width, image.ResolutionMode) ||
			!matchesResolution(media.Height, image.Height, image.ResolutionMode) || !f.matchesRatio(media) {
			return false, false, media, nil
		}
	case f.video[ext]:
		video := f.movie
		if !withinRange(size, video.MinMB, video.MaxMB) {
			return false, false, media, nil
		}
		if video.MinSeconds == 0 && video.MaxSeconds == 0 {
			break
		}
		media, err = cachedMedia(path, st, false, ext)
		if err != nil {
			return false, false, media, err
		}
		if media.DurationSeconds <= 0 {
			return false, true, media, nil
		}
		return withinRange(media.DurationSeconds, video.MinSeconds, video.MaxSeconds), false, media, nil
	}
	return true, false, media, nil
}

func matchesResolution(actual, requested int, mode string) bool {
	if requested == 0 {
		return true
	}
	switch mode {
	case "exact":
		return actual == requested
	case "max":
		return actual <= requested
	default:
		return actual >= requested
	}
}

func (f mediaFilters) matchesRatio(media MediaProperties) bool {
	switch f.picture.Ratio {
	case "":
		return true
	case "横图":
		return media.aspect > 1
	case "竖图":
		return media.aspect < 1
	}
	if media.Height > 0 && media.Width > 0 {
		// A resized image may round either dimension to the nearest pixel.
		return math.Abs(float64(media.Width)-f.ratio*float64(media.Height)) <= math.Max(1, f.ratio)/2
	}
	return math.Abs(media.aspect-f.ratio) <= f.ratio*0.0001
}

type mediaCacheKey struct {
	path     string
	image    bool
	size     int64
	modified time.Time
}

var mediaCache = struct {
	sync.Mutex
	values map[mediaCacheKey]MediaProperties
}{values: make(map[mediaCacheKey]MediaProperties)}

func cachedMedia(path string, st os.FileInfo, image bool, ext string) (MediaProperties, error) {
	key := mediaCacheKey{path, image, st.Size(), st.ModTime()}
	mediaCache.Lock()
	props, found := mediaCache.values[key]
	mediaCache.Unlock()
	if found {
		return props, nil
	}
	if image && ext == "svg" {
		props = svgProperties(path)
	} else {
		var err error
		props, err = readMediaProperties(path, image)
		if err != nil {
			return props, err
		}
	}
	if props.Width > 0 && props.Height > 0 {
		props.aspect = float64(props.Width) / float64(props.Height)
	}
	mediaCache.Lock()
	if len(mediaCache.values) >= 8192 {
		for old := range mediaCache.values {
			delete(mediaCache.values, old)
			break
		}
	}
	mediaCache.values[key] = props
	mediaCache.Unlock()
	return props, nil
}

func svgProperties(path string) MediaProperties {
	var props MediaProperties
	file, err := os.Open(path)
	if err != nil {
		return props
	}
	defer file.Close()
	decoder := xml.NewDecoder(io.LimitReader(file, 256*1024))
	for {
		token, err := decoder.Token()
		if err != nil {
			return props
		}
		root, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if root.Name.Local != "svg" {
			return props
		}
		for _, attr := range root.Attr {
			switch attr.Name.Local {
			case "width":
				props.Width = svgPixels(attr.Value)
			case "height":
				props.Height = svgPixels(attr.Value)
			case "viewBox":
				parts := strings.Fields(strings.ReplaceAll(attr.Value, ",", " "))
				if len(parts) == 4 {
					w, _ := strconv.ParseFloat(parts[2], 64)
					h, _ := strconv.ParseFloat(parts[3], 64)
					if w > 0 && h > 0 && !math.IsInf(w, 0) && !math.IsInf(h, 0) {
						props.aspect = w / h
					}
				}
			}
		}
		return props
	}
}

func svgPixels(value string) int {
	n, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(value), "px"), 64)
	if err != nil || !(n > 0 && n <= math.MaxInt32) {
		return 0
	}
	return int(math.Round(n))
}
