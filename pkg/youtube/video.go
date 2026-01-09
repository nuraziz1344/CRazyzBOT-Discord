package youtube

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/tidwall/gjson"
)

type VideoInfo struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Thumbnail  string `json:"thumbnail"`
	YouTubeURL string `json:"youtube_url"`
}

func GetVideoInfo(videoURL string) (*VideoInfo, error) {
	parsedUrl, err := url.Parse(videoURL)
	if err != nil {
		return &VideoInfo{}, err
	}

	vId := ""
	switch parsedUrl.Host {
	case "youtu.be":
		vId = parsedUrl.Path[1:]
	case "youtube.com", "www.youtube.com":
		vId = parsedUrl.Query().Get("v")
	default:
		return &VideoInfo{}, errors.New("invalid_video_url")
	}

	if vId == "" {
		return &VideoInfo{}, errors.New("invalid_video_url")
	}

	reqUrl := fmt.Sprintf("https://youtube.googleapis.com/youtube/v3/videos?part=snippet&id=%s&key=%s", vId, os.Getenv("YT_APIKEY"))
	req, err := http.Get(reqUrl)
	if err != nil {
		return &VideoInfo{}, err
	}

	defer req.Body.Close()
	res, err := io.ReadAll(req.Body)
	if err != nil {
		return &VideoInfo{}, err
	}

	data := string(res)
	result := &VideoInfo{
		ID:         vId,
		Title:      gjson.Get(data, "items.0.snippet.title").Str,
		Thumbnail:  fmt.Sprintf("https://i.ytimg.com/vi/%s/default.jpg", vId),
		YouTubeURL: videoURL,
	}

	return result, nil
}

func SearchVideo(keyword string) (*VideoInfo, error) {
	var ytApikey = os.Getenv("YT_APIKEY")

	if len(keyword) >= 4 && keyword[:4] == "http" {
		return GetVideoInfo(keyword)
	}

	reqUrl := fmt.Sprintf("https://youtube.googleapis.com/youtube/v3/search?part=snippet&type=video&q=%s&key=%s", url.QueryEscape(keyword), ytApikey)
	req, err := http.Get(reqUrl)
	if err != nil {
		return &VideoInfo{}, err
	}

	defer req.Body.Close()
	res, err := io.ReadAll(req.Body)
	if err != nil {
		return &VideoInfo{}, err
	}

	data := string(res)
	vId := gjson.Get(data, "items.0.id.videoId").String()
	result := &VideoInfo{
		ID:         vId,
		Title:      gjson.Get(data, "items.0.snippet.title").Str,
		Thumbnail:  fmt.Sprintf("https://i.ytimg.com/vi/%s/default.jpg", vId),
		YouTubeURL: fmt.Sprintf("https://youtu.be/%s", vId),
	}

	return result, nil
}
