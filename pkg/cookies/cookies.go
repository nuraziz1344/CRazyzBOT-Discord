package cookies

import (
	"bufio"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Entry struct {
	Domain   string
	Path     string
	Secure   bool
	Expires  int64
	Name     string
	Value    string
	HttpOnly bool
}

var (
	cacheMu     sync.RWMutex
	cachedPath  string
	cachedMtime time.Time
	cachedList  []Entry
)

// LoadNetscape reads a Netscape HTTP Cookie File (e.g. cookies.txt exported from browser or yt-dlp)
// and caches the parsed entries in memory unless the file has been modified.
func LoadNetscape(filePath string) ([]Entry, error) {
	if filePath == "" {
		return nil, nil
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	cacheMu.RLock()
	if cachedPath == filePath && cachedMtime.Equal(info.ModTime()) {
		res := cachedList
		cacheMu.RUnlock()
		return res, nil
	}
	cacheMu.RUnlock()

	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		httpOnly := false
		if strings.HasPrefix(line, "#HttpOnly_") {
			httpOnly = true
			line = strings.TrimPrefix(line, "#HttpOnly_")
		} else if strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) < 7 {
			continue
		}

		exp, _ := strconv.ParseInt(parts[4], 10, 64)
		entries = append(entries, Entry{
			Domain:    strings.ToLower(strings.TrimSpace(parts[0])),
			Path:     strings.TrimSpace(parts[2]),
			Secure:   strings.EqualFold(strings.TrimSpace(parts[3]), "TRUE"),
			Expires:  exp,
			Name:     strings.TrimSpace(parts[5]),
			Value:    strings.TrimSpace(parts[6]),
			HttpOnly: httpOnly,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	cacheMu.Lock()
	cachedPath = filePath
	cachedMtime = info.ModTime()
	cachedList = entries
	cacheMu.Unlock()

	return entries, nil
}

// Apply attaches matching domain cookies from a Netscape cookies.txt file to an outbound HTTP request.
func Apply(req *http.Request, cookieFile string) {
	if req == nil || req.URL == nil || cookieFile == "" {
		return
	}

	entries, err := LoadNetscape(cookieFile)
	if err != nil || len(entries) == 0 {
		return
	}

	host := strings.ToLower(req.URL.Hostname())
	now := time.Now().Unix()

	for _, e := range entries {
		if e.Expires > 0 && e.Expires < now {
			continue
		}
		if matchDomain(host, e.Domain) {
			req.AddCookie(&http.Cookie{
				Name:  e.Name,
				Value: e.Value,
			})
		}
	}
}

func matchDomain(host, cookieDomain string) bool {
	cleanCookie := strings.TrimPrefix(cookieDomain, ".")
	if host == cleanCookie {
		return true
	}
	return strings.HasSuffix(host, "."+cleanCookie)
}
