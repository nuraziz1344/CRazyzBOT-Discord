package youtube

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/tidwall/gjson"
)

var Debug = false

func YtMp3(q string) (T string, U string, E error) {
	form := &url.Values{}
	form.Set("u", q)
	form.Set("c", "ID")

	req, err := http.PostForm("https://ytpp3.com/newp", *form)
	if err != nil {
		return "", "", err
	}

	res, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return "", "", err
	}

	if Debug {
		fmt.Println(string(res))
	}
	mp3Url := gjson.GetBytes(res, "data.mp3").Str
	title := gjson.GetBytes(res, "data.title").Str

	if mp3Url == "" {
		mp3Url = gjson.GetBytes(res, "data.mp3_cdn").Str
		mp3UrlCache := ""
		if mp3Url == "" && gjson.GetBytes(res, "data.mp3_cdn").Type.String() == "JSON" {
			for _, v := range gjson.GetBytes(res, "data.mp3_cdn").Array() {
				mp3UrlCache = v.Get("mp3_cdn").Str
				if mp3UrlCache == "" {
					mp3UrlCache = v.Get("mp3_url").Str
				}
				if Debug {
					fmt.Printf("format:%s | url:%s\n", v.Get("mp3_format").Str, v.Get("mp3_url").Str)
				}
				if mp3UrlCache != "" && v.Get("mp3_format").Str == "mp3" {
					if Debug {
						fmt.Println("breaking")
					}
					mp3Url = mp3UrlCache
					break
				}
			}
			if mp3Url == "" {
				mp3Url = mp3UrlCache
			}
		} else if mp3Url == "" && gjson.GetBytes(res, "data.mp3").Type.String() == "JSON" {
			for _, v := range gjson.GetBytes(res, "data.mp3").Array() {
				mp3UrlCache = v.Get("mp3_cdn").Str
				if mp3UrlCache == "" {
					mp3UrlCache = v.Get("mp3_url").Str
				}
				if Debug {
					fmt.Printf("format:%s | url:%s\n", v.Get("mp3_format").Str, v.Get("mp3_url").Str)
				}
				if mp3UrlCache != "" && v.Get("mp3_format").Str == "mp3" {
					if Debug {
						fmt.Println("breaking")
					}
					mp3Url = mp3UrlCache
					break
				}
			}
			if mp3Url == "" {
				mp3Url = mp3UrlCache
			}
		}
	}
	if mp3Url == "" {
		if gjson.GetBytes(res, "status").Int() == 0 {
			return "", "", errors.New("ivalid_url")
		}
		fmt.Println(string(res))
		return "", "", errors.New("failed fetch ytmp3")
	} else if mp3Url[:4] != "http" {
		mp3Url = "https://ytpp3.com" + mp3Url
	}

	return title, mp3Url, nil
}
